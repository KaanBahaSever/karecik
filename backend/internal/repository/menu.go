package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// BuildPublicMenu, musteri tarafindaki menu govdesini hazirlar.
// Ceviriler istenen dile cozulur; translations sozlugu disari sizmaz.
//
// includeInactive = false  -> yalnizca yayindaki kategori/urunler (gercek menu)
// includeInactive = true   -> pasif kayitlar da doner (panel canli onizlemesi)
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

	// --- kategoriler
	catRows, err := db.Query(ctx, categoryQuery, business.ID)
	if err != nil {
		return nil, fmt.Errorf("kategoriler okunamadi: %w", err)
	}

	categories := make([]models.PublicCategory, 0)
	indexByID := make(map[uuid.UUID]int)

	for catRows.Next() {
		var c models.Category
		if err := catRows.Scan(&c.ID, &c.BusinessID, &c.Translations, &c.Icon, &c.ImageURL,
			&c.Position, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			catRows.Close()
			return nil, err
		}
		if c.Translations == nil {
			c.Translations = models.Translations{}
		}
		tr := c.Translations.Resolve(lang, fallback)

		indexByID[c.ID] = len(categories)
		categories = append(categories, models.PublicCategory{
			ID:          c.ID,
			Name:        tr.Name,
			Description: tr.Description,
			Icon:        c.Icon,
			ImageURL:    c.ImageURL,
			IsActive:    c.IsActive,
			Products:    make([]models.PublicProduct, 0),
		})
	}
	catRows.Close()
	if err := catRows.Err(); err != nil {
		return nil, err
	}

	// --- urunler
	prodRows, err := db.Query(ctx, productQuery, business.ID)
	if err != nil {
		return nil, fmt.Errorf("urunler okunamadi: %w", err)
	}

	for prodRows.Next() {
		var p models.Product
		if err := prodRows.Scan(&p.ID, &p.BusinessID, &p.CategoryID, &p.Translations, &p.Price,
			&p.ComparePrice, &p.ImageURL, &p.Allergens, &p.IsActive, &p.IsFeatured,
			&p.Position, &p.CreatedAt, &p.UpdatedAt); err != nil {
			prodRows.Close()
			return nil, err
		}
		normalizeProduct(&p)

		idx, ok := indexByID[p.CategoryID]
		if !ok {
			continue // kategorisi pasif olan urun menude gorunmez
		}
		tr := p.Translations.Resolve(lang, fallback)

		categories[idx].Products = append(categories[idx].Products, models.PublicProduct{
			ID:           p.ID,
			Name:         tr.Name,
			Description:  tr.Description,
			Ingredients:  tr.Ingredients,
			Price:        utils.Round2(p.Price),
			ComparePrice: p.ComparePrice,
			ImageURL:     p.ImageURL,
			Allergens:    p.Allergens,
			IsFeatured:   p.IsFeatured,
			IsActive:     p.IsActive,
		})
	}
	prodRows.Close()
	if err := prodRows.Err(); err != nil {
		return nil, err
	}

	return &models.PublicMenu{
		Business:   toPublicBusiness(business),
		Categories: categories,
		Footer:     buildFooter(business),
	}, nil
}

// resolveLanguage, istenen dili dogrular; desteklenmiyorsa varsayilana duser.
func resolveLanguage(b *models.Business, lang string) string {
	if lang == "" {
		return b.DefaultLanguage
	}
	for _, l := range b.Languages {
		if l == lang {
			return lang
		}
	}
	return b.DefaultLanguage
}

func toPublicBusiness(b *models.Business) models.PublicBusiness {
	return models.PublicBusiness{
		Name:            b.Name,
		Slug:            b.Slug,
		LogoURL:         b.LogoURL,
		CoverURL:        b.CoverURL,
		Currency:        b.Currency,
		CurrencySymbol:  utils.CurrencySymbol(b.Currency),
		Theme:           b.Theme,
		FontFamily:      b.FontFamily,
		PrimaryColor:    b.PrimaryColor,
		DefaultLanguage: b.DefaultLanguage,
		Languages:       b.Languages,
		SplashEnabled:   b.SplashEnabled,
		SplashDuration:  b.SplashDuration,
		SplashBgColor:   b.SplashBgColor,
		SplashText:      b.SplashText,
		ShowVatNote:     b.ShowVatNote,
		VatNoteText:     b.VatNoteText,
		ShowPriceDate:   b.ShowPriceDate,
		PriceUpdatedAt:  b.PriceUpdatedAt,
		Phone:           b.Phone,
		Address:         b.Address,
		Instagram:       b.Instagram,
		WifiPassword:    b.WifiPassword,
	}
}

// buildFooter, menunun altindaki yasal ibareleri sistem tarafindan uretir.
// Fiyat tarihi her toplu guncellemede otomatik tazelenir.
func buildFooter(b *models.Business) models.PublicFooter {
	footer := models.PublicFooter{PoweredBy: "Karecik ile hazırlandı"}

	if b.ShowPriceDate {
		footer.PriceNote = fmt.Sprintf(
			"Fiyatlarımız %s tarihinden itibaren geçerlidir.",
			b.PriceUpdatedAt.Local().Format("02.01.2006"))
	}
	if b.ShowVatNote {
		footer.VatNote = b.VatNoteText
	}
	return footer
}
