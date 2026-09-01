package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// BuildPublicMenu assembles the customer-facing menu payload.
// Translations are resolved into the requested language, so the translations
// map itself never leaves the server.
//
// includeInactive = false -> only published categories/products (the real menu)
// includeInactive = true  -> inactive records are included too (dashboard preview)
func BuildPublicMenu(ctx context.Context, db DB, business *models.Business,
	lang string, includeInactive bool) (*models.PublicMenu, error) {

	lang = resolveLanguage(business, lang)
	fallback := business.DefaultLanguage

	categoryQuery := `SELECT ` + categoryColumns + `
		FROM categories WHERE business_id = $1`
	productQuery := `SELECT ` + productColumns + `
		FROM products WHERE business_id = $1`

	if !includeInactive {
		categoryQuery += ` AND is_active = true`
		productQuery += ` AND is_active = true`
	}
	categoryQuery += ` ORDER BY position ASC, created_at ASC`
	productQuery += ` ORDER BY position ASC, created_at ASC`

	// --- categories
	categoryRows, err := db.Query(ctx, categoryQuery, business.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the categories: %w", err)
	}

	categories := make([]models.PublicCategory, 0)
	indexByID := make(map[uuid.UUID]int)

	for categoryRows.Next() {
		var category models.Category
		if err := categoryRows.Scan(&category.ID, &category.BusinessID, &category.Translations,
			&category.Icon, &category.ImageURL, &category.Position, &category.IsActive,
			&category.CreatedAt, &category.UpdatedAt); err != nil {
			categoryRows.Close()
			return nil, err
		}
		if category.Translations == nil {
			category.Translations = models.Translations{}
		}
		translation := category.Translations.Resolve(lang, fallback)

		indexByID[category.ID] = len(categories)
		categories = append(categories, models.PublicCategory{
			ID:          category.ID,
			Name:        translation.Name,
			Description: translation.Description,
			Icon:        category.Icon,
			ImageURL:    category.ImageURL,
			IsActive:    category.IsActive,
			Products:    make([]models.PublicProduct, 0),
		})
	}
	categoryRows.Close()
	if err := categoryRows.Err(); err != nil {
		return nil, err
	}

	// --- products
	productRows, err := db.Query(ctx, productQuery, business.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the products: %w", err)
	}

	for productRows.Next() {
		var product models.Product
		if err := productRows.Scan(&product.ID, &product.BusinessID, &product.CategoryID,
			&product.Translations, &product.Price, &product.ComparePrice, &product.ImageURL,
			&product.Allergens, &product.IsActive, &product.IsFeatured, &product.Position,
			&product.CreatedAt, &product.UpdatedAt); err != nil {
			productRows.Close()
			return nil, err
		}
		normalizeProduct(&product)

		index, ok := indexByID[product.CategoryID]
		if !ok {
			continue // a product whose category is inactive never shows up
		}
		translation := product.Translations.Resolve(lang, fallback)

		categories[index].Products = append(categories[index].Products, models.PublicProduct{
			ID:           product.ID,
			Name:         translation.Name,
			Description:  translation.Description,
			Ingredients:  translation.Ingredients,
			Price:        utils.Round2(product.Price),
			ComparePrice: product.ComparePrice,
			ImageURL:     product.ImageURL,
			Allergens:    product.Allergens,
			IsFeatured:   product.IsFeatured,
			IsActive:     product.IsActive,
		})
	}
	productRows.Close()
	if err := productRows.Err(); err != nil {
		return nil, err
	}

	return &models.PublicMenu{
		Business:   toPublicBusiness(business),
		Categories: categories,
		Footer:     buildFooter(business),
	}, nil
}

// resolveLanguage validates the requested language, falling back to the default.
func resolveLanguage(business *models.Business, lang string) string {
	if lang == "" {
		return business.DefaultLanguage
	}
	for _, supported := range business.Languages {
		if supported == lang {
			return lang
		}
	}
	return business.DefaultLanguage
}

func toPublicBusiness(business *models.Business) models.PublicBusiness {
	return models.PublicBusiness{
		Name:            business.Name,
		Slug:            business.Slug,
		LogoURL:         business.LogoURL,
		CoverURL:        business.CoverURL,
		Currency:        business.Currency,
		CurrencySymbol:  utils.CurrencySymbol(business.Currency),
		Theme:           business.Theme,
		FontFamily:      business.FontFamily,
		PrimaryColor:    business.PrimaryColor,
		DefaultLanguage: business.DefaultLanguage,
		Languages:       business.Languages,
		SplashEnabled:   business.SplashEnabled,
		SplashDuration:  business.SplashDuration,
		SplashBgColor:   business.SplashBgColor,
		SplashText:      business.SplashText,
		ShowVatNote:     business.ShowVatNote,
		VatNoteText:     business.VatNoteText,
		ShowPriceDate:   business.ShowPriceDate,
		PriceUpdatedAt:  business.PriceUpdatedAt,
		Phone:           business.Phone,
		Address:         business.Address,
		Instagram:       business.Instagram,
		WifiPassword:    business.WifiPassword,
	}
}

// buildFooter produces the legal notices at the bottom of the menu.
// The price date refreshes automatically after every bulk price update.
//
// NOTE: the wording is customer-facing and therefore Turkish on purpose.
func buildFooter(business *models.Business) models.PublicFooter {
	footer := models.PublicFooter{PoweredBy: "Karecik ile hazırlandı"}

	if business.ShowPriceDate {
		footer.PriceNote = fmt.Sprintf(
			"Fiyatlarımız %s tarihinden itibaren geçerlidir.",
			business.PriceUpdatedAt.Local().Format("02.01.2006"))
	}
	if business.ShowVatNote {
		footer.VatNote = business.VatNoteText
	}
	return footer
}
