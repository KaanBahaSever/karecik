package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"karecik/backend/internal/audit"
	"karecik/backend/internal/clientip"
	"karecik/backend/internal/ipexclude"
	"karecik/backend/internal/middleware"
	"karecik/backend/internal/repository"
	"karecik/backend/internal/utils"
)

// The owner's side of the analytics exclusions: the list of addresses and
// ranges whose visits the business's analytics never store, and the opt-out
// mark of the owner's own browser. The events endpoint enforces both
// (handlers.TrackEvent, visitExcluded); this file only manages them.
//
// What the panel shows is this business's list and nothing else. The platform's
// ANALYTICS_EXCLUDED_IPS list also applies to this business's events, but no
// response here or anywhere else names it or says whether it matched: it holds
// the operator's addresses, which are no tenant's business. current_ip_excluded
// is accordingly answered from the business's own list alone.

// Turkish messages shared by the endpoints below.
const (
	msgExclusionMissing  = "IP adresi veya aralığı girilmelidir."
	msgExclusionInvalid  = "Geçerli bir IP adresi (ör. 198.18.139.87) veya CIDR aralığı (ör. 198.18.139.0/24) girin."
	msgExclusionDup      = "Bu IP zaten listede."
	msgExclusionNotFound = "Bu IP listede bulunamadı."
)

// msgExclusionTooBroad explains the breadth limit rather than just stating it:
// the owner who typed a /8 meant "my provider", and needs to hear why that
// would erase their customers too.
var msgExclusionTooBroad = fmt.Sprintf("Bu aralık çok geniş: IPv4 için en fazla /%d, IPv6 için en fazla /%d "+
	"genişliğinde bir aralık girilebilir. Daha geniş bir aralık, sizinle aynı internet sağlayıcısını "+
	"kullanan çok sayıda gerçek ziyaretçiyi de analizlerden çıkarır.", ipexclude.MinIPv4Bits, ipexclude.MinIPv6Bits)

// msgExclusionLimit names the limit and the way out of it.
var msgExclusionLimit = fmt.Sprintf("En fazla %d IP adresi veya aralığı eklenebilir. "+
	"Yenisini eklemek için listeden birini kaldırın.", ipexclude.MaxPerBusiness)

// parseExclusion reads an address or range from a request, or the Turkish
// message it is refused with.
func parseExclusion(raw string) (netip.Prefix, string) {
	if strings.TrimSpace(raw) == "" {
		return netip.Prefix{}, msgExclusionMissing
	}
	prefix, err := ipexclude.Parse(raw)
	switch {
	case errors.Is(err, ipexclude.ErrTooBroad):
		return netip.Prefix{}, msgExclusionTooBroad
	case err != nil:
		return netip.Prefix{}, msgExclusionInvalid
	}
	return prefix, ""
}

// excludedIPList is the body of GET /api/analytics/excluded-ips.
type excludedIPList struct {
	Items []repository.ExcludedIP `json:"items"`

	// CurrentIP is the address this very request was resolved to — what the
	// panel offers as "add my current address" — or null when none could be
	// established. CurrentIPSource says how much it is worth (clientip.Source):
	// an address the edge did not prove may not be the one the menu's events
	// will arrive with.
	CurrentIP       *string `json:"current_ip"`
	CurrentIPSource string  `json:"current_ip_source"`

	// CurrentIPExcluded is whether this business's list covers CurrentIP.
	CurrentIPExcluded bool `json:"current_ip_excluded"`

	// OptOut is whether this browser carries the opt-out mark.
	OptOut bool `json:"optout"`

	Max int `json:"max"`
}

