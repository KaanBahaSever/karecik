package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/models"
)

const productColumns = `id, business_id, category_id, translations, price, compare_price,
	image_url, allergens, is_active, is_featured, position, created_at, updated_at`

func scanProduct(row pgx.Row) (*models.Product, error) {
	var product models.Product
	err := row.Scan(&product.ID, &product.BusinessID, &product.CategoryID, &product.Translations,
		&product.Price, &product.ComparePrice, &product.ImageURL, &product.Allergens,
		&product.IsActive, &product.IsFeatured, &product.Position,
		&product.CreatedAt, &product.UpdatedAt)
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
		args = append(args, "%"+term+"%")
		conditions = append(conditions, fmt.Sprintf("p.translations::text ILIKE $%d", len(args)))
	}

	query := `
		SELECT p.id, p.business_id, p.category_id, p.translations, p.price, p.compare_price,
		       p.image_url, p.allergens, p.is_active, p.is_featured, p.position,
		       p.created_at, p.updated_at
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
			&product.Translations, &product.Price, &product.ComparePrice, &product.ImageURL,
			&product.Allergens, &product.IsActive, &product.IsFeatured, &product.Position,
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
func CreateProduct(ctx context.Context, db DB, businessID, categoryID uuid.UUID,
	translations models.Translations, price float64, comparePrice *float64,
	imageURL *string, allergens []string, isActive, isFeatured bool) (*models.Product, error) {

	if allergens == nil {
		allergens = []string{}
	}

	var nextPosition int
	err := db.QueryRow(ctx,
		`SELECT COALESCE(MAX(position) + 1, 0) FROM products WHERE category_id = $1`,
		categoryID).Scan(&nextPosition)
	if err != nil {
		return nil, err
	}

	return scanProduct(db.QueryRow(ctx, `
		INSERT INTO products (business_id, category_id, translations, price, compare_price,
		                      image_url, allergens, is_active, is_featured, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+productColumns,
		businessID, categoryID, translations, price, comparePrice,
		imageURL, allergens, isActive, isFeatured, nextPosition))
}

var productUpdatableColumns = map[string]bool{
	"category_id": true, "translations": true, "price": true, "compare_price": true,
	"image_url": true, "allergens": true, "is_active": true, "is_featured": true,
	"position": true,
}

// UpdateProduct applies a partial update to the given columns.
func UpdateProduct(ctx context.Context, db DB, id, businessID uuid.UUID,
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

	return scanProduct(db.QueryRow(ctx, query, args...))
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
func ReorderProducts(ctx context.Context, db DB, businessID, categoryID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := db.Exec(ctx, `
		UPDATE products p
		SET position = data.ord - 1, category_id = $3
		FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
		WHERE p.id = data.id AND p.business_id = $1`,
		businessID, ids, categoryID)
	return err
}

// PriceRow is the lightweight product record used by the bulk price update.
type PriceRow struct {
	ID           uuid.UUID
	Price        float64
	Translations models.Translations
}

// ListPriceRows returns the products that take part in a bulk update.
// An empty categoryIDs selects every product of the business.
func ListPriceRows(ctx context.Context, db DB, businessID uuid.UUID,
	categoryIDs []uuid.UUID) ([]PriceRow, error) {

	query := `SELECT p.id, p.price, p.translations
	          FROM products p
	          JOIN categories c ON c.id = p.category_id
	          WHERE p.business_id = $1`
	args := []any{businessID}

	if len(categoryIDs) > 0 {
		args = append(args, categoryIDs)
		query += fmt.Sprintf(" AND p.category_id = ANY($%d::uuid[])", len(args))
	}
	query += " ORDER BY c.position ASC, p.position ASC"

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PriceRow, 0)
	for rows.Next() {
		var row PriceRow
		if err := rows.Scan(&row.ID, &row.Price, &row.Translations); err != nil {
			return nil, err
		}
		if row.Translations == nil {
			row.Translations = models.Translations{}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ApplyPrices writes the computed new prices inside a single transaction.
func ApplyPrices(ctx context.Context, pool *pgxpool.Pool, businessID uuid.UUID,
	newPrices map[uuid.UUID]float64) (int, error) {

	if len(newPrices) == 0 {
		return 0, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	affected := 0
	for id, price := range newPrices {
		tag, err := tx.Exec(ctx,
			`UPDATE products SET price = $1 WHERE id = $2 AND business_id = $3`,
			price, id, businessID)
		if err != nil {
			return 0, err
		}
		affected += int(tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return affected, nil
}
