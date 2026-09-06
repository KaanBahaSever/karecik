package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"karecik/backend/internal/models"
)

// businessColumns is the account scan order. A business is the tenant and
// nothing else: a name and the slug it answers on. That slug is the subdomain
// of {business-slug}.karecik.com, so it is globally unique; every published
// setting — branding, splash, contact, pricing, languages — lives on the menu,
// because a menu is what a customer opens.
//
// There is no is_active on a business any more: a tenant is visible exactly
// when it has active menus.
const businessColumns = `id, user_id, name, slug, created_at, updated_at`

func scanBusiness(row pgx.Row) (*models.Business, error) {
	var business models.Business
	err := row.Scan(&business.ID, &business.UserID, &business.Name, &business.Slug,
		&business.CreatedAt, &business.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &business, nil
}

// insertBusiness creates the account during sign-up — and nothing else. A brand
// new tenant owns zero menus: there is no default menu to seed, because there
// is no default menu at all. The dashboard greets the owner with its empty
// states until they create the first one themselves.
func insertBusiness(ctx context.Context, db DB, userID uuid.UUID, name, slug string) (*models.Business, error) {
	business, err := scanBusiness(db.QueryRow(ctx, `
		INSERT INTO businesses (user_id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING `+businessColumns, userID, name, slug))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("could not create the business: %w", err)
	}
	return business, nil
}

// GetBusinessByID fetches a business by identifier.
func GetBusinessByID(ctx context.Context, db DB, id uuid.UUID) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE id = $1`, id))
}

// GetBusinessByUserID fetches the business that belongs to a user.
func GetBusinessByUserID(ctx context.Context, db DB, userID uuid.UUID) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE user_id = $1`, userID))
}

// GetBusinessBySlug is the public lookup: it turns the subdomain the customer
// typed into the tenant behind it.
//
// It deliberately has no publication filter — a business carries no is_active
// flag — so an account that publishes nothing still resolves. The emptiness is
// then answered by the payload (menu_resolved false plus an empty menu list),
// never by a 404 on a tenant that really exists.
func GetBusinessBySlug(ctx context.Context, db DB, slug string) (*models.Business, error) {
	return scanBusiness(db.QueryRow(ctx,
		`SELECT `+businessColumns+` FROM businesses WHERE slug = lower($1)`,
		strings.TrimSpace(slug)))
}

// BusinessSlugTaken reports whether another business already answers on this
// slug. The namespace is global on purpose: the slug is a hostname label, so
// two tenants can no more share one than two hosts could. Menu slugs are the
// opposite — see MenuSlugTaken, which is scoped to a single business.
func BusinessSlugTaken(ctx context.Context, db DB, slug string, exceptID uuid.UUID) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM businesses WHERE slug = $1 AND id <> $2)`,
		strings.ToLower(strings.TrimSpace(slug)), exceptID).Scan(&exists)
	return exists, err
}

// businessUpdatableColumns lists the columns PUT /api/business may change, and
// it is exactly two: the account holds nothing else that anybody may rewrite.
// Column names only ever come from this allowlist, so SQL injection is
// impossible.
var businessUpdatableColumns = map[string]bool{
	"name": true, "slug": true,
}

// UpdateBusiness applies a partial update to the given columns and returns the
// updated record.
func UpdateBusiness(ctx context.Context, db DB, id uuid.UUID,
	fields map[string]any) (*models.Business, error) {

	columns := make([]string, 0, len(fields))
	for column := range fields {
		if businessUpdatableColumns[column] {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return GetBusinessByID(ctx, db, id)
	}
	sort.Strings(columns)

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+1)
	for i, column := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, i+1))
		args = append(args, fields[column])
	}
	args = append(args, id)

	query := `UPDATE businesses SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d RETURNING `, len(args)) + businessColumns

	business, err := scanBusiness(db.QueryRow(ctx, query, args...))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return business, nil
}

// LogPriceUpdate writes the audit record of a bulk price change.
func LogPriceUpdate(ctx context.Context, db DB, businessID uuid.UUID,
	percentage float64, rounding string, affected int) error {
	_, err := db.Exec(ctx,
		`INSERT INTO price_update_logs (business_id, percentage, rounding, affected)
		 VALUES ($1, $2, $3, $4)`,
		businessID, percentage, rounding, affected)
	return err
}