// ListExcludedIPs — GET /api/analytics/excluded-ips
func (h *Handler) ListExcludedIPs(c *fiber.Ctx) error {
	businessID := middleware.BusinessID(c)
	items, err := repository.ListExcludedIPs(c.Context(), h.DB, businessID)
	if err != nil {
		return utils.Internal(c, err)
	}

	addr := middleware.ClientAddrOf(c)
	body := excludedIPList{
		Items:           items,
		CurrentIPSource: string(addr.Source),
		OptOut:          middleware.AnalyticsOptedOut(c),
		Max:             ipexclude.MaxPerBusiness,
	}
	if ip := strings.TrimSpace(addr.IP); ip != "" && ip != clientip.IPUnknown {
		body.CurrentIP = &ip
		prefixes := make([]netip.Prefix, 0, len(items))
		for _, item := range items {
			prefixes = append(prefixes, item.Prefix)
		}
		body.CurrentIPExcluded = ipexclude.Contains(prefixes, ip)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return utils.OK(c, body)
}

// ExcludedIPMatchCount — GET /api/analytics/excluded-ips/match-count?cidr=
//
// How many of this business's stored events fall inside the address or range
// — the number the panel shows next to "also delete the visits already
// recorded" before the owner commits to it. The input obeys exactly the rules
// of an add, so the preview is never offered for a range the add would refuse.
func (h *Handler) ExcludedIPMatchCount(c *fiber.Ctx) error {
	prefix, message := parseExclusion(c.Query("cidr"))
	if message != "" {
		return utils.Unprocessable(c, message)
	}
	count, err := repository.CountEventsInRange(c.Context(), h.DB, middleware.BusinessID(c), prefix)
	if err != nil {
		return utils.Internal(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return utils.OK(c, fiber.Map{"cidr": prefix.String(), "count": count})
}

// excludedIPRequest is the body of POST /api/analytics/excluded-ips.
// DeleteHistory is a pointer because it is required: the choice between
// keeping and deleting the visits already recorded is the owner's, and a
// client that forgot to ask must not have it made for them either way.
type excludedIPRequest struct {
	CIDR          string  `json:"cidr"`
	Label         *string `json:"label"`
	DeleteHistory *bool   `json:"delete_history"`
}

// AddExcludedIP — POST /api/analytics/excluded-ips
//
// 201 {"item": {...}, "deleted_events": n}. The entry, the optional deletion
// of the range's stored events and the audit row are one transaction
// (repository.AddExcludedIP), and the events endpoint's cached copy of the list
// is dropped the moment it commits, so the owner's very next view of the menu
// is already not counted.
func (h *Handler) AddExcludedIP(c *fiber.Ctx) error {
	var req excludedIPRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return utils.BadRequest(c, "İstek gövdesi okunamadı.")
	}

	prefix, message := parseExclusion(req.CIDR)
	if message != "" {
		return utils.Unprocessable(c, message)
	}
	label, message := exclusionLabel(req.Label)
	if message != "" {
		return utils.Unprocessable(c, message)
	}
	if req.DeleteHistory == nil {
		return utils.Unprocessable(c,
			"Bu adresten daha önce kaydedilmiş ziyaretlerin silinip silinmeyeceği belirtilmelidir (delete_history).")
	}

	actor := auditActorOf(c)
	record := recordHook(actor,
		func(_ context.Context, _ pgx.Tx, added repository.AddedExcludedIP) (*auditRecord, error) {
			changes := audit.Changes{
				"cidr":           {Old: nil, New: added.Item.CIDR},
				"deleted_events": {Old: nil, New: added.DeletedEvents},
			}
			if added.Item.Label != "" {
				changes["label"] = audit.Change{Old: nil, New: added.Item.Label}
			}
			return &auditRecord{
				action:     audit.ActionAnalyticsExcludeIPAdd,
				entityType: audit.EntityAnalyticsExclusion,
				entityID:   added.Item.ID.String(),
				label:      added.Item.Display,
				changes:    changes,
			}, nil
		})

	entry := repository.NewExcludedIP{
		BusinessID:    actor.businessID,
		Prefix:        prefix,
		Label:         label,
		CreatedBy:     actor.userID,
		DeleteHistory: *req.DeleteHistory,
	}
	// One transaction of its own, so a run PostgreSQL aborted over a lock
	// conflict — the history delete against the retention purge or a menu
	// delete — wrote nothing and is simply run again.
	added, err := repository.RetryOnConflictValue(c.Context(), "AddExcludedIP",
		func() (repository.AddedExcludedIP, error) {
			return repository.AddExcludedIP(c.Context(), h.DB, entry, record)
		})
	// Dropped whatever the outcome: after a failed COMMIT nobody can tell
	// whether the row is there, and reading the list again costs one query.
	h.invalidateExclusions(actor.businessID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicate):
			return utils.Conflict(c, msgExclusionDup)
		case errors.Is(err, repository.ErrExcludedIPLimit):
			return utils.Unprocessable(c, msgExclusionLimit)
		}
		return utils.Internal(c, err)
	}
	return utils.Created(c, fiber.Map{"item": added.Item, "deleted_events": added.DeletedEvents})
}

