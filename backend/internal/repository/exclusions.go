package repository

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"karecik/backend/internal/ipexclude"
)

// The analytics exclusion list — analytics_excluded_ips of migration 015: the
// addresses and ranges whose visits a business's analytics never store. The
// panel reads and edits it (handlers/exclusions.go); the public events endpoint
// reads it through the cache of package ipexclude (ExcludedPrefixes). Every
// statement is scoped to one business by its first predicate.

// ExcludedIP is one entry of a business's list, as GET
// /api/analytics/excluded-ips returns it. CIDR is always the range's CIDR text
// ("198.18.139.87/32"); Display is how the panel shows it (ipexclude.Display).
// CreatedByEmail is null once the user who added it is gone.
type ExcludedIP struct {
	ID             uuid.UUID    `json:"id"`
	CIDR           string       `json:"cidr"`
	Display        string       `json:"display"`
	Label          string       `json:"label"`
	CreatedAt      time.Time    `json:"created_at"`
	CreatedByEmail *string      `json:"created_by_email"`
	Prefix         netip.Prefix `json:"-"`
}

// withPrefix fills the two text forms of an entry from its range.
func (e *ExcludedIP) withPrefix(prefix netip.Prefix) {
	e.Prefix = prefix
	e.CIDR = prefix.String()
	e.Display = ipexclude.Display(prefix)
}

// ErrExcludedIPLimit is AddExcludedIP's answer when the business already has
// ipexclude.MaxPerBusiness entries. Nothing was written.
var ErrExcludedIPLimit = errors.New("the business already has the maximum number of excluded addresses")

// ExclusionLockNamespace is the advisory-lock namespace of a business's
// exclusion list ("KRXI"), next to the menu and category ones of locks.go.
const ExclusionLockNamespace int32 = 0x4b525849

// ListExcludedIPs returns a business's list in the order it was built.
func ListExcludedIPs(ctx context.Context, db DB, businessID uuid.UUID) ([]ExcludedIP, error) {
	rows, err := db.Query(ctx, `
		SELECT x.id, x.cidr, x.label, x.created_at, u.email
		FROM analytics_excluded_ips x
		LEFT JOIN users u ON u.id = x.created_by
		WHERE x.business_id = $1
		ORDER BY x.created_at, x.id`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ExcludedIP, 0)
	for rows.Next() {
		var (
			item   ExcludedIP
			prefix netip.Prefix
		)
		if err := rows.Scan(&item.ID, &prefix, &item.Label, &item.CreatedAt, &item.CreatedByEmail); err != nil {
			return nil, err
		}
		item.withPrefix(prefix)
		items = append(items, item)
	}
	return items, rows.Err()
}

