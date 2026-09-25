package handlers

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// The audit trail of the owner's own writes.
//
// Every mutating dashboard endpoint hands its repository write a hook built
// here (recordHook). The hook runs inside the write's own transaction, as its
// last statement, and inserts one audit_logs row describing what the write
// produced — so the change and its record commit together, and a write that
// repository.RetryOnConflict runs again is recorded once, by the run that
// committed. See repository.WriteHook.
//
// The one write with no transaction of its own to join is an upload, which
// puts a file on disk and no row in the database; it is recorded afterwards,
// best-effort (recordNow).

// auditActor is who made a request and from where: the session's user and
// tenant, and the address middleware.ClientAddrOf resolved — the one the
// request log line prints, so the trail and the log never disagree.
type auditActor struct {
	businessID uuid.UUID
	userID     uuid.UUID
	ip         string
	port       string
	source     string
}

// auditActorOf reads the actor of a request. It is read before the write
// starts, on the request goroutine: the hook runs later, inside the
// repository, and must not touch the fiber.Ctx.
func auditActorOf(c *fiber.Ctx) auditActor {
	addr := middleware.ClientAddrOf(c)
	return auditActor{
		businessID: middleware.BusinessID(c),
		userID:     middleware.UserID(c),
		ip:         addr.IP,
		port:       addr.Port,
		source:     string(addr.Source),
	}
}

// auditRecord is what one write says about itself: the action, the record it
// touched and the fields that changed.
type auditRecord struct {
	action     string
	entityType string
	entityID   string
	label      string
	changes    audit.Changes
}

// entry completes a record with the actor into the row to insert.
func (a auditActor) entry(record auditRecord) audit.Entry {
	return audit.Entry{
		BusinessID:  a.businessID,
		UserID:      a.userID,
		Action:      record.action,
		EntityType:  record.entityType,
		EntityID:    record.entityID,
		EntityLabel: record.label,
		Changes:     record.changes,
		IP:          a.ip,
		Port:        a.port,
		IPSource:    a.source,
	}
}

// recordHook builds the hook of one write. build turns what the write produced
// into the record to insert, reading anything else it needs through the
// transaction it is handed; a nil record inserts nothing — an update that
// changed no field, say, is not an event worth a row.
func recordHook[T any](actor auditActor,
	build func(ctx context.Context, tx pgx.Tx, result T) (*auditRecord, error)) repository.WriteHook[T] {

	return func(ctx context.Context, tx pgx.Tx, result T) error {
		record, err := build(ctx, tx, result)
		if err != nil || record == nil {
			return err
		}
		return repository.InsertAuditLog(ctx, tx, actor.entry(*record))
	}
}

// recordNow writes a record outside any transaction, for the one write that
// has none (an upload). It is best-effort: the write it describes has already
// happened and cannot be undone, so a failure here is logged and the request
// still succeeds.
func (h *Handler) recordNow(c *fiber.Ctx, record auditRecord) {
	actor := auditActorOf(c)
	if err := repository.InsertAuditLog(c.Context(), h.DB, actor.entry(record)); err != nil {
		log.Printf("[karecik] could not record the audit entry %s for %s: %v",
			record.action, actor.businessID, err)
	}
}

// ListAuditLogs — GET /api/audit-logs?entity_type=&action=&limit=50&offset=0
//
// The tenant's trail, newest first. Both filters are optional; a value that is
// not one of the known ones is a 422 rather than an empty page, so a typo in
// the dashboard shows up as a typo.
func (h *Handler) ListAuditLogs(c *fiber.Ctx) error {
	entityType := strings.TrimSpace(c.Query("entity_type"))
	if entityType != "" && !audit.IsEntityType(entityType) {
		return utils.Unprocessable(c, "Geçersiz kayıt türü.")
	}
	action := strings.TrimSpace(c.Query("action"))
	if action != "" && !audit.IsAction(action) {
		return utils.Unprocessable(c, "Geçersiz işlem türü.")
	}
	limit, offset, ok := pageOf(c)
	if !ok {
		return utils.Unprocessable(c, "Sayfalama değerleri geçersiz.")
	}

	items, total, err := repository.ListAuditLogs(c.Context(), h.DB, middleware.BusinessID(c),
		repository.AuditLogFilter{EntityType: entityType, Action: action, Limit: limit, Offset: offset})
	if err != nil {
		return utils.Internal(c, err)
	}
	return utils.OK(c, fiber.Map{"items": items, "total": total, "limit": limit, "offset": offset})
}

// Paging of the dashboard's log lists.
const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

// pageOf reads limit and offset. A missing value takes its default, a limit
// above maxPageLimit is lowered to it and a limit below 1 takes the default;
// anything that is not a whole number — or a negative offset — reports false.
func pageOf(c *fiber.Ctx) (limit, offset int, ok bool) {
	limit, offset = defaultPageLimit, 0

	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, false
		}
		switch {
		case value < 1:
			limit = defaultPageLimit
		case value > maxPageLimit:
			limit = maxPageLimit
		default:
			limit = value
		}
	}
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return 0, 0, false
		}
		offset = value
	}
	return limit, offset, true
}
