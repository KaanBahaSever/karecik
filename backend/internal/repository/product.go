package repository

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// productColumns is the single source of truth for the product scan order:
// scanProduct, the inline scan in ListProducts and the product scan in
// BuildPublicMenu all read the columns in exactly this order.
const productColumns = `id, business_id, category_id, translations, price, compare_price,
	calories, image_url, allergens, badges, options, is_active, is_featured, position,
	created_at, updated_at`

func scanProduct(row pgx.Row) (*models.Product, error) {
	var product models.Product
	err := row.Scan(&product.ID, &product.BusinessID, &product.CategoryID, &product.Translations,
		&product.Price, &product.ComparePrice, &product.Calories, &product.ImageURL,
		&product.Allergens, &product.Badges, &product.Options, &product.IsActive,
		&product.IsFeatured, &product.Position, &product.CreatedAt, &product.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	normalizeProduct(&product)
	return &product, nil
}

func normalizeProduct(product *models.Product) {
	if product.Translations == nil {
		product.Translations = models.Translations{}
	}
	if product.Allergens == nil {
		product.Allergens = []string{}
	}
	if product.Badges == nil {
		product.Badges = models.Badges{}
	}
	// options is a NOT NULL jsonb column like badges, so the payload has to
	// carry [] and never null.
	if product.Options == nil {
		product.Options = models.ProductOptions{}
	}
}

// ListProducts returns the products of a business ordered by category position
// and then product position. When categoryID is non-nil only that category is
// returned; a non-empty search filters on the name and description.
func ListProducts(ctx context.Context, db DB, businessID uuid.UUID,
	categoryID *uuid.UUID, search string) ([]models.Product, error) {

	conditions := []string{"p.business_id = $1"}
	args := []any{businessID}

	if categoryID != nil {
		args = append(args, *categoryID)
		conditions = append(conditions, fmt.Sprintf("p.category_id = $%d", len(args)))
	}
	if term := strings.TrimSpace(search); term != "" {
		// No stored text holds U+0000 or bytes that are not UTF-8 (see
		// UnstorableText), so a search for such a term matches nothing — which
		// is the answer, without the query.
		if UnstorableText(term) {
			return make([]models.Product, 0), nil
		}
		args = append(args, "%"+term+"%")
		conditions = append(conditions, fmt.Sprintf("p.translations::text ILIKE $%d", len(args)))
	}

	query := `
		SELECT p.id, p.business_id, p.category_id, p.translations, p.price, p.compare_price,
		       p.calories, p.image_url, p.allergens, p.badges, p.options, p.is_active,
		       p.is_featured, p.position, p.created_at, p.updated_at
		FROM products p
		JOIN categories c ON c.id = p.category_id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY c.position ASC, p.position ASC, p.created_at ASC`

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]models.Product, 0)
	for rows.Next() {
		var product models.Product
		if err := rows.Scan(&product.ID, &product.BusinessID, &product.CategoryID,
			&product.Translations, &product.Price, &product.ComparePrice, &product.Calories,
			&product.ImageURL, &product.Allergens, &product.Badges, &product.Options,
			&product.IsActive, &product.IsFeatured, &product.Position,
			&product.CreatedAt, &product.UpdatedAt); err != nil {
			return nil, err
		}
		normalizeProduct(&product)
		products = append(products, product)
	}
	return products, rows.Err()
}

// GetProduct fetches a product and verifies that it belongs to the business.
func GetProduct(ctx context.Context, db DB, id, businessID uuid.UUID) (*models.Product, error) {
	return scanProduct(db.QueryRow(ctx,
		`SELECT `+productColumns+` FROM products WHERE id = $1 AND business_id = $2`,
		id, businessID))
}

