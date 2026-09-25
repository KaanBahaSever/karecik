package repository

import (
	"context"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
)

// Display names of products and categories, for the places that show a record
// by name rather than by id: the labels of the audit trail and the top lists of
// the analytics summary.
//
// A name is resolved in the default language of the menu the record sits on —
// the language the owner writes the menu in, and the one every dashboard list
// already shows — through models.Translations.Resolve, so a record with no text
// in that language still gets the first name it has.

// ProductNames returns the names of the given products of the business, in the
// order of ids. An id the business does not own, or that no longer exists,
// comes back as "" in its place, so the result always lines up with ids.
func ProductNames(ctx context.Context, db DB, businessID uuid.UUID, ids []uuid.UUID) ([]string, error) {
	return namesInOrder(ctx, db, `
		SELECT data.ord, p.translations, m.default_language
		FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
		JOIN products p ON p.id = data.id AND p.business_id = $1
		JOIN categories c ON c.id = p.category_id AND c.business_id = $1
		JOIN menus m ON m.id = c.menu_id AND m.business_id = $1`, businessID, ids)
}

// CategoryNames is ProductNames for categories.
func CategoryNames(ctx context.Context, db DB, businessID uuid.UUID, ids []uuid.UUID) ([]string, error) {
	return namesInOrder(ctx, db, `
		SELECT data.ord, c.translations, m.default_language
		FROM unnest($2::uuid[]) WITH ORDINALITY AS data(id, ord)
		JOIN categories c ON c.id = data.id AND c.business_id = $1
		JOIN menus m ON m.id = c.menu_id AND m.business_id = $1`, businessID, ids)
}

// ProductName and CategoryName are the one-record forms.
func ProductName(ctx context.Context, db DB, businessID, id uuid.UUID) (string, error) {
	names, err := ProductNames(ctx, db, businessID, []uuid.UUID{id})
	if err != nil {
		return "", err
	}
	return names[0], nil
}

// CategoryName — see ProductName.
func CategoryName(ctx context.Context, db DB, businessID, id uuid.UUID) (string, error) {
	names, err := CategoryNames(ctx, db, businessID, []uuid.UUID{id})
	if err != nil {
		return "", err
	}
	return names[0], nil
}

// RemovedProductName names a product whose row a delete has just removed, from
// the copy the delete read before it: its translations resolved in the default
// language of the menu its category sits on. It runs inside the delete's
// transaction, where the product is gone but its category and menu are not.
func RemovedProductName(ctx context.Context, db DB, businessID uuid.UUID, product *models.Product) (string, error) {
	return nameInMenuLanguage(ctx, db, `
		SELECT m.default_language
		FROM categories c
		JOIN menus m ON m.id = c.menu_id AND m.business_id = $1
		WHERE c.id = $2 AND c.business_id = $1`, businessID, product.CategoryID, product.Translations)
}

// RemovedCategoryName is RemovedProductName for a category: its menu is still
// there to say the language.
func RemovedCategoryName(ctx context.Context, db DB, businessID uuid.UUID, category *models.Category) (string, error) {
	menuID := uuid.Nil
	if category.MenuID != nil {
		menuID = *category.MenuID
	}
	return nameInMenuLanguage(ctx, db, `
		SELECT default_language FROM menus WHERE id = $2 AND business_id = $1`,
		businessID, menuID, category.Translations)
}

// nameInMenuLanguage reads a menu's default language with one of the queries
// above and resolves translations in it. A menu that cannot be found leaves
// the language empty, and Resolve then takes the first name there is.
func nameInMenuLanguage(ctx context.Context, db DB, query string, businessID, id uuid.UUID,
	translations models.Translations) (string, error) {

	var language string
	if err := db.QueryRow(ctx, query, businessID, id).Scan(&language); err != nil && !isNoRows(err) {
		return "", err
	}
	return translations.Resolve(language, language).Name, nil
}

// namesInOrder runs one of the queries above — ordinal, translations, the
// menu's default language — and places each resolved name at its ordinal.
func namesInOrder(ctx context.Context, db DB, query string, businessID uuid.UUID,
	ids []uuid.UUID) ([]string, error) {

	names := make([]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := db.Query(ctx, query, businessID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			ordinal      int64
			translations models.Translations
			language     string
		)
		if err := rows.Scan(&ordinal, &translations, &language); err != nil {
			return nil, err
		}
		if ordinal >= 1 && int(ordinal) <= len(names) {
			names[ordinal-1] = translations.Resolve(language, language).Name
		}
	}
	return names, rows.Err()
}
