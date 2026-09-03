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

const categoryColumns = `id, business_id, menu_id, translations, icon, image_url,
	position, is_active, created_at, updated_at`

func scanCategory(row pgx.Row) (*models.Category, error) {
	var category models.Category
	err := row.Scan(&category.ID, &category.BusinessID, &category.MenuID,
		&category.Translations, &category.Icon,
		&category.ImageURL, &category.Position, &category.IsActive,
		&category.CreatedAt, &category.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if category.Translations == nil {
		category.Translations = models.Translations{}
	}
	return &category, nil
}

// ListCategories returns the categories of a business ordered by position.
// Each category also carries the number of products it holds (product_count).
//
// menuID == nil lists every category of the business — the bulk price and
// reorder paths work on all of them. When a menu is given the result mirrors
// what BuildPublicMenu shows for that menu, so the editor lists exactly what
// the customer will see: a category that was never assigned to a menu still
// belongs to the business and therefore stays on the default menu.
func ListCategories(ctx context.Context, db DB, businessID uuid.UUID,
	menuID *uuid.UUID) ([]models.Category, error) {

	query := `
		SELECT c.id, c.business_id, c.menu_id, c.translations, c.icon, c.image_url,
		       c.position, c.is_active, c.created_at, c.updated_at,
		       COUNT(p.id) AS product_count
		FROM categories c
		LEFT JOIN products p ON p.category_id = c.id
		WHERE c.business_id = $1`
	args := []any{businessID}

	if menuID != nil {
		args = append(args, *menuID)
		query += ` AND (c.menu_id = $2
		           OR (c.menu_id IS NULL AND EXISTS (
		               SELECT 1 FROM menus m WHERE m.id = $2 AND m.is_default)))`
	}
	query += `
		GROUP BY c.id
		ORDER BY c.position ASC, c.created_at ASC`

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]models.Category, 0)
	for rows.Next() {
		var category models.Category
		if err := rows.Scan(&category.ID, &category.BusinessID, &category.MenuID,
			&category.Translations, &category.Icon, &category.ImageURL,
			&category.Position, &category.IsActive,
			&category.CreatedAt, &category.UpdatedAt, &category.ProductCount); err != nil {
			return nil, err
		}
		if category.Translations == nil {
			category.Translations = models.Translations{}
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

// GetCategory fetches a category and verifies that it belongs to the business.
func GetCategory(ctx context.Context, db DB, id, businessID uuid.UUID) (*models.Category, error) {
	return scanCategory(db.QueryRow(ctx,
		`SELECT `+categoryColumns+` FROM categories WHERE id = $1 AND business_id = $2`,
		id, businessID))
}

// CreateCategory appends a new category to the end of the list of the given
// menu. The caller resolves the menu (the business' default one when the
// request did not name it) and has already checked that it belongs to the
// business.
func CreateCategory(ctx context.Context, db DB, businessID, menuID uuid.UUID,
	translations models.Translations, icon, imageURL *string, isActive bool) (*models.Category, error) {

	var nextPosition int
	err := db.QueryRow(ctx,
		`SELECT COALESCE(MAX(position) + 1, 0) FROM categories WHERE business_id = $1`,
		businessID).Scan(&nextPosition)
	if err != nil {
		return nil, err
	}

	return scanCategory(db.QueryRow(ctx, `
		INSERT INTO categories (business_id, menu_id, translations, icon, image_url,
		                        position, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+categoryColumns,
		businessID, menuID, translations, icon, imageURL, nextPosition, isActive))
}

// menu_id is updatable so a category can be moved between menus.
var categoryUpdatableColumns = map[string]bool{
	"translations": true, "icon": true, "image_url": true, "is_active": true, "position": true,
	"menu_id": true,
}

// UpdateCategory applies a partial update to the given columns.
func UpdateCategory(ctx context.Context, db DB, id, businessID uuid.UUID,
	fields map[string]any) (*models.Category, error) {

	columns := make([]string, 0, len(fields))
	for column := range fields {
		if categoryUpdatableColumns[column] {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return GetCategory(ctx, db, id, businessID)
	}
	sort.Strings(columns)

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+2)
	for i, column := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, i+1))
		args = append(args, fields[column])
	}
	args = append(args, id, businessID)

	query := `UPDATE categories SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d AND business_id = $%d RETURNING `, len(args)-1, len(args)) +
		categoryColumns

	return scanCategory(db.QueryRow(ctx, query, args...))
}

// DeleteCategory removes a category and (through the cascade) its products.
// It returns how many products were deleted along with it.
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

// ReorderCategories writes the given id order into the position column (0,1,2...).
// It runs as a single UPDATE; categories missing from the list are untouched.
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
