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

// menuColumns is the menu scan order. menuColumnsM is the same list qualified
// with the m alias, needed wherever menus is joined against branch_menus:
// position and is_default exist on both tables.
const menuColumns = `id, business_id, name, slug, description, is_default,
	is_active, position, created_at, updated_at`

const menuColumnsM = `m.id, m.business_id, m.name, m.slug, m.description, m.is_default,
	m.is_active, m.position, m.created_at, m.updated_at`

// branchColumns is the branch scan order. branchColumnsBr is the same list
// qualified with the br alias for the join against businesses, which carries
// almost the same column names.
const branchColumns = `id, business_id, name, slug, phone, address, wifi_ssid,
	wifi_password, is_default, is_active, position, created_at, updated_at`

const branchColumnsBr = `br.id, br.business_id, br.name, br.slug, br.phone, br.address,
	br.wifi_ssid, br.wifi_password, br.is_default, br.is_active, br.position,
	br.created_at, br.updated_at`

func scanMenu(row pgx.Row) (*models.Menu, error) {
	var menu models.Menu
	err := row.Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Slug, &menu.Description,
		&menu.IsDefault, &menu.IsActive, &menu.Position, &menu.CreatedAt, &menu.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &menu, nil
}

func scanBranch(row pgx.Row) (*models.Branch, error) {
	var branch models.Branch
	err := row.Scan(&branch.ID, &branch.BusinessID, &branch.Name, &branch.Slug, &branch.Phone,
		&branch.Address, &branch.WifiSSID, &branch.WifiPassword, &branch.IsDefault,
		&branch.IsActive, &branch.Position, &branch.CreatedAt, &branch.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	branch.MenuIDs = []uuid.UUID{}
	return &branch, nil
}

// ---------------------------------------------------------------- menus

// ListMenus returns the menus of a business ordered by position. Each menu also
// carries how many categories it holds; categories that were never assigned to
// a menu are counted on the default menu, which is exactly where
// BuildPublicMenu shows them.
func ListMenus(ctx context.Context, db DB, businessID uuid.UUID) ([]models.Menu, error) {
	rows, err := db.Query(ctx, `
		SELECT `+menuColumnsM+`, COUNT(c.id) AS category_count
		FROM menus m
		LEFT JOIN categories c
		       ON c.business_id = m.business_id
		      AND (c.menu_id = m.id OR (c.menu_id IS NULL AND m.is_default))
		WHERE m.business_id = $1
		GROUP BY m.id
		ORDER BY m.position ASC, m.created_at ASC`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menus := make([]models.Menu, 0)
	for rows.Next() {
		var menu models.Menu
		if err := rows.Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Slug,
			&menu.Description, &menu.IsDefault, &menu.IsActive, &menu.Position,
			&menu.CreatedAt, &menu.UpdatedAt, &menu.CategoryCount); err != nil {
			return nil, err
		}
		menus = append(menus, menu)
	}
	return menus, rows.Err()
}

// GetMenu fetches a menu and verifies that it belongs to the business.
func GetMenu(ctx context.Context, db DB, id, businessID uuid.UUID) (*models.Menu, error) {
	return scanMenu(db.QueryRow(ctx,
		`SELECT `+menuColumns+` FROM menus WHERE id = $1 AND business_id = $2`,
		id, businessID))
}

// GetMenuBySlug fetches a menu of the business by its slug.
func GetMenuBySlug(ctx context.Context, db DB, businessID uuid.UUID, slug string) (*models.Menu, error) {
	return scanMenu(db.QueryRow(ctx,
		`SELECT `+menuColumns+` FROM menus WHERE business_id = $1 AND slug = lower($2)`,
		businessID, strings.TrimSpace(slug)))
}

// GetDefaultMenu returns the menu the business serves when no menu was asked
// for. It falls back to the first menu by position so a business whose default
// flag was lost still resolves; whether the menu is published is decided by the
// caller.
func GetDefaultMenu(ctx context.Context, db DB, businessID uuid.UUID) (*models.Menu, error) {
	return scanMenu(db.QueryRow(ctx, `
		SELECT `+menuColumns+` FROM menus
		WHERE business_id = $1
		ORDER BY is_default DESC, position ASC, created_at ASC
		LIMIT 1`, businessID))
}

// CreateMenu appends a menu to the end of the list. The first menu of a
// business automatically becomes its default one.
func CreateMenu(ctx context.Context, db DB, businessID uuid.UUID,
	name, slug, description string, isActive bool) (*models.Menu, error) {

	var (
		nextPosition int
		hasMenu      bool
	)
	err := db.QueryRow(ctx, `
		SELECT COALESCE(MAX(position) + 1, 0), COUNT(*) > 0
		FROM menus WHERE business_id = $1`, businessID).Scan(&nextPosition, &hasMenu)
	if err != nil {
		return nil, err
	}

	menu, err := scanMenu(db.QueryRow(ctx, `
		INSERT INTO menus (business_id, name, slug, description, is_default, is_active, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+menuColumns,
		businessID, name, slug, description, !hasMenu, isActive, nextPosition))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("could not create the menu: %w", err)
	}
	return menu, nil
}

var menuUpdatableColumns = map[string]bool{
	"name": true, "slug": true, "description": true,
	"is_default": true, "is_active": true, "position": true,
}

// UpdateMenu applies a partial update to the given columns. Promoting a menu to
// the default clears the previous default first, because menus_one_default_idx
// allows only one default menu per business.
func UpdateMenu(ctx context.Context, db DB, id, businessID uuid.UUID,
	fields map[string]any) (*models.Menu, error) {

	columns := make([]string, 0, len(fields))
	for column := range fields {
		if menuUpdatableColumns[column] {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return GetMenu(ctx, db, id, businessID)
	}
	sort.Strings(columns)

	if isDefault, ok := fields["is_default"].(bool); ok && isDefault {
		if _, err := db.Exec(ctx,
			`UPDATE menus SET is_default = false
			 WHERE business_id = $1 AND id <> $2 AND is_default`, businessID, id); err != nil {
			return nil, err
		}
	}

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+2)
	for i, column := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, i+1))
		args = append(args, fields[column])
	}
	args = append(args, id, businessID)

	query := `UPDATE menus SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d AND business_id = $%d RETURNING `, len(args)-1, len(args)) +
		menuColumns

	menu, err := scanMenu(db.QueryRow(ctx, query, args...))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return menu, nil
}

// DeleteMenu removes a menu. A business must keep at least one menu, so the
// last one is refused with ErrLastMenu.
//
// NOTE: categories.menu_id cascades, so deleting a menu also deletes its
// categories and their products. The handler warns the user before calling it.
func DeleteMenu(ctx context.Context, db DB, id, businessID uuid.UUID) error {
	var total int
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM menus WHERE business_id = $1`, businessID).Scan(&total); err != nil {
		return err
	}
	if total <= 1 {
		return ErrLastMenu
	}

	tag, err := db.Exec(ctx,
		`DELETE FROM menus WHERE id = $1 AND business_id = $2`, id, businessID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	// The business must keep a default menu, otherwise the public lookup has
	// nothing to fall back to.
	_, err = db.Exec(ctx, `
		UPDATE menus SET is_default = true
		WHERE id = (
			SELECT id FROM menus WHERE business_id = $1
			ORDER BY position ASC, created_at ASC LIMIT 1
		)
		AND NOT EXISTS (SELECT 1 FROM menus WHERE business_id = $1 AND is_default)`,
		businessID)
	return err
}

// MenuSlugTaken reports whether another menu of the same business uses the slug.
func MenuSlugTaken(ctx context.Context, db DB, businessID uuid.UUID,
	slug string, exceptID uuid.UUID) (bool, error) {

	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM menus WHERE business_id = $1 AND slug = $2 AND id <> $3
		)`,
		businessID, strings.ToLower(strings.TrimSpace(slug)), exceptID).Scan(&exists)
	return exists, err
}

// ------------------------------------------------------------- branches

// ListBranches returns the branches of a business ordered by position, each
// with the ids of the menus it serves.
func ListBranches(ctx context.Context, db DB, businessID uuid.UUID) ([]models.Branch, error) {
	rows, err := db.Query(ctx,
		`SELECT `+branchColumns+` FROM branches WHERE business_id = $1
		 ORDER BY position ASC, created_at ASC`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	branches := make([]models.Branch, 0)
	for rows.Next() {
		var branch models.Branch
		if err := rows.Scan(&branch.ID, &branch.BusinessID, &branch.Name, &branch.Slug,
			&branch.Phone, &branch.Address, &branch.WifiSSID, &branch.WifiPassword,
			&branch.IsDefault, &branch.IsActive, &branch.Position,
			&branch.CreatedAt, &branch.UpdatedAt); err != nil {
			return nil, err
		}
		branch.MenuIDs = []uuid.UUID{}
		branches = append(branches, branch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := attachBranchMenuIDs(ctx, db, branches); err != nil {
		return nil, err
	}
	return branches, nil
}

// attachBranchMenuIDs fills MenuIDs on every branch with a single extra query.
func attachBranchMenuIDs(ctx context.Context, db DB, branches []models.Branch) error {
	if len(branches) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(branches))
	indexByID := make(map[uuid.UUID]int, len(branches))
	for i, branch := range branches {
		ids = append(ids, branch.ID)
		indexByID[branch.ID] = i
	}

	rows, err := db.Query(ctx, `
		SELECT branch_id, menu_id FROM branch_menus
		WHERE branch_id = ANY($1::uuid[])
		ORDER BY position ASC, menu_id ASC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var branchID, menuID uuid.UUID
		if err := rows.Scan(&branchID, &menuID); err != nil {
			return err
		}
		if index, ok := indexByID[branchID]; ok {
			branches[index].MenuIDs = append(branches[index].MenuIDs, menuID)
		}
	}
	return rows.Err()
}

// GetBranch fetches a branch and verifies that it belongs to the business.
func GetBranch(ctx context.Context, db DB, id, businessID uuid.UUID) (*models.Branch, error) {
	branch, err := scanBranch(db.QueryRow(ctx,
		`SELECT `+branchColumns+` FROM branches WHERE id = $1 AND business_id = $2`,
		id, businessID))
	if err != nil {
		return nil, err
	}

	single := []models.Branch{*branch}
	if err := attachBranchMenuIDs(ctx, db, single); err != nil {
		return nil, err
	}
	branch.MenuIDs = single[0].MenuIDs
	return branch, nil
}

// GetBranchBySlug fetches a branch by its globally unique slug. Only branches
// of a published business are returned — an inactive business stays invisible
// exactly like it does in GetBusinessBySlug.
func GetBranchBySlug(ctx context.Context, db DB, slug string) (*models.Branch, error) {
	return scanBranch(db.QueryRow(ctx, `
		SELECT `+branchColumnsBr+`
		FROM branches br
		JOIN businesses b ON b.id = br.business_id
		WHERE br.slug = lower($1) AND br.is_active = true AND b.is_active = true`,
		strings.TrimSpace(slug)))
}

// CreateBranch appends a branch to the end of the list. The first branch of a
// business automatically becomes its default one.
func CreateBranch(ctx context.Context, db DB, businessID uuid.UUID,
	in models.Branch) (*models.Branch, error) {

	var (
		nextPosition int
		hasBranch    bool
	)
	err := db.QueryRow(ctx, `
		SELECT COALESCE(MAX(position) + 1, 0), COUNT(*) > 0
		FROM branches WHERE business_id = $1`, businessID).Scan(&nextPosition, &hasBranch)
	if err != nil {
		return nil, err
	}

	isDefault := in.IsDefault || !hasBranch
	if isDefault {
		// branches_one_default_idx allows only one default per business.
		if _, err := db.Exec(ctx,
			`UPDATE branches SET is_default = false
			 WHERE business_id = $1 AND is_default`, businessID); err != nil {
			return nil, err
		}
	}

	branch, err := scanBranch(db.QueryRow(ctx, `
		INSERT INTO branches (business_id, name, slug, phone, address, wifi_ssid,
		                      wifi_password, is_default, is_active, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+branchColumns,
		businessID, in.Name, in.Slug, in.Phone, in.Address, in.WifiSSID,
		in.WifiPassword, isDefault, in.IsActive, nextPosition))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("could not create the branch: %w", err)
	}
	return branch, nil
}

var branchUpdatableColumns = map[string]bool{
	"name": true, "slug": true, "phone": true, "address": true,
	"wifi_ssid": true, "wifi_password": true,
	"is_default": true, "is_active": true, "position": true,
}

// UpdateBranch applies a partial update to the given columns. Promoting a
// branch to the default clears the previous default first.
func UpdateBranch(ctx context.Context, db DB, id, businessID uuid.UUID,
	fields map[string]any) (*models.Branch, error) {

	columns := make([]string, 0, len(fields))
	for column := range fields {
		if branchUpdatableColumns[column] {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return GetBranch(ctx, db, id, businessID)
	}
	sort.Strings(columns)

	if isDefault, ok := fields["is_default"].(bool); ok && isDefault {
		if _, err := db.Exec(ctx,
			`UPDATE branches SET is_default = false
			 WHERE business_id = $1 AND id <> $2 AND is_default`, businessID, id); err != nil {
			return nil, err
		}
	}

	setParts := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)+2)
	for i, column := range columns {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", column, i+1))
		args = append(args, fields[column])
	}
	args = append(args, id, businessID)

	query := `UPDATE branches SET ` + strings.Join(setParts, ", ") +
		fmt.Sprintf(` WHERE id = $%d AND business_id = $%d RETURNING `, len(args)-1, len(args)) +
		branchColumns

	branch, err := scanBranch(db.QueryRow(ctx, query, args...))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}

	single := []models.Branch{*branch}
	if err := attachBranchMenuIDs(ctx, db, single); err != nil {
		return nil, err
	}
	branch.MenuIDs = single[0].MenuIDs
	return branch, nil
}

// DeleteBranch removes a branch together with its menu links and price
// overrides, which cascade.
func DeleteBranch(ctx context.Context, db DB, id, businessID uuid.UUID) error {
	tag, err := db.Exec(ctx,
		`DELETE FROM branches WHERE id = $1 AND business_id = $2`, id, businessID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	// Keep a default branch as long as the business still has any.
	_, err = db.Exec(ctx, `
		UPDATE branches SET is_default = true
		WHERE id = (
			SELECT id FROM branches WHERE business_id = $1
			ORDER BY position ASC, created_at ASC LIMIT 1
		)
		AND NOT EXISTS (SELECT 1 FROM branches WHERE business_id = $1 AND is_default)`,
		businessID)
	return err
}

// BranchSlugTaken reports whether a branch slug is already in use. Branch slugs
// and business slugs share one namespace because both are subdomains, so the
// businesses table is checked too — except for the business that owns the
// branch being edited, whose own slug the 003 backfill already gave to its
// default branch.
func BranchSlugTaken(ctx context.Context, db DB, slug string, exceptID uuid.UUID) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM branches WHERE slug = $1 AND id <> $2)
		    OR EXISTS (
		        SELECT 1 FROM businesses b
		        WHERE b.slug = $1
		          AND NOT EXISTS (
		              SELECT 1 FROM branches br
		              WHERE br.id = $2 AND br.business_id = b.id
		          )
		    )`,
		strings.ToLower(strings.TrimSpace(slug)), exceptID).Scan(&exists)
	return exists, err
}

// --------------------------------------------------------- branch menus

// SetBranchMenus replaces the menus a branch serves. Menus that do not belong
// to the branch' business are ignored. defaultMenuID picks the menu the branch
// opens with; when it is not part of menuIDs the first one is used instead.
func SetBranchMenus(ctx context.Context, db DB, branchID uuid.UUID,
	menuIDs []uuid.UUID, defaultMenuID uuid.UUID) error {

	if menuIDs == nil {
		menuIDs = []uuid.UUID{}
	}

	if _, err := db.Exec(ctx, `
		DELETE FROM branch_menus
		WHERE branch_id = $1 AND NOT (menu_id = ANY($2::uuid[]))`,
		branchID, menuIDs); err != nil {
		return fmt.Errorf("could not clear the branch menus: %w", err)
	}

	// branch_menus_one_default_idx allows a single default per branch, so the
	// old flag has to go before a new one is set.
	if _, err := db.Exec(ctx,
		`UPDATE branch_menus SET is_default = false WHERE branch_id = $1 AND is_default`,
		branchID); err != nil {
		return err
	}

	for position, menuID := range menuIDs {
		if _, err := db.Exec(ctx, `
			INSERT INTO branch_menus (branch_id, menu_id, position, is_default)
			SELECT br.id, m.id, $3, false
			FROM branches br
			JOIN menus m ON m.business_id = br.business_id
			WHERE br.id = $1 AND m.id = $2
			ON CONFLICT (branch_id, menu_id)
			DO UPDATE SET position = EXCLUDED.position`,
			branchID, menuID, position); err != nil {
			return fmt.Errorf("could not link the menu to the branch: %w", err)
		}
	}

	// The requested default wins; otherwise the first linked menu takes over.
	_, err := db.Exec(ctx, `
		UPDATE branch_menus SET is_default = true
		WHERE branch_id = $1 AND menu_id = (
			SELECT menu_id FROM branch_menus
			WHERE branch_id = $1
			ORDER BY (menu_id = $2::uuid) DESC, position ASC, menu_id ASC
			LIMIT 1
		)`, branchID, defaultMenuID)
	return err
}

// ListBranchMenus returns every menu linked to a branch, the default one first.
func ListBranchMenus(ctx context.Context, db DB, branchID uuid.UUID) ([]models.Menu, error) {
	rows, err := db.Query(ctx, `
		SELECT `+menuColumnsM+`
		FROM branch_menus bm
		JOIN menus m ON m.id = bm.menu_id
		WHERE bm.branch_id = $1
		ORDER BY bm.is_default DESC, bm.position ASC, m.position ASC, m.created_at ASC`,
		branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menus := make([]models.Menu, 0)
	for rows.Next() {
		var menu models.Menu
		if err := rows.Scan(&menu.ID, &menu.BusinessID, &menu.Name, &menu.Slug,
			&menu.Description, &menu.IsDefault, &menu.IsActive, &menu.Position,
			&menu.CreatedAt, &menu.UpdatedAt); err != nil {
			return nil, err
		}
		menus = append(menus, menu)
	}
	return menus, rows.Err()
}

// DefaultMenuOfBranch returns the menu a branch opens with: the one flagged in
// branch_menus, otherwise its first published menu.
func DefaultMenuOfBranch(ctx context.Context, db DB, branchID uuid.UUID) (*models.Menu, error) {
	return scanMenu(db.QueryRow(ctx, `
		SELECT `+menuColumnsM+`
		FROM branch_menus bm
		JOIN menus m ON m.id = bm.menu_id
		WHERE bm.branch_id = $1 AND m.is_active = true
		ORDER BY bm.is_default DESC, bm.position ASC, m.position ASC, m.created_at ASC
		LIMIT 1`, branchID))
}

// -------------------------------------------------------- branch prices

// ListBranchPrices returns the price overrides of a branch keyed by product id.
func ListBranchPrices(ctx context.Context, db DB, branchID uuid.UUID) (map[uuid.UUID]models.BranchPrice, error) {
	rows, err := db.Query(ctx, `
		SELECT branch_id, product_id, price, compare_price, is_available
		FROM branch_product_prices WHERE branch_id = $1`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make(map[uuid.UUID]models.BranchPrice)
	for rows.Next() {
		var price models.BranchPrice
		if err := rows.Scan(&price.BranchID, &price.ProductID, &price.Price,
			&price.ComparePrice, &price.IsAvailable); err != nil {
			return nil, err
		}
		prices[price.ProductID] = price
	}
	return prices, rows.Err()
}

// UpsertBranchPrice writes one price override. A nil Price means the branch
// inherits the price of the product itself.
func UpsertBranchPrice(ctx context.Context, db DB, in models.BranchPrice) error {
	_, err := db.Exec(ctx, `
		INSERT INTO branch_product_prices (branch_id, product_id, price, compare_price, is_available)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (branch_id, product_id) DO UPDATE
		SET price = EXCLUDED.price,
		    compare_price = EXCLUDED.compare_price,
		    is_available = EXCLUDED.is_available`,
		in.BranchID, in.ProductID, in.Price, in.ComparePrice, in.IsAvailable)
	if err != nil {
		return fmt.Errorf("could not save the branch price: %w", err)
	}
	return nil
}

// DeleteBranchPrice drops one price override, so the branch falls back to the
// price of the product itself.
func DeleteBranchPrice(ctx context.Context, db DB, branchID, productID uuid.UUID) error {
	tag, err := db.Exec(ctx,
		`DELETE FROM branch_product_prices WHERE branch_id = $1 AND product_id = $2`,
		branchID, productID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