// CreateProduct appends a product to the end of its category.
//
// It is a write into the category, so it runs inside writeIntoCategory: the
// shared advisory locks of the category's menu and of the category come first
// (see locks.go). A delete of either makes the create wait before it has locked
// anything, and once that delete has removed the category the create comes back
// with ErrParentNotFound, having written nothing. A delete of the category that
// starts while the create runs waits for it instead, so the delete's count of
// the products it removes includes the new one.
//
// A run PostgreSQL aborted over a lock conflict rolled its transaction back, so
// RetryOnConflict can run it again as it is.
func CreateProduct(ctx context.Context, db TxDB, businessID, categoryID uuid.UUID,
	translations models.Translations, price float64, comparePrice *float64, calories *int,
	imageURL *string, allergens []string, badges models.Badges, options models.ProductOptions,
	isActive, isFeatured bool) (*models.Product, error) {

	if allergens == nil {
		allergens = []string{}
	}
	// badges and options are NOT NULL jsonb columns, so a nil slice becomes an
	// empty array — encoding/json would write the literal null instead.
	if badges == nil {
		badges = models.Badges{}
	}
	if options == nil {
		options = models.ProductOptions{}
	}

	var product *models.Product
	err := writeIntoCategory(ctx, db, businessID, categoryID, func(tx pgx.Tx) error {
		var nextPosition int
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(position) + 1, 0) FROM products WHERE category_id = $1`,
			categoryID).Scan(&nextPosition); err != nil {
			return err
		}

		created, err := scanProduct(tx.QueryRow(ctx, `
			INSERT INTO products (business_id, category_id, translations, price, compare_price,
			                      calories, image_url, allergens, badges, options, is_active,
			                      is_featured, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING `+productColumns,
			businessID, categoryID, translations, price, comparePrice, calories,
			imageURL, allergens, badges, options, isActive, isFeatured, nextPosition))
		product = created
		return err
	})
	if err != nil {
		return nil, err
	}
	return product, nil
}

var productUpdatableColumns = map[string]bool{
	"category_id": true, "translations": true, "price": true, "compare_price": true,
	"calories": true, "image_url": true, "allergens": true, "badges": true,
	"options": true, "is_active": true, "is_featured": true, "position": true,
}

// UpdateProduct applies a partial update to the given columns.
//
// An update that names a category — category_id among the columns, as a
// uuid.UUID — runs inside writeIntoCategory: a transaction that first holds the
// shared advisory locks of that category's menu and of the category (see
// locks.go). That is a move unless the category is the one the product is
// already in, which the product dialog sends on every save; it is locked all
// the same, because telling the two apart would take a read of the product that
// a concurrent move could make stale. A delete of the category or of its menu
// makes the update wait before it has locked the product, and once that delete
// has removed the category the update comes back with ErrParentNotFound. Every
// other update is one statement.
//
// Either way a run PostgreSQL aborted over a lock conflict wrote nothing, so
// RetryOnConflict can run it again as it is.
func UpdateProduct(ctx context.Context, db TxDB, id, businessID uuid.UUID,
	fields map[string]any) (*models.Product, error) {

	columns := make([]string, 0, len(fields))
	for column := range fields {
		if productUpdatableColumns[column] {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return GetProduct(ctx, db, id, businessID)
	}
	sort.Strings(columns)

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+2)
	for i, column := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, i+1))
		args = append(args, fields[column])
	}
	args = append(args, id, businessID)

	query := `UPDATE products SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d AND business_id = $%d RETURNING `, len(args)-1, len(args)) +
		productColumns

	value, moves := fields["category_id"]
	if !moves {
		return scanProduct(db.QueryRow(ctx, query, args...))
	}
	// The locks are keyed on the id, so a move has to name its category as a
	// uuid.UUID; anything else would move the product without them.
	target, ok := value.(uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("UpdateProduct: category_id has to be a uuid.UUID, not %T", value)
	}

	var product *models.Product
	err := writeIntoCategory(ctx, db, businessID, target, func(tx pgx.Tx) error {
		updated, err := scanProduct(tx.QueryRow(ctx, query, args...))
		product = updated
		return err
	})
	if err != nil {
		return nil, err
	}
	return product, nil
}

// DeleteProduct removes a product.
func DeleteProduct(ctx context.Context, db DB, id, businessID uuid.UUID) error {
	tag, err := db.Exec(ctx,
		`DELETE FROM products WHERE id = $1 AND business_id = $2`, id, businessID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderProducts writes the given id order into the position column and moves
// the products into the target category, which is how drag-and-drop between
// categories is handled.
//
// It rewrites category_id, so it is a write into the category and runs inside
// writeIntoCategory: the shared advisory locks of the category's menu and of
// the category come first (see locks.go), so a delete of either makes the
// reorder wait before it has locked a single product. ErrParentNotFound comes
// back when the business has no such category, or no longer has it once the
// locks are held.
//
// Then two statements. The first locks the listed products of the business in
// ascending id order — the one lock order every multi-row product writer
// follows, explained at DeleteMenu — and the second writes them. Every row is
// locked before the first one is written, and the UPDATE takes its snapshot
// only once all of them are locked, so it sees the latest version of every row
// it names and writes all of them: two reorders of the same category that run
// into each other leave distinct positions. Locking and writing in one
// statement — a locking CTE and an UPDATE ... FROM a join against it — would
// work from the snapshot the statement started with, and under READ COMMITTED
// could leave a row that another transaction wrote in the meantime unwritten,
// without an error. The locking statement has no join either, so a row that
// changed while the reorder waited for it is checked again against its id and
// its business alone.
//
// A run PostgreSQL aborted over a lock conflict rolled its transaction back, so
// RetryOnConflict can run it again as it is.
func ReorderProducts(ctx context.Context, db TxDB, businessID, categoryID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return writeIntoCategory(ctx, db, businessID, categoryID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			SELECT id FROM products
			WHERE id = ANY($2::uuid[]) AND business_id = $1
			ORDER BY id
			FOR UPDATE`, businessID, ids); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE products p
			SET position = data.ord - 1, category_id = $3
			FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
			WHERE p.id = data.id AND p.business_id = $1`,
			businessID, ids, categoryID)
		return err
	})
}

// PriceRow is the lightweight product record used by the bulk price update.
type PriceRow struct {
	ID           uuid.UUID
	Price        float64
	Translations models.Translations

	// Where the menu editor lists the product: the bulk price update reports its
	// rows in that order (see SortPriceRows).
	CategoryPosition int
	CategoryID       uuid.UUID
	Position         int
}

// PriceChange is one product of a bulk price update: the row as it was read and
// the price the update gives it.
type PriceChange struct {
	PriceRow
	NewPrice float64
}

// Changed reports whether the update writes a new price for the product. The
// price read is compared at two decimals, the precision of its column.
func (c PriceChange) Changed() bool {
	return c.NewPrice != utils.Round2(c.Price)
}

// PlanPriceChanges prices every row, keeping their order:
// RoundPrice(ApplyPercentage(price, percentage), rounding). The preview and
// ApplyPrices both price through it, so the two agree on every price they read
// the same.
func PlanPriceChanges(rows []PriceRow, percentage float64, rounding string) []PriceChange {
	changes := make([]PriceChange, 0, len(rows))
	for _, row := range rows {
		changes = append(changes, PriceChange{
			PriceRow: row,
			NewPrice: utils.RoundPrice(utils.ApplyPercentage(row.Price, percentage), rounding),
		})
	}
	return changes
}

// priceRowColumns is the select list collectPriceRows scans, in its order.
const priceRowColumns = `p.id, p.price, p.translations, p.category_id, c.position, p.position`

// priceRowsQuery is the SELECT every bulk price read runs, with the given
// select list. A bulk update is scoped to one menu — prices belong to the menu
// they are printed on — so the products of the business are reached through
// their category; a non-empty categoryIDs narrows them to those categories. It
// carries no ORDER BY: each caller adds what it needs.
func priceRowsQuery(columns string, businessID, menuID uuid.UUID,
	categoryIDs []uuid.UUID) (string, []any) {

	query := `SELECT ` + columns + `
	          FROM products p
	          JOIN categories c ON c.id = p.category_id
	          WHERE p.business_id = $1 AND c.menu_id = $2`
	args := []any{businessID, menuID}

	if len(categoryIDs) > 0 {
		args = append(args, categoryIDs)
		query += fmt.Sprintf(" AND p.category_id = ANY($%d::uuid[])", len(args))
	}
	return query, args
}

// collectPriceRows scans the result of priceRowsQuery and closes it.
func collectPriceRows(rows pgx.Rows) ([]PriceRow, error) {
	defer rows.Close()

	out := make([]PriceRow, 0)
	for rows.Next() {
		var row PriceRow
		if err := rows.Scan(&row.ID, &row.Price, &row.Translations, &row.CategoryID,
			&row.CategoryPosition, &row.Position); err != nil {
			return nil, err
		}
		if row.Translations == nil {
			row.Translations = models.Translations{}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SortPriceRows puts rows in menu editor order: category position, then product
// position. Equal positions fall back to the category id and the product id,
// compared the way PostgreSQL orders uuid values, so the order never depends on
// how a query happened to return the rows — the preview and an apply of the
// same rows list them identically.
func SortPriceRows(rows []PriceRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.CategoryPosition != b.CategoryPosition {
			return a.CategoryPosition < b.CategoryPosition
		}
		if a.CategoryID != b.CategoryID {
			return bytes.Compare(a.CategoryID[:], b.CategoryID[:]) < 0
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return bytes.Compare(a.ID[:], b.ID[:]) < 0
	})
}

// ListPriceRows returns the products that take part in a bulk update, in menu
// editor order, without locking them. It backs the preview, which writes
// nothing; ApplyPrices reads the rows again under its locks.
func ListPriceRows(ctx context.Context, db DB, businessID, menuID uuid.UUID,
	categoryIDs []uuid.UUID) ([]PriceRow, error) {

	query, args := priceRowsQuery(priceRowColumns, businessID, menuID, categoryIDs)
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out, err := collectPriceRows(rows)
	if err != nil {
		return nil, err
	}
	SortPriceRows(out)
	return out, nil
}

// PriceLimitExceeded reports whether a planned price is larger than
// utils.MaxPrice, the largest price products.price holds. The preview refuses
// such a plan exactly as ApplyPrices does, so the two agree on it for every
// price they read the same.
func PriceLimitExceeded(changes []PriceChange) bool {
	for _, change := range changes {
		if change.NewPrice > utils.MaxPrice {
			return true
		}
	}
	return false
}

// ApplyPrices applies a bulk price update to the products of one menu of the
// business. It returns every product it priced — in menu editor order, with the
// price it was read at and the price the update gives it — and the number of
// products it wrote.
//
// It is one transaction of four steps:
//
//  1. The preview's SELECT, without a lock, names the products the update
//     takes part in.
//  2. A second SELECT locks exactly those rows, by id and in ascending id
//     order — the one lock order every multi-row product writer follows (see
//     DeleteMenu) — with FOR UPDATE and no join. A row another transaction
//     wrote while this one waited for it is checked again against its id and
//     its business alone, so it is locked whatever else about it changed. A
//     join to categories would be checked again with the category row the
//     statement read first, and under READ COMMITTED a product moved to another
//     category of the same menu in the meantime would drop out of the lock —
//     and out of the update — without an error.
//  3. The preview's SELECT runs again for the locked ids, as a statement of its
//     own. It takes its snapshot once every row is locked, so it reads each of
//     them as it is now: a price edited while the apply waited is the price the
//     percentage applies to, a product moved to another category of the menu is
//     read in that category, and a product that left the menu is not read.
//  4. PlanPriceChanges computes the new prices in Go from exactly those rows.
//     When one of them is larger than utils.MaxPrice the transaction ends with
//     ErrPriceTooLarge and writes nothing. Otherwise one UPDATE writes the rows
//     whose price changes, and no other; it starts after every row is locked,
//     so its snapshot sees all of them as they are now and it writes every row
//     it names.
//
// Locking and writing are separate statements. A locking CTE and an
// UPDATE ... FROM a join against it would work from the snapshot the statement
// started with, and under READ COMMITTED could leave a row that another
// transaction wrote in the meantime unwritten, without an error.
//
// Not a loop of single-row UPDATEs either: every changed price fires the
// products_touch_menu_price_date trigger of migration 010, which locks the menu
// row to move its price date. Row-level AFTER triggers fire at the end of their
// statement, and the UPDATE runs after the SELECT has locked every product, so
// the menu row is locked last — the order a single-product edit uses. A loop
// would lock the menu row after its first product and could then wait for a
// later product held by a concurrent inline edit that waits for the menu row.
//
// The ids and the prices travel as two parallel arrays that unnest zips back
// into pairs. pgx encodes each float64 of the numeric[] from its shortest
// round-trip decimal (strconv.FormatFloat with precision -1), and every price
// here has already been through utils.RoundPrice, so NUMERIC(12,2) stores
// exactly the two-decimal value the answer reports.
//
// A run PostgreSQL aborted over a lock conflict rolled its transaction back, so
// RetryOnConflict can run it again: the next run reads the prices afresh.
func ApplyPrices(ctx context.Context, db TxDB, businessID, menuID uuid.UUID,
	categoryIDs []uuid.UUID, percentage float64, rounding string) ([]PriceChange, int, error) {

	var (
		changes []PriceChange
		written int
	)
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		// 1. The products the update takes part in.
		query, args := priceRowsQuery("p.id", businessID, menuID, categoryIDs)
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		named, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return err
		}
		if len(named) == 0 {
			changes, written = PlanPriceChanges(nil, percentage, rounding), 0
			return nil
		}

		// 2. Their row locks, taken by id alone.
		if _, err := tx.Exec(ctx, `
			SELECT id FROM products
			WHERE id = ANY($1::uuid[]) AND business_id = $2
			ORDER BY id
			FOR UPDATE`, named, businessID); err != nil {
			return err
		}

		// 3. The rows as they are once every one of them is locked.
		query, args = priceRowsQuery(priceRowColumns, businessID, menuID, categoryIDs)
		args = append(args, named)
		rows, err = tx.Query(ctx, query+fmt.Sprintf(" AND p.id = ANY($%d::uuid[])", len(args)), args...)
		if err != nil {
			return err
		}
		locked, err := collectPriceRows(rows)
		if err != nil {
			return err
		}
		SortPriceRows(locked)

		// 4. The new prices, and the write.
		changes = PlanPriceChanges(locked, percentage, rounding)
		if PriceLimitExceeded(changes) {
			return ErrPriceTooLarge
		}

		ids := make([]uuid.UUID, 0, len(changes))
		prices := make([]float64, 0, len(changes))
		for _, change := range changes {
			if change.Changed() {
				ids = append(ids, change.ID)
				prices = append(prices, change.NewPrice)
			}
		}
		if len(ids) == 0 {
			written = 0
			return nil
		}

		tag, err := tx.Exec(ctx, `
			UPDATE products p
			SET price = data.price
			FROM unnest($1::uuid[], $2::numeric[]) AS data(id, price)
			WHERE p.id = data.id AND p.business_id = $3`,
			ids, prices, businessID)
		if err != nil {
			return err
		}
		written = int(tag.RowsAffected())
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return changes, written, nil
}
