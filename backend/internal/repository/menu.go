package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// PublicMenuOptions scopes the customer menu payload.
type PublicMenuOptions struct {
	Lang            string
	IncludeInactive bool           // dashboard preview
	Branch          *models.Branch // nil = business-wide menu
	Menu            *models.Menu   // nil = the business' default menu
}

// BuildPublicMenu assembles the customer-facing menu payload.
// Translations are resolved into the requested language, so the translations
// map itself never leaves the server.
//
// opts.IncludeInactive = false -> only published categories/products (the real menu)
// opts.IncludeInactive = true  -> inactive records are included too (dashboard preview)
func BuildPublicMenu(ctx context.Context, db DB, business *models.Business,
	opts PublicMenuOptions) (*models.PublicMenu, error) {

	lang := resolveLanguage(business, opts.Lang)
	fallback := business.DefaultLanguage

	categoryQuery := `SELECT ` + categoryColumns + `
		FROM categories WHERE business_id = $1`
	categoryArgs := []any{business.ID}
	productQuery := `SELECT ` + productColumns + `
		FROM products WHERE business_id = $1`

	// A category that was never assigned to a menu still belongs to the
	// business, so it keeps showing up on the default menu.
	if opts.Menu != nil {
		categoryArgs = append(categoryArgs, opts.Menu.ID)
		if opts.Menu.IsDefault {
			categoryQuery += ` AND (menu_id = $2 OR menu_id IS NULL)`
		} else {
			categoryQuery += ` AND menu_id = $2`
		}
	}

	if !opts.IncludeInactive {
		categoryQuery += ` AND is_active = true`
		productQuery += ` AND is_active = true`
	}
	categoryQuery += ` ORDER BY position ASC, created_at ASC`
	productQuery += ` ORDER BY position ASC, created_at ASC`

	// --- branch pricing
	// A NULL override price means "inherit the product's own price"; an
	// unavailable product is dropped from the menu of that branch.
	var branchPrices map[uuid.UUID]models.BranchPrice
	if opts.Branch != nil {
		prices, err := ListBranchPrices(ctx, db, opts.Branch.ID)
		if err != nil {
			return nil, fmt.Errorf("could not read the branch prices: %w", err)
		}
		branchPrices = prices
	}

	// --- categories
	categoryRows, err := db.Query(ctx, categoryQuery, categoryArgs...)
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
	productRows, err := db.Query(ctx, productQuery, business.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read the products: %w", err)
	}

	for productRows.Next() {
		var product models.Product
		if err := productRows.Scan(&product.ID, &product.BusinessID, &product.CategoryID,
			&product.Translations, &product.Price, &product.ComparePrice, &product.Calories,
			&product.ImageURL, &product.Allergens, &product.Badges, &product.IsActive,
			&product.IsFeatured, &product.Position,
			&product.CreatedAt, &product.UpdatedAt); err != nil {
			productRows.Close()
			return nil, err
		}
		normalizeProduct(&product)

		index, ok := indexByID[product.CategoryID]
		if !ok {
			continue // a product whose category is inactive never shows up
		}

		price := product.Price
		comparePrice := product.ComparePrice
		if override, ok := branchPrices[product.ID]; ok {
			if !override.IsAvailable && !opts.IncludeInactive {
				continue // the branch does not serve this product
			}
			if override.Price != nil {
				price = *override.Price
			}
			if override.ComparePrice != nil {
				comparePrice = override.ComparePrice
			}
		}

		translation := product.Translations.Resolve(lang, fallback)

		categories[index].Products = append(categories[index].Products, models.PublicProduct{
			ID:           product.ID,
			Name:         translation.Name,
			Description:  translation.Description,
			Ingredients:  translation.Ingredients,
			Price:        utils.Round2(price),
			ComparePrice: comparePrice,
			Calories:     product.Calories,
			ImageURL:     product.ImageURL,
			Allergens:    product.Allergens,
			Badges:       product.Badges,
			IsFeatured:   product.IsFeatured,
			IsActive:     product.IsActive,
		})
	}
	productRows.Close()
	if err := productRows.Err(); err != nil {
		return nil, err
	}

	menus, err := publicMenuRefs(ctx, db, business, opts)
	if err != nil {
		return nil, err
	}

	return &models.PublicMenu{
		Business:   toPublicBusiness(business, opts),
		Categories: categories,
		Footer:     buildFooter(business),
		Menus:      menus,
	}, nil
}

// publicMenuRefs lists the menus reachable from this context: the menus of the
// branch when the request came through one, otherwise the menus of the
// business. The result is always non-nil and stays empty when there is nothing
// to switch between.
func publicMenuRefs(ctx context.Context, db DB, business *models.Business,
	opts PublicMenuOptions) ([]models.MenuRef, error) {

	var (
		menus []models.Menu
		err   error
	)
	if opts.Branch != nil {
		menus, err = ListBranchMenus(ctx, db, opts.Branch.ID)
	} else {
		menus, err = ListMenus(ctx, db, business.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the menus: %w", err)
	}

	refs := make([]models.MenuRef, 0, len(menus))
	for _, menu := range menus {
		if !menu.IsActive && !opts.IncludeInactive {
			continue
		}
		refs = append(refs, models.MenuRef{Slug: menu.Slug, Name: menu.Name})
	}
	if len(refs) < 2 {
		return make([]models.MenuRef, 0), nil
	}
	return refs, nil
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

func toPublicBusiness(business *models.Business, opts PublicMenuOptions) models.PublicBusiness {
	public := models.PublicBusiness{
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

		SplashEnabled:       business.SplashEnabled,
		SplashDuration:      business.SplashDuration,
		SplashBgColor:       business.SplashBgColor,
		SplashText:          business.SplashText,
		SplashLogoURL:       business.SplashLogoURL,
		SplashHeadline:      business.SplashHeadline,
		SplashExitAnimation: business.SplashExitAnimation,
		SplashExitDuration:  business.SplashExitDuration,
		SplashExitEasing:    business.SplashExitEasing,
		SplashDisplay:       business.SplashDisplay,

		BackgroundType:           business.BackgroundType,
		BackgroundColor:          business.BackgroundColor,
		BackgroundImageURL:       business.BackgroundImageURL,
		BackgroundOverlayOpacity: business.BackgroundOverlayOpacity,

		HeaderDisplay: business.HeaderDisplay,

		ShowVatNote:    business.ShowVatNote,
		VatNoteText:    business.VatNoteText,
		ShowPriceDate:  business.ShowPriceDate,
		PriceUpdatedAt: business.PriceUpdatedAt,

		Phone:        business.Phone,
		Address:      business.Address,
		Instagram:    business.Instagram,
		WifiSSID:     business.WifiSSID,
		WifiPassword: business.WifiPassword,
	}

	// A branch overrides the contact details it fills in itself; whatever it
	// leaves empty keeps falling back to the business.
	if branch := opts.Branch; branch != nil {
		if branch.Phone != nil {
			public.Phone = branch.Phone
		}
		if branch.Address != nil {
			public.Address = branch.Address
		}
		if branch.WifiSSID != nil {
			public.WifiSSID = branch.WifiSSID
		}
		if branch.WifiPassword != nil {
			public.WifiPassword = branch.WifiPassword
		}
		name, slug := branch.Name, branch.Slug
		public.BranchName = &name
		public.BranchSlug = &slug
	}
	if menu := opts.Menu; menu != nil {
		name, slug := menu.Name, menu.Slug
		public.MenuName = &name
		public.MenuSlug = &slug
	}
	return public
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
