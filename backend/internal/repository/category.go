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

const categoryColumns = `id, business_id, translations, icon, image_url,
	position, is_active, created_at, updated_at`

func scanCategory(row pgx.Row) (*models.Category, error) {
	var c models.Category
	err := row.Scan(&c.ID, &c.BusinessID, &c.Translations, &c.Icon, &c.ImageURL,
		&c.Position, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if c.Translations == nil {
		c.Translations = models.Translations{}
	}
	return &c, nil
}

// ListCategories, isletmenin kategorilerini sira numarasina gore dondurur.
// Her kategori icindeki urun sayisini (product_count) da tasir.
func ListCategories(ctx context.Context, db DB, businessID uuid.UUID) ([]models.Category, error) {
	rows, err := db.Query(ctx, `
		SELECT c.id, c.business_id, c.translations, c.icon, c.image_url,
		       c.position, c.is_active, c.created_at, c.updated_at,
		       COUNT(p.id) AS product_count
		FROM categories c
		LEFT JOIN products p ON p.category_id = c.id
		WHERE c.business_id = $1
		GROUP BY c.id
		ORDER BY c.position ASC, c.created_at ASC`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]models.Category, 0)
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.BusinessID, &c.Translations, &c.Icon, &c.ImageURL,
			&c.Position, &c.IsActive, &c.CreatedAt, &c.UpdatedAt, &c.ProductCount); err != nil {
			return nil, err
		}
		if c.Translations == nil {
			c.Translations = models.Translations{}
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// GetCategory, kategoriyi getirir ve isletmeye ait oldugunu dogrular.
func GetCategory(ctx context.Context, db DB, id, businessID uuid.UUID) (*models.Category, error) {
	return scanCategory(db.QueryRow(ctx,
		`SELECT `+categoryColumns+` FROM categories WHERE id = $1 AND business_id = $2`,
		id, businessID))
}

// CreateCategory, yeni kategoriyi listenin sonuna ekler.
func CreateCategory(ctx context.Context, db DB, businessID uuid.UUID,
	translations models.Translations, icon, imageURL *string, isActive bool) (*models.Category, error) {

	var nextPosition int
	err := db.QueryRow(ctx,
		`SELECT COALESCE(MAX(position) + 1, 0) FROM categories WHERE business_id = $1`,
		businessID).Scan(&nextPosition)
	if err != nil {
		return nil, err
	}

	return scanCategory(db.QueryRow(ctx, `
		INSERT INTO categories (business_id, translations, icon, image_url, position, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+categoryColumns,
		businessID, translations, icon, imageURL, nextPosition, isActive))
}

var categoryUpdatableColumns = map[string]bool{
	"translations": true, "icon": true, "image_url": true, "is_active": true, "position": true,
}

// UpdateCategory, verilen sutunlari gunceller (kismi guncelleme).
func UpdateCategory(ctx context.Context, db DB, id, businessID uuid.UUID,
	fields map[string]any) (*models.Category, error) {

	columns := make([]string, 0, len(fields))
	for col := range fields {
		if categoryUpdatableColumns[col] {
			columns = append(columns, col)
		}
	}
	if len(columns) == 0 {
		return GetCategory(ctx, db, id, businessID)
	}
	sort.Strings(columns)

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+2)
	for i, col := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", col, i+1))
		args = append(args, fields[col])
	}
	args = append(args, id, businessID)

	query := `UPDATE categories SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d AND business_id = $%d RETURNING `, len(args)-1, len(args)) +
		categoryColumns

	return scanCategory(db.QueryRow(ctx, query, args...))
}

// DeleteCategory, kategoriyi ve (cascade ile) icindeki urunleri siler.
// Silinen urun sayisini dondurur.
func DeleteCategory(ctx context.Context, db DB, id, businessID uuid.UUID) (int, error) {
	var productCount int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM products WHERE category_id = $1 AND business_id = $2`,
		id, businessID).Scan(&productCount)
	if err != nil {
		return 0, err
	}

	tag, err := db.Exec(ctx,
		`DELETE FROM categories WHERE id = $1 AND business_id = $2`, id, businessID)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() == 0 {
		return 0, ErrNotFound
	}
	return productCount, nil
}

// ReorderCategories, verilen kimlik sirasini position sutununa yazar (0,1,2...).
// Tek bir UPDATE ile calisir; listede olmayan kategoriler etkilenmez.
func ReorderCategories(ctx context.Context, db DB, businessID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := db.Exec(ctx, `
		UPDATE categories c
		SET position = data.ord - 1
		FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
		WHERE c.id = data.id AND c.business_id = $1`,
		businessID, ids)
	return err
}
