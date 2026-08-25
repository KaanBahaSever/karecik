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
	var p models.Product
	err := row.Scan(&p.ID, &p.BusinessID, &p.CategoryID, &p.Translations, &p.Price,
		&p.ComparePrice, &p.ImageURL, &p.Allergens, &p.IsActive, &p.IsFeatured,
		&p.Position, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	normalizeProduct(&p)
	return &p, nil
}

func normalizeProduct(p *models.Product) {
	if p.Translations == nil {
		p.Translations = models.Translations{}
	}
	if p.Allergens == nil {
		p.Allergens = []string{}
	}
}

// ListProducts, isletmenin urunlerini kategori sirasi + urun sirasina gore dondurur.
// categoryID nil degilse yalnizca o kategori, search bos degilse ad/aciklama filtresi uygulanir.
func ListProducts(ctx context.Context, db DB, businessID uuid.UUID,
	categoryID *uuid.UUID, search string) ([]models.Product, error) {

	conditions := []string{"p.business_id = $1"}
	args := []any{businessID}

	if categoryID != nil {
		args = append(args, *categoryID)
		conditions = append(conditions, fmt.Sprintf("p.category_id = $%d", len(args)))
	}
	if s := strings.TrimSpace(search); s != "" {
		args = append(args, "%"+s+"%")
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
		var p models.Product
		if err := rows.Scan(&p.ID, &p.BusinessID, &p.CategoryID, &p.Translations, &p.Price,
			&p.ComparePrice, &p.ImageURL, &p.Allergens, &p.IsActive, &p.IsFeatured,
			&p.Position, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		normalizeProduct(&p)
		products = append(products, p)
	}
	return products, rows.Err()
}

// GetProduct, urunu getirir ve isletmeye ait oldugunu dogrular.
func GetProduct(ctx context.Context, db DB, id, businessID uuid.UUID) (*models.Product, error) {
	return scanProduct(db.QueryRow(ctx,
		`SELECT `+productColumns+` FROM products WHERE id = $1 AND business_id = $2`,
		id, businessID))
}

// CreateProduct, urunu kategorisinin sonuna ekler.
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

// UpdateProduct, verilen sutunlari gunceller (kismi guncelleme).
func UpdateProduct(ctx context.Context, db DB, id, businessID uuid.UUID,
	fields map[string]any) (*models.Product, error) {

	columns := make([]string, 0, len(fields))
	for col := range fields {
		if productUpdatableColumns[col] {
			columns = append(columns, col)
		}
	}
	if len(columns) == 0 {
		return GetProduct(ctx, db, id, businessID)
	}
	sort.Strings(columns)

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+2)
	for i, col := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", col, i+1))
		args = append(args, fields[col])
	}
	args = append(args, id, businessID)

	query := `UPDATE products SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d AND business_id = $%d RETURNING `, len(args)-1, len(args)) +
		productColumns

	return scanProduct(db.QueryRow(ctx, query, args...))
}

// DeleteProduct, urunu siler.
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

// ReorderProducts, verilen kimlik sirasini position sutununa yazar ve ayni zamanda
// urunleri hedef kategoriye tasir (surukleyerek kategori degistirme).
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

// PriceRow, toplu fiyat guncellemesinde kullanilan hafif urun kaydi.
type PriceRow struct {
	ID           uuid.UUID
	Price        float64
	Translations models.Translations
}

// ListPriceRows, toplu guncellemeye girecek urunleri dondurur.
// categoryIDs bos ise isletmenin tum urunleri secilir.
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
		var r PriceRow
		if err := rows.Scan(&r.ID, &r.Price, &r.Translations); err != nil {
			return nil, err
		}
		if r.Translations == nil {
			r.Translations = models.Translations{}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ApplyPrices, hesaplanan yeni fiyatlari tek transaction icinde yazar.
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
