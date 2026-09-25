package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/clientip"
)

// WriteHook runs inside the transaction of a write, as its last statement, and
// receives what the write produced. It is how an audit row is written in the
// same transaction as the change it describes: the two commit or roll back
// together, so the trail never records a write that did not happen and never
// misses one that did.
//
// It is also what keeps a retried write from being recorded twice. A run that
// RetryOnConflict starts again was aborted by PostgreSQL, and the audit row its
// hook inserted was rolled back with everything else; only the run that
// commits leaves a row.
//
// The writers that take hooks accept them as a trailing variadic argument, so a
// caller that records nothing — a test, a script — calls them exactly as it
// always did. A hook's error fails the write.
//
// What an update or a delete hands its hooks includes the row as it was right
// before the write (Update.Before, or the removed row itself), and that copy is
// read inside the write's transaction too — never by the caller beforehand. A
// copy read earlier, outside the transaction, is only as old as the moment it
// was read: a write that commits between that read and this one would be
// reported as this write's change, with the wrong old values, and a change
// this write makes back to a value the other write had moved would not be
// reported at all. See "The before-read" below.
//
// Lock order. A hook runs after every row lock its write takes, and the audit
// INSERT adds only the KEY SHARE locks its foreign keys take on the business and
// the user row. Nothing that holds either of those more strongly waits for
// anything else: a business row is written by UpdateBusiness alone, whose
// transaction takes no other lock but those same KEY SHARE locks, and a
// password change updates no key column. So the hook cannot close a lock cycle
// with the orders documented at DeleteMenu.
type WriteHook[T any] func(ctx context.Context, tx pgx.Tx, result T) error

// Update is what the hooks of an update receive: the row as the write found it,
// under the row lock the write holds, and the row the write returned. Nothing
// can commit to the row between the two, so their difference is exactly what
// this write changed.
type Update[T any] struct {
	Before T
	After  T
}

// The before-read.
//
// Every update and delete that has hooks reads the row it is about to write —
// lockedProduct, lockedCategory, lockedMenu, lockedBusiness — and the reads
// follow the same rules:
//
//   - a SELECT ... FOR <lock> of the one row, scoped to the business like the
//     write, run as a statement of its own immediately before the write —
//     after any advisory lock the write takes (writeIntoMenu,
//     writeIntoCategory, DeleteCategory, DeleteMenu) and after the row locks a
//     delete takes on the rows below it, so the documented lock order does not
//     change;
//   - it takes exactly the lock the write takes on that row, one statement
//     earlier, so it adds no lock, the write never has to strengthen it, and it
//     cannot close a lock cycle the write alone would not;
//   - under READ COMMITTED a locking read that waited for a concurrent writer
//     returns the row as that writer committed it, and the write that follows
//     takes a fresh snapshot while the lock is held, so the copy is the very
//     version the write replaces;
//   - a row the business does not have, or no longer has, is ErrNotFound
//     before anything is written.
//
// Without hooks there is nothing to hand a copy to, and the write runs alone,
// exactly the statement it always was.

// The row locks of a before-read. Each is the lock the write that follows takes
// on the same row anyway: FOR UPDATE for a DELETE and for an UPDATE that may
// change a column of a unique key a foreign key can reference — menus.slug and
// businesses.slug — and FOR NO KEY UPDATE for every other UPDATE.
const (
	lockForUpdate      = "FOR UPDATE"
	lockForNoKeyUpdate = "FOR NO KEY UPDATE"
)

// updateInTx runs an update of one row inside tx and then its hooks with both
// copies of the row. With no hooks it skips the before-read.
func updateInTx[T any](ctx context.Context, tx pgx.Tx, hooks []WriteHook[Update[T]],
	before func(db DB) (T, error), update func(db DB) (T, error)) (T, error) {

	var zero, locked T
	if len(hooks) > 0 {
		value, err := before(tx)
		if err != nil {
			return zero, err
		}
		locked = value
	}
	after, err := update(tx)
	if err != nil {
		return zero, err
	}
	if err := runHooks(ctx, tx, Update[T]{Before: locked, After: after}, hooks); err != nil {
		return zero, err
	}
	return after, nil
}

// updateWithHooks is writeWithHooks for an update: on its own when nothing is
// to be recorded, and otherwise in a transaction of its own whose first
// statement is the before-read (see "The before-read"), then the update, then
// the hooks. A run PostgreSQL aborted over a lock conflict rolled all of it
// back, so RetryOnConflict can run it again — and the next run reads the row
// afresh.
func updateWithHooks[T any](ctx context.Context, db TxDB, hooks []WriteHook[Update[T]],
	before func(db DB) (T, error), update func(db DB) (T, error)) (T, error) {

	if len(hooks) == 0 {
		return update(db)
	}
	var result T
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		value, err := updateInTx(ctx, tx, hooks, before, update)
		if err != nil {
			return err
		}
		result = value
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// runHooks runs the hooks of one write, in order, and stops at the first error.
func runHooks[T any](ctx context.Context, tx pgx.Tx, result T, hooks []WriteHook[T]) error {
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		if err := hook(ctx, tx, result); err != nil {
			return fmt.Errorf("could not record the audit entry: %w", err)
		}
	}
	return nil
}

