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
// menuID == nil lists every category of the business — the reorder path works
// on all of them. When a menu is given the result mirrors what BuildPublicMenu
// shows for that menu, so the editor lists exactly what the customer will see:
// categories.menu_id is NOT NULL since migration 005, so the match is a plain
// equality.
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
		query += ` AND c.menu_id = $2`
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
// menu. The menu is always named by the request — there is no default menu to
// fall back to.
//
// It is a write into the menu, so it runs inside writeIntoMenu: the menu's
// shared advisory lock comes first (see locks.go). A delete of the menu makes
// the create wait before it has locked anything, and once that delete has
// removed the menu the create comes back with ErrParentNotFound, having written
// nothing; so does a create naming a menu the business does not own.
//
// A run PostgreSQL aborted over a lock conflict rolled its transaction back, so
// RetryOnConflict can run it again as it is.
func CreateCategory(ctx context.Context, db TxDB, businessID, menuID uuid.UUID,
	translations models.Translations, icon, imageURL *string, isActive bool) (*models.Category, error) {

	var category *models.Category
	err := writeIntoMenu(ctx, db, businessID, menuID, func(tx pgx.Tx) error {
		var nextPosition int
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(position) + 1, 0) FROM categories WHERE business_id = $1`,
			businessID).Scan(&nextPosition); err != nil {
			return err
		}

		created, err := scanCategory(tx.QueryRow(ctx, `
			INSERT INTO categories (business_id, menu_id, translations, icon, image_url,
			                        position, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING `+categoryColumns,
			businessID, menuID, translations, icon, imageURL, nextPosition, isActive))
		category = created
		return err
	})
	if err != nil {
		return nil, err
	}
	return category, nil
}

// menu_id is updatable so a category can be moved between menus.
var categoryUpdatableColumns = map[string]bool{
	"translations": true, "icon": true, "image_url": true, "is_active": true, "position": true,
	"menu_id": true,
}

// UpdateCategory applies a partial update to the given columns.
//
// An update that names a menu — menu_id among the columns, as a uuid.UUID —
// runs inside writeIntoMenu: a transaction that first holds the shared advisory
// lock of that menu (see locks.go). That is a move unless the menu is the one
// the category is already on; it is locked all the same, because telling the
// two apart would take a read of the category that a concurrent move could make
// stale. A delete of the menu makes the update wait before it has locked the
// category, and once that delete has removed the menu the update comes back
// with ErrParentNotFound, having moved nothing; so does an update naming a menu
// the business does not own. Every other update is one statement.
//
// Either way a run PostgreSQL aborted over a lock conflict wrote nothing, so
// RetryOnConflict can run it again as it is.
func UpdateCategory(ctx context.Context, db TxDB, id, businessID uuid.UUID,
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

	value, moves := fields["menu_id"]
	if !moves {
		return scanCategory(db.QueryRow(ctx, query, args...))
	}
	// The lock is keyed on the id, so a move has to name its menu as a
	// uuid.UUID; anything else would move the category without it.
	target, ok := value.(uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("UpdateCategory: menu_id has to be a uuid.UUID, not %T", value)
	}

	var category *models.Category
	err := writeIntoMenu(ctx, db, businessID, target, func(tx pgx.Tx) error {
		updated, err := scanCategory(tx.QueryRow(ctx, query, args...))
		category = updated
		return err
	})
	if err != nil {
		return nil, err
	}
	return category, nil
}

// DeleteCategory removes a category and (through the cascade) its products.
// It returns how many products were deleted along with it.
//
// When the business owns no such category it answers ErrNotFound having locked
// nothing. Otherwise it runs one transaction, in this order:
//
//  1. the category's exclusive advisory lock (see locks.go), so a move or a
//     reorder of products into the category waits until the delete is over,
//     and the delete waits for one already writing into it;
//  2. the category's products, in ascending id order — the one lock order every
//     multi-row product writer follows, explained at DeleteMenu. Left to the
//     cascade they would be locked in whatever order it reached them, and a
//     bulk price update or a product reorder taking the same rows in another
//     order could deadlock with it;
//  3. the DELETE, whose cascade finds those products already locked.
//
// The count is how many rows step 2 locked. It takes its snapshot after the
// advisory lock is held, and no write can bring a product into the category
// after that — a create, a move and a reorder into the category all take its
// advisory lock shared — so it counts every product the cascade removes. Step
// 2 has no join: a product that moved to another category while the delete
// waited for its row lock is checked again against its own category_id and
// left out, as it has to be.
func DeleteCategory(ctx context.Context, db TxDB, id, businessID uuid.UUID) (int, error) {
	var productCount int
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		owned, err := categoryOwned(ctx, tx, id, businessID)
		if err != nil {
			return err
		}
		if !owned {
			return ErrNotFound
		}
		if err := lockCategory(ctx, tx, id, true); err != nil {
			return err
		}

		locked, err := tx.Exec(ctx, `
			SELECT id FROM products
			WHERE category_id = $1 AND business_id = $2
			ORDER BY id
			FOR UPDATE`, id, businessID)
		if err != nil {
			return err
		}
		productCount = int(locked.RowsAffected())

		tag, err := tx.Exec(ctx,
			`DELETE FROM categories WHERE id = $1 AND business_id = $2`, id, businessID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return productCount, nil
}

// ReorderCategories writes the given id order into the position column
// (0, 1, 2 ...); categories missing from the list are untouched.
//
// One transaction of two statements. The first locks the listed categories of
// the business in ascending id order — the order DeleteMenu locks the
// categories of a menu in — and the second writes the positions. An UPDATE on
// its own locks each row when its plan reaches it, in an order that need not be
// the id order, and a menu delete taking the same categories in id order could
// then deadlock with it, each holding one category and waiting for the other.
// The lock is FOR NO KEY UPDATE, the one an UPDATE of position takes anyway,
// which does not conflict with the KEY SHARE lock the foreign key check of a
// product create or move takes on its category, so a reorder never waits for
// one of those. The UPDATE takes its snapshot once every row is locked, so it
// sees the latest version of each and writes all of them, and two reorders that
// run into each other leave distinct positions. Locking and writing in one
// statement — a locking CTE and an UPDATE ... FROM a join against it — would
// work from the snapshot the statement started with, and under READ COMMITTED
// could leave a row that another transaction wrote in the meantime unwritten.
//
// A run PostgreSQL aborted over a lock conflict rolled its transaction back, so
// RetryOnConflict can run it again as it is.
func ReorderCategories(ctx context.Context, db TxDB, businessID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			SELECT id FROM categories
			WHERE id = ANY($2::uuid[]) AND business_id = $1
			ORDER BY id
			FOR NO KEY UPDATE`, businessID, ids); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE categories c
			SET position = data.ord - 1
			FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
			WHERE c.id = data.id AND c.business_id = $1`,
			businessID, ids)
		return err
	})
}
