package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// poweredBy is the footer credit of every customer-facing payload, the tenant
// directory included.
//
// NOTE: the wording is customer-facing and therefore Turkish on purpose.
const poweredBy = "Karecik ile hazırlandı"

// PublicMenuOptions scopes the customer menu payload.
type PublicMenuOptions struct {
	Lang            string
	IncludeInactive bool // dashboard preview
}

// BuildPublicMenu assembles the customer-facing payload of one menu.
// Translations are resolved into the requested language, so the translations
// map itself never leaves the server.
//
// It takes both halves of the address: the business identifies the tenant —
// the subdomain of {business-slug}.karecik.com — and the menu identifies what
// is served under it. The menu owns every branding and contact setting of the
// payload and its categories are selected strictly by menu_id; the business
// only lends its name and slug, so the customer frontend can spell the real
// address without a second request.
//
// opts.IncludeInactive = false -> only published categories/products (the real menu)
// opts.IncludeInactive = true  -> inactive records are included too (dashboard preview)
func BuildPublicMenu(ctx context.Context, db DB, business *models.Business,
	menu *models.Menu, opts PublicMenuOptions) (*models.PublicMenu, error) {

	lang := resolveLanguage(menu, opts.Lang)
	fallback := menu.DefaultLanguage

	// categories.menu_id is NOT NULL since 005, so a category belongs to
	// exactly one menu and nothing has to be inherited any more.
	categoryQuery := `SELECT ` + categoryColumns + `
		FROM categories WHERE business_id = $1 AND menu_id = $2`

	// The products are reached through their category, which is what scopes
	// them to this menu.
	productQuery := `
		SELECT p.id, p.business_id, p.category_id, p.translations, p.price,
		       p.compare_price, p.calories, p.image_url, p.allergens, p.badges,
		       p.options, p.is_active, p.is_featured, p.position,
		       p.created_at, p.updated_at
		FROM products p
		JOIN categories c ON c.id = p.category_id
		WHERE p.business_id = $1 AND c.menu_id = $2`

	if !opts.IncludeInactive {
		categoryQuery += ` AND is_active = true`
		productQuery += ` AND p.is_active = true`
	}
	categoryQuery += ` ORDER BY position ASC, created_at ASC`
	productQuery += ` ORDER BY p.position ASC, p.created_at ASC`

	// --- categories
	categoryRows, err := db.Query(ctx, categoryQuery, menu.BusinessID, menu.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the categories: %w", err)
	}

	categories := make([]models.PublicCategory, 0)
	indexByID := make(map[uuid.UUID]int)

	for categoryRows.Next() {
		var category models.Category
		if err := categoryRows.Scan(&category.ID, &category.BusinessID, &category.MenuID,
			&category.Translations, &category.Icon, &category.ImageURL,
			&category.Position, &category.IsActive,
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
	productRows, err := db.Query(ctx, productQuery, menu.BusinessID, menu.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the products: %w", err)
	}

	for productRows.Next() {
		var product models.Product
		if err := productRows.Scan(&product.ID, &product.BusinessID, &product.CategoryID,
			&product.Translations, &product.Price, &product.ComparePrice, &product.Calories,
			&product.ImageURL, &product.Allergens, &product.Badges, &product.Options,
			&product.IsActive, &product.IsFeatured, &product.Position,
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
			Calories:     product.Calories,
			ImageURL:     product.ImageURL,
			Allergens:    product.Allergens,
			Badges:       product.Badges,
			Options:      product.Options,
			IsFeatured:   product.IsFeatured,
			IsActive:     product.IsActive,
		})
	}
	productRows.Close()
	if err := productRows.Err(); err != nil {
		return nil, err
	}

	menus, err := publicMenuRefs(ctx, db, business.ID, opts)
	if err != nil {
		return nil, err
	}

	return &models.PublicMenu{
		Business:     toPublicBusiness(business, menu),
		Categories:   categories,
		Footer:       buildFooter(menu),
		Menus:        menus,
		MenuResolved: true,
	}, nil
}

// BuildPublicDirectory assembles the payload of a tenant whose request did not
// resolve to a single menu: the subdomain was right but the path named no menu
// and the business publishes zero menus, or two and more.
//
// It is a 200 with menu_resolved false, an empty category list and the menu
// list — never a 404, because the business itself exists. That payload is what
// makes the frontend render the tenant directory (or its "no active menus"
// placeholder) instead of a menu.
func BuildPublicDirectory(ctx context.Context, db DB, business *models.Business,
	opts PublicMenuOptions) (*models.PublicMenu, error) {

	menus, err := publicMenuRefs(ctx, db, business.ID, opts)
	if err != nil {
		return nil, err
	}

	return &models.PublicMenu{
		Business:     toDirectoryBusiness(business),
		Categories:   make([]models.PublicCategory, 0),
		Footer:       models.PublicFooter{PoweredBy: poweredBy},
		Menus:        menus,
		MenuResolved: false,
	}, nil
}

// publicMenuRefs lists the menus of the tenant — the current one included, when
// there is one — so the customer menu can render a switcher and the directory
// page can render its cards. The result is always non-nil and every entry is a
// real address: {business-slug}.karecik.com/{slug}.
//
// The owner's own preview also sees the unpublished ones; a customer never
// does.
func publicMenuRefs(ctx context.Context, db DB, businessID uuid.UUID,
	opts PublicMenuOptions) ([]models.PublicMenuRef, error) {

	var (
		menus []models.Menu
		err   error
	)
	if opts.IncludeInactive {
		menus, err = ListMenus(ctx, db, businessID)
	} else {
		menus, err = ListActiveMenus(ctx, db, businessID)
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the menus: %w", err)
	}

	refs := make([]models.PublicMenuRef, 0, len(menus))
	for _, sibling := range menus {
		refs = append(refs, models.PublicMenuRef{
			Slug:        sibling.Slug,
			Name:        sibling.Name,
			Description: sibling.Description,
		})
	}
	return refs, nil
}

// resolveLanguage validates the requested language, falling back to the default
// language of the menu.
func resolveLanguage(menu *models.Menu, lang string) string {
	if lang == "" {
		return menu.DefaultLanguage
	}
	for _, supported := range menu.Languages {
		if supported == lang {
			return lang
		}
	}
	return menu.DefaultLanguage
}

// toPublicBusiness turns the menu into the header block of the customer
// payload. Name and Slug are the MENU's — the menu is the venue the customer
// opened — while BusinessName and BusinessSlug carry the tenant alongside them,
// so the two together spell {business_slug}.karecik.com/{menu_slug}.
//
// CurrencySymbol is the stored one, never re-derived: the repository is the
// only writer of that column, so what is stored is by definition what belongs
// to the currency.
func toPublicBusiness(business *models.Business, menu *models.Menu) models.PublicBusiness {
	name := menu.Name

	return models.PublicBusiness{
		Name:            menu.Name,
		Slug:            menu.Slug,
		LogoURL:         menu.LogoURL,
		CoverURL:        menu.CoverURL,
		Currency:        menu.Currency,
		CurrencySymbol:  menu.CurrencySymbol,
		Theme:           menu.Theme,
		FontFamily:      menu.FontFamily,
		PrimaryColor:    menu.PrimaryColor,
		DefaultLanguage: menu.DefaultLanguage,
		Languages:       menu.Languages,

		SplashEnabled:       menu.SplashEnabled,
		SplashDuration:      menu.SplashDuration,
		SplashBgColor:       menu.SplashBgColor,
		SplashText:          menu.SplashText,
		SplashLogoURL:       menu.SplashLogoURL,
		SplashHeadline:      menu.SplashHeadline,
		SplashExitAnimation: menu.SplashExitAnimation,
		SplashExitDuration:  menu.SplashExitDuration,
		SplashExitEasing:    menu.SplashExitEasing,
		SplashDisplay:       menu.SplashDisplay,
		SplashSlideFade:     menu.SplashSlideFade,

		BackgroundType:           menu.BackgroundType,
		BackgroundColor:          menu.BackgroundColor,
		BackgroundImageURL:       menu.BackgroundImageURL,
		BackgroundOverlayOpacity: menu.BackgroundOverlayOpacity,

		HeaderDisplay: menu.HeaderDisplay,
		LogoFadeIn:    menu.LogoFadeIn,

		// The customer view tints its text with TextColor and builds the legal
		// footer from the other two. YerliUretimLogoURL is passed through as it
		// is stored — nil means "no artwork", which is what makes the footer
		// fall back to a plain text pill instead of drawing the official mark.
		TextColor:          menu.TextColor,
		ShowYerliUretim:    menu.ShowYerliUretim,
		YerliUretimLogoURL: menu.YerliUretimLogoURL,

		ShowVatNote:    menu.ShowVatNote,
		VatNoteText:    menu.VatNoteText,
		ShowPriceDate:  menu.ShowPriceDate,
		PriceUpdatedAt: menu.PriceUpdatedAt,

		Phone:        menu.Phone,
		Address:      menu.Address,
		Instagram:    menu.Instagram,
		WifiSSID:     menu.WifiSSID,
		WifiPassword: menu.WifiPassword,

		BusinessName: business.Name,
		BusinessSlug: business.Slug,
		MenuSlug:     menu.Slug,
		MenuName:     &name,
	}
}

// toDirectoryBusiness is the header block when no menu resolved: the tenant
// identity and nothing else. Every menu-sourced setting stays at its zero
// value, because there is no menu to source it from — the directory page paints
// itself with the neutral theme defaults. Languages is still an empty slice so
// the payload carries [] instead of null.
func toDirectoryBusiness(business *models.Business) models.PublicBusiness {
	return models.PublicBusiness{
		BusinessName: business.Name,
		BusinessSlug: business.Slug,
		Languages:    []string{},
	}
}

// buildFooter produces the legal notices at the bottom of the menu.
// The price date refreshes automatically after every bulk price update of that
// menu.
//
// NOTE: the wording is customer-facing and therefore Turkish on purpose.
func buildFooter(menu *models.Menu) models.PublicFooter {
	footer := models.PublicFooter{PoweredBy: poweredBy}

	if menu.ShowPriceDate {
		footer.PriceNote = fmt.Sprintf(
			"Fiyatlarımız %s tarihinden itibaren geçerlidir.",
			menu.PriceUpdatedAt.Local().Format("02.01.2006"))
	}
	if menu.ShowVatNote {
		footer.VatNote = menu.VatNoteText
	}
	return footer
}