// writeWithHooks runs one self-contained write: on its own when nothing is to
// be recorded — exactly the statement it always was — and otherwise in a
// transaction of its own, followed by the hooks. The write itself is the first
// statement either way, so it takes its row locks in the order it always did.
//
// A run PostgreSQL aborted over a lock conflict rolled the whole transaction
// back, hook rows included, so RetryOnConflict can still run it again as it is.
func writeWithHooks[T any](ctx context.Context, db TxDB, hooks []WriteHook[T],
	write func(db DB) (T, error)) (T, error) {

	if len(hooks) == 0 {
		return write(db)
	}
	var result T
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		value, err := write(tx)
		if err != nil {
			return err
		}
		result = value
		return runHooks(ctx, tx, value, hooks)
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// maxAuditLabelRunes bounds entity_label. It is a snapshot of a name the
// application already caps far below this; the bound is for a label built from
// something less disciplined, such as an uploaded file's original name.
const maxAuditLabelRunes = 200

// InsertAuditLog writes one audit_logs row.
//
// user_email is read from users inside the same statement rather than passed
// in: the session knows only the user id, and a subquery is one round trip
// fewer than a lookup — and a snapshot taken in the same transaction as the
// write. A user id that matches nothing (uuid.Nil included) leaves it empty.
//
// The resolver's "-" (and "") become NULL in ip and port, so a row never
// claims an address that was not established; ip_source still says how the
// request arrived.
func InsertAuditLog(ctx context.Context, db DB, entry audit.Entry) error {
	changes := entry.Changes
	if changes == nil {
		changes = audit.Changes{}
	}
	var userID *uuid.UUID
	if entry.UserID != uuid.Nil {
		id := entry.UserID
		userID = &id
	}
	var entityID *string
	if entry.EntityID != "" {
		id := entry.EntityID
		entityID = &id
	}
	label := entry.EntityLabel
	if runes := []rune(label); len(runes) > maxAuditLabelRunes {
		label = string(runes[:maxAuditLabelRunes])
	}
	ip, port, source := AddrColumns(entry.IP, entry.Port, entry.IPSource)

	_, err := db.Exec(ctx, `
		INSERT INTO audit_logs (business_id, user_id, user_email, action, entity_type,
		                        entity_id, entity_label, changes, ip, port, ip_source)
		VALUES ($1, $2::uuid, COALESCE((SELECT email FROM users WHERE id = $2::uuid), ''),
		        $3, $4, $5, $6, $7, $8, $9, $10)`,
		entry.BusinessID, userID, entry.Action, entry.EntityType, entityID, label,
		changes, ip, port, source)
	return err
}

// AddrColumns turns the resolver's strings into the three address columns
// shared by audit_logs and menu_events: the address or NULL, the port as a
// number or NULL, and the source label ("unknown" when blank). A port that is
// not a number between 0 and 65535 — the resolver never produces one, but the
// column CHECK would refuse it — is NULL as well.
func AddrColumns(ip, port, source string) (*string, *int, string) {
	var ipColumn *string
	if ip = strings.TrimSpace(ip); ip != "" && ip != clientip.IPUnknown {
		ipColumn = &ip
	}
	var portColumn *int
	if value, err := strconv.Atoi(strings.TrimSpace(port)); err == nil && value >= 0 && value <= 65535 {
		portColumn = &value
	}
	if source = strings.TrimSpace(source); source == "" {
		source = string(clientip.SourceUnknown)
	}
	return ipColumn, portColumn, source
}

// AuditLog is one row of GET /api/audit-logs.
type AuditLog struct {
	ID          uuid.UUID       `json:"id"`
	CreatedAt   time.Time       `json:"created_at"`
	UserID      *uuid.UUID      `json:"user_id"`
	UserEmail   string          `json:"user_email"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entity_type"`
	EntityID    *string         `json:"entity_id"`
	EntityLabel string          `json:"entity_label"`
	Changes     json.RawMessage `json:"changes"`
	IP          *string         `json:"ip"`
	Port        *int            `json:"port"`
	IPSource    string          `json:"ip_source"`
}

// AuditLogFilter narrows the audit list. Empty strings do not filter; the
// handler has already checked that a non-empty one is a known value.
type AuditLogFilter struct {
	EntityType string
	Action     string
	Limit      int
	Offset     int
}

// ListAuditLogs returns one page of a tenant's audit trail, newest first, and
// the number of rows the filter matches in all.
//
// The business predicate is the first condition and is not optional: the
// trail of one tenant is never readable through another's session. The filter
// values travel as parameters; the only SQL built from pieces here is the
// fixed text of the conditions themselves.
func ListAuditLogs(ctx context.Context, db DB, businessID uuid.UUID,
	filter AuditLogFilter) ([]AuditLog, int, error) {

	conditions := []string{"business_id = $1"}
	args := []any{businessID}
	if filter.EntityType != "" {
		args = append(args, filter.EntityType)
		conditions = append(conditions, fmt.Sprintf("entity_type = $%d", len(args)))
	}
	if filter.Action != "" {
		args = append(args, filter.Action)
		conditions = append(conditions, fmt.Sprintf("action = $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")

	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE `+where, args...).
		Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := db.Query(ctx, `
		SELECT id, created_at, user_id, user_email, action, entity_type, entity_id,
		       entity_label, changes, ip, port, ip_source
		FROM audit_logs
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := make([]AuditLog, 0)
	for rows.Next() {
		var row AuditLog
		var changes []byte
		if err := rows.Scan(&row.ID, &row.CreatedAt, &row.UserID, &row.UserEmail, &row.Action,
			&row.EntityType, &row.EntityID, &row.EntityLabel, &changes, &row.IP, &row.Port,
			&row.IPSource); err != nil {
			return nil, 0, err
		}
		row.Changes = json.RawMessage(changes)
		if len(row.Changes) == 0 {
			row.Changes = json.RawMessage(`{}`)
		}
		logs = append(logs, row)
	}
	return logs, total, rows.Err()
}