// RemoveExcludedIP — DELETE /api/analytics/excluded-ips/:id
//
// 204. An id of another business is the same 404 as an id nobody has: the
// endpoint never tells one tenant what another's list holds.
func (h *Handler) RemoveExcludedIP(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "Geçersiz kayıt kimliği.")
	}

	actor := auditActorOf(c)
	record := recordHook(actor,
		func(_ context.Context, _ pgx.Tx, removed repository.ExcludedIP) (*auditRecord, error) {
			changes := audit.Changes{"cidr": {Old: removed.CIDR, New: nil}}
			if removed.Label != "" {
				changes["label"] = audit.Change{Old: removed.Label, New: nil}
			}
			return &auditRecord{
				action:     audit.ActionAnalyticsExcludeIPRemove,
				entityType: audit.EntityAnalyticsExclusion,
				entityID:   removed.ID.String(),
				label:      removed.Display,
				changes:    changes,
			}, nil
		})

	_, err = repository.RemoveExcludedIP(c.Context(), h.DB, actor.businessID, id, record)
	h.invalidateExclusions(actor.businessID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.NotFound(c, msgExclusionNotFound)
		}
		return utils.Internal(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AnalyticsOptOut — POST /api/analytics/optout
//
// 204, and this browser carries the opt-out mark for a year: its visits to any
// menu are no longer stored (middleware.SetAnalyticsOptOut). Nothing is
// written to the database or the audit trail — the mark belongs to a browser,
// not to the business, and says nothing about the business's data.
func (h *Handler) AnalyticsOptOut(c *fiber.Ctx) error {
	middleware.SetAnalyticsOptOut(c, h.Cfg)
	return c.SendStatus(fiber.StatusNoContent)
}

// AnalyticsOptIn — DELETE /api/analytics/optout
//
// 204, and the mark is gone: this browser's visits count again.
func (h *Handler) AnalyticsOptIn(c *fiber.Ctx) error {
	middleware.ClearAnalyticsOptOut(c, h.Cfg)
	return c.SendStatus(fiber.StatusNoContent)
}

// invalidateExclusions drops the events endpoint's cached copy of a business's
// list.
func (h *Handler) invalidateExclusions(businessID uuid.UUID) {
	if h.Exclusions != nil {
		h.Exclusions.Invalidate(businessID)
	}
}

// exclusionLabel checks an entry's optional label: trimmed, at most
// ipexclude.MaxLabelRunes characters, and plain text — a control character
// has no business in a name shown in a table, and U+0000 could not even be
// stored.
func exclusionLabel(raw *string) (string, string) {
	if raw == nil {
		return "", ""
	}
	label := strings.TrimSpace(*raw)
	if UnstorableText(label) || strings.IndexFunc(label, unicode.IsControl) >= 0 {
		return "", "Etiket geçersiz karakterler içeriyor."
	}
	if utf8.RuneCountInString(label) > ipexclude.MaxLabelRunes {
		return "", fmt.Sprintf("Etiket en fazla %d karakter olabilir.", ipexclude.MaxLabelRunes)
	}
	return label, ""
}
