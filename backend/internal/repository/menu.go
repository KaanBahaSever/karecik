package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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
	// exactly one menu and nothing has to be inherited from another one.
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
		Business:     ToPublicBusiness(business, menu),
		Categories:   categories,
		Footer:       BuildFooter(menu),
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

// ToPublicBusiness turns the menu into the header block of the customer
// payload. Name and Slug are the MENU's — the menu is the venue the customer
// opened — while BusinessName and BusinessSlug carry the tenant alongside them,
// so the two together spell {business_slug}.karecik.com/{menu_slug}.
//
// CurrencySymbol is the stored one, never re-derived: the repository is the
// only writer of that column, so what is stored is by definition what belongs
// to the currency.
func ToPublicBusiness(business *models.Business, menu *models.Menu) models.PublicBusiness {
	name := menu.Name

	// Only the entries that pass the link rules, and always an array — see
	// PublicLinks.
	links := PublicLinks(menu.Links)

	public := models.PublicBusiness{
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
		SplashEntrance:      menu.SplashEntrance,
		SplashExitAnimation: menu.SplashExitAnimation,
		SplashExitDuration:  menu.SplashExitDuration,
		SplashExitEasing:    menu.SplashExitEasing,
		SplashDisplay:       menu.SplashDisplay,
		SplashSlideFade:     menu.SplashSlideFade,

		BackgroundType:           menu.BackgroundType,
		BackgroundColor:          menu.BackgroundColor,
		BackgroundImageURL:       menu.BackgroundImageURL,
		BackgroundOverlayOpacity: menu.BackgroundOverlayOpacity,

		// Slogan travels as it is stored: '' means the header prints no
		// tagline at all, which is exactly what an owner who cleared the field
		// asked for.
		HeaderDisplay: menu.HeaderDisplay,
		LogoFadeIn:    menu.LogoFadeIn,
		Slogan:        menu.Slogan,

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

		ContactDisplay: menu.ContactDisplay,
		Links:          links,

		BusinessName: business.Name,
		BusinessSlug: business.Slug,
		MenuSlug:     menu.Slug,
		MenuName:     &name,
	}

	// "hidden" is the owner saying the contact block is not for customers, and
	// a customer can read this payload as easily as the page drawn from it — so
	// in that mode the entries are left out of the payload rather than merely
	// not drawn. The owner loses nothing: GET /api/menus/:id reads the menu, not
	// this payload. Address is not part of the contact block and stays. The
	// other three modes only move the block around the page, so they send it
	// whole.
	if menu.ContactDisplay == utils.ContactDisplayHidden {
		public.Phone = nil
		public.Instagram = nil
		public.WifiSSID = nil
		public.WifiPassword = nil
		public.Links = models.MenuLinks{}
	}

	return public
}

// PublicLinks is the list of links a customer receives: the stored entries that
// pass the link rules (utils.CheckMenuLink), in their stored order, with their
// label and url cleaned the way a save cleans them. The API refuses a list that
// breaks a rule, but a row written past the API can still hold one, and the
// owner's own endpoints return such a row as it is, so that the owner can see
// the entry and fix it. The payload a customer reads never carries it.
//
// At most utils.MaxMenuLinks entries are kept, the first ones that pass: the
// API never stores more, and a row that holds more was not written by it.
//
// Ids are never rewritten into new UUIDs here — a payload built twice from the
// same row has to come out the same, or every read would change its ETag.
// Instead every entry in the payload gets a unique id: the first entry to carry
// an id keeps it, and an entry whose id is blank or already used gets
// "link-<index>", its index in this list — or, when that id is itself taken,
// the next "link-<n>" that is free.
//
// The result is never nil, so the payload carries [] and not null.
func PublicLinks(stored models.MenuLinks) models.MenuLinks {
	links := make(models.MenuLinks, 0, len(stored))
	for _, link := range stored {
		label, address, message := utils.CheckMenuLink(link.Label, link.URL)
		if message != "" {
			continue
		}
		links = append(links, models.MenuLink{ID: link.ID, Label: label, URL: address})
		if len(links) == utils.MaxMenuLinks {
			break
		}
	}

	// Two passes, so that a generated "link-<n>" can never collide with an id
	// an entry further down the list already carries.
	keeps := make([]bool, len(links))
	used := make(map[string]bool, len(links))
	for i, link := range links {
		if strings.TrimSpace(link.ID) != "" && !used[link.ID] {
			keeps[i] = true
			used[link.ID] = true
		}
	}
	for i := range links {
		if keeps[i] {
			continue
		}
		for n := i; ; n++ {
			candidate := "link-" + strconv.Itoa(n)
			if !used[candidate] {
				links[i].ID = candidate
				used[candidate] = true
				break
			}
		}
	}
	return links
}

// toDirectoryBusiness is the header block when no menu resolved: the tenant
// identity and nothing else. Every menu-sourced setting stays at its zero
// value, because there is no menu to source it from — the directory page paints
// itself with the neutral theme defaults. Languages and Links are still empty
// slices so the payload carries [] instead of null.
func toDirectoryBusiness(business *models.Business) models.PublicBusiness {
	return models.PublicBusiness{
		BusinessName: business.Name,
		BusinessSlug: business.Slug,
		Languages:    []string{},
		Links:        models.MenuLinks{},
	}
}

// defaultVatNote is footer.vat_note when a menu shows the VAT note but its text
// is blank. It is the column default of menus.vat_note_text (migration 005), and
// the settings page leaves an empty field empty and shows this sentence only as
// its placeholder, so an owner who switches the note on without typing a text
// sees in advance the sentence the customer reads.
//
// NOTE: the wording is customer-facing and therefore Turkish on purpose.
const defaultVatNote = "Fiyatlarımıza KDV dahildir."

// BuildFooter produces the legal notices at the bottom of the menu.
//
// The price date is menus.price_updated_at. Nothing in Go writes it: the
// products_touch_menu_price_date trigger of migration 010 moves it whenever a
// price on that menu really changes — through the product dialog, the inline
// quick edit or a bulk update alike — and leaves it alone otherwise.
//
// It is formatted in utils.Istanbul and never in time.Local. The note names a
// day on the menu's calendar, while time.Local is only the zone this process
// runs in: on a server in UTC, a price changed between 00:00 and 03:00 Istanbul
// time would print the previous day.
//
// NOTE: the wording is customer-facing and therefore Turkish on purpose.
func BuildFooter(menu *models.Menu) models.PublicFooter {
	footer := models.PublicFooter{PoweredBy: poweredBy}

	if menu.ShowPriceDate {
		footer.PriceNote = fmt.Sprintf(
			"Fiyatlarımız %s tarihinden itibaren geçerlidir.",
			menu.PriceUpdatedAt.In(utils.Istanbul).Format("02.01.2006"))
	}
	// The VAT note is the trimmed text, or defaultVatNote when that is blank: a
	// note that is switched on is never printed empty, and an empty field prints
	// the sentence the settings page shows as its placeholder.
	if menu.ShowVatNote {
		footer.VatNote = strings.TrimSpace(menu.VatNoteText)
		if footer.VatNote == "" {
			footer.VatNote = defaultVatNote
		}
	}
	return footer
}