// ExcludedPrefixes returns only the ranges of a business's list — what the
// events endpoint's cache holds (ipexclude.Loader). A business without a list
// gets an empty slice, which is cached like any other answer.
func ExcludedPrefixes(ctx context.Context, db DB, businessID uuid.UUID) ([]netip.Prefix, error) {
	rows, err := db.Query(ctx,
		`SELECT cidr FROM analytics_excluded_ips WHERE business_id = $1`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefixes := make([]netip.Prefix, 0)
	for rows.Next() {
		var prefix netip.Prefix
		if err := rows.Scan(&prefix); err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, rows.Err()
}

// eventsInRange is the condition that picks a business's stored events whose
// address falls inside a range: $1 is the business, $2 the range.
//
// menu_events.ip is TEXT, and a value that is not an address — 'unknown',
// '-', the empty string — must neither match nor fail the statement, so it
// goes through karecik_try_inet (migration 015) rather than a plain cast. The
// business predicate comes first and is served by
// menu_events_business_created_idx, so the function only ever sees one
// tenant's rows — which can still be close to a million. On PostgreSQL 16+
// the function is a plain SQL expression the planner inlines here, so a count
// costs about what a bare cast would; the migration explains the older body.
const eventsInRange = `business_id = $1 AND karecik_try_inet(ip) <<= $2::cidr`

// CountEventsInRange counts a business's stored events whose address falls
// inside the range — what adding the range with its history deleted would
// remove (GET /api/analytics/excluded-ips/match-count).
func CountEventsInRange(ctx context.Context, db DB, businessID uuid.UUID, prefix netip.Prefix) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM menu_events WHERE `+eventsInRange,
		businessID, prefix).Scan(&n)
	return n, err
}

// NewExcludedIP is one entry to add. Prefix has been through ipexclude.Parse
// and Label through the handler's checks. CreatedBy uuid.Nil is stored as
// NULL.
type NewExcludedIP struct {
	BusinessID    uuid.UUID
	Prefix        netip.Prefix
	Label         string
	CreatedBy     uuid.UUID
	DeleteHistory bool
}

// AddedExcludedIP is what AddExcludedIP produced: the new entry, and how many
// stored events of the range it deleted (0 unless DeleteHistory was set).
type AddedExcludedIP struct {
	Item          ExcludedIP
	DeletedEvents int64
}

// AddExcludedIP adds one entry to a business's list and, when asked, deletes
// the events the business already stored from inside the range — all in ONE
// transaction, with the hooks (the audit row) as its last statement: the entry,
// the deletion and their record commit or roll back together, so the trail
// never says a history was deleted when it was not, and an entry is never added
// without the deletion the owner asked for.
//
// It refuses, having written nothing:
//
//   - ErrExcludedIPLimit when the business already has
//     ipexclude.MaxPerBusiness entries;
//   - ErrDuplicate when the range is already listed or lies entirely inside a
//     range that is — excluding 198.18.139.87 under an existing
//     198.18.139.0/24 would change nothing. A broader range over narrower
//     entries is accepted; they simply become redundant.
//
// Both checks read the list before the INSERT, so two adds of one business
// running side by side could each see room for one more, or each miss the
// other's range. The transaction therefore starts with an exclusive advisory
// lock on the business's list (ExclusionLockNamespace), taken before any row
// lock, as locks.go's rules require: the second add waits there, then reads
// the list the first one committed. Nothing else takes that lock, so it cannot
// close a cycle with the row locks of any other writer.
//
// The history DELETE locks menu_events rows, which the retention purge and a
// menu delete's cascade delete as well, each in an order of its own; a lost
// deadlock there aborts this whole transaction, and the caller runs it again
// under RetryOnConflict. The event gate's count of the business's events today
// is not lowered by the deletion: the cap is a guard against floods, and an
// overestimate for the rest of one day is the safe direction.
func AddExcludedIP(ctx context.Context, db TxDB, entry NewExcludedIP,
	hooks ...WriteHook[AddedExcludedIP]) (AddedExcludedIP, error) {

	var added AddedExcludedIP
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::int4, hashtext($2::uuid::text))`,
			ExclusionLockNamespace, entry.BusinessID); err != nil {
			return err
		}

		var count int
		var covered bool
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(bool_or($2::cidr <<= cidr), false)
			FROM analytics_excluded_ips
			WHERE business_id = $1`, entry.BusinessID, entry.Prefix).Scan(&count, &covered); err != nil {
			return err
		}
		if covered {
			return ErrDuplicate
		}
		if count >= ipexclude.MaxPerBusiness {
			return ErrExcludedIPLimit
		}

		var createdBy *uuid.UUID
		if entry.CreatedBy != uuid.Nil {
			id := entry.CreatedBy
			createdBy = &id
		}
		var (
			item   ExcludedIP
			prefix netip.Prefix
		)
		err := tx.QueryRow(ctx, `
			WITH inserted AS (
				INSERT INTO analytics_excluded_ips (business_id, cidr, label, created_by)
				VALUES ($1, $2::cidr, $3, $4::uuid)
				RETURNING id, cidr, label, created_at, created_by
			)
			SELECT i.id, i.cidr, i.label, i.created_at, u.email
			FROM inserted i
			LEFT JOIN users u ON u.id = i.created_by`,
			entry.BusinessID, entry.Prefix, entry.Label, createdBy).
			Scan(&item.ID, &prefix, &item.Label, &item.CreatedAt, &item.CreatedByEmail)
		if err != nil {
			if IsUniqueViolation(err) {
				return ErrDuplicate
			}
			return err
		}
		item.withPrefix(prefix)

		var deleted int64
		if entry.DeleteHistory {
			tag, err := tx.Exec(ctx, `DELETE FROM menu_events WHERE `+eventsInRange,
				entry.BusinessID, entry.Prefix)
			if err != nil {
				return err
			}
			deleted = tag.RowsAffected()
		}

		added = AddedExcludedIP{Item: item, DeletedEvents: deleted}
		return runHooks(ctx, tx, added, hooks)
	})
	if err != nil {
		return AddedExcludedIP{}, err
	}
	return added, nil
}

// RemoveExcludedIP removes one entry of a business's list and returns it as it
// was, with the hooks run in the same transaction. An id the business does not
// have — another tenant's included — is ErrNotFound, and nothing is removed.
func RemoveExcludedIP(ctx context.Context, db TxDB, businessID, id uuid.UUID,
	hooks ...WriteHook[ExcludedIP]) (ExcludedIP, error) {

	return writeWithHooks(ctx, db, hooks, func(db DB) (ExcludedIP, error) {
		var (
			item   ExcludedIP
			prefix netip.Prefix
		)
		err := db.QueryRow(ctx, `
			DELETE FROM analytics_excluded_ips
			WHERE id = $1 AND business_id = $2
			RETURNING id, cidr, label, created_at`, id, businessID).
			Scan(&item.ID, &prefix, &item.Label, &item.CreatedAt)
		if err != nil {
			if isNoRows(err) {
				return ExcludedIP{}, ErrNotFound
			}
			return ExcludedIP{}, err
		}
		item.withPrefix(prefix)
		return item, nil
	})
}
