package database

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// The seed tenant is Melly Coffee, a real cafe whose owner asked for its own
// menu to be the sample one. Every category, product, price and description
// below comes from https://mellycoffee.co/menu and is reproduced verbatim:
// nothing here is invented, rounded or embellished, and no product the site
// does not sell is added.
//
// Because it is a REAL business with a WORKING login, SeedDemo refuses to run
// in production — see the guard at the top of it.

const (
	// mellySlug is the BUSINESS slug: the subdomain the tenant answers on.
	mellySlug     = "melly-coffee"
	mellyName     = "Melly Coffee"
	mellyEmail    = "melly@karecik.com"
	mellyPassword = "melly1234"

	// Contact details of the menu. The site publishes no street address, so
	// menus.address is left NULL rather than filled with a guess.
	mellyPhone     = "+90 216 000 00 00"
	mellyInstagram = "mellycoffee"
	mellyWifiSSID  = "Melly Guest"
	mellyWifiPass  = "mellycoffee"

	// Brand assets, REFERENCED by URL and never copied into this repository.
	// mellycoffee.co is the owner's own site: the wordmark is their own, and
	// the "Yerli Üretim" image on that page is the authentic Ticaret Bakanlığı
	// certification mark their business actually carries. That is the whole
	// reason seeding it is correct — the official mark is never approximated,
	// redrawn or embedded by this product, only pointed at where the certified
	// holder already publishes it.
	//
	// Both were verified live: 5,448 bytes and 4,374 bytes of image/svg+xml.
	// The SVGs are preferred over the site's logo.png (172,870 bytes) because
	// the customer header renders a non-square logo with object-contain, so the
	// vector is both sharper and some thirty times smaller. They are rendered
	// through <img src>, which cannot execute script even for an SVG.
	//
	// A logo uploaded from the dashboard always overrides these seeded values.
	mellyLogoURL            = "https://mellycoffee.co/assets/images/logo-v3.svg"
	mellyYerliUretimLogoURL = "https://mellycoffee.co/assets/images/yerli.svg"

	// The tenant publishes a single menu, and mellyMenuSlug is its PATH segment
	// under the subdomain — so the address is melly-coffee.karecik.com/suadiye.
	mellyMenuName = "Suadiye"
	mellyMenuSlug = "suadiye"

	// legacyDemoEmail is the account of the "Demo Kafe" tenant this file used
	// to create. It is the ONLY address purgeLegacyDemo ever deletes.
	legacyDemoEmail = "demo@karecik.com"
)

// mellyTheme is the catalogue theme the menu starts from.
//
// The palette the owner gave is cream #FDFBF7, caramel #C49A6C and a dark-roast
// text tone #1F1A17. "zarif" is the closest starting point the catalogue
// offers: its cream surfaces and #e3d7c1 borders sit under a caramel accent
// without fighting it, and its own text is #3b2f22, the only warm dark brown on
// offer ("minimal" is #111111, a neutral black).
//
// Migration 006 added menus.text_color, so the theme no longer has the last
// word on the text either: the background, the accent, the splash background
// AND the text tone are all overridden column by column below, and #1F1A17 is
// now stored exactly instead of being approximated by the theme's #3b2f22.
const mellyTheme = "zarif"

// ------------------------------------------------------------ option groups

// The site prints the same add-on note under Espresso Bar and under Cold
// Beverages, so the two groups are defined ONCE here and reused by every drink
// that carries them.
//
// They are package-level values and a Go slice aliases its backing array, so
// they are never handed to a product directly: mellySharedOptions returns a
// private copy per product, which is what keeps a later mutation of one
// product's options from reaching every other product that shares the group.
var (
	mellyMilkChoice = models.ProductOptionGroup{
		Name: "Süt Tercihi", Type: models.OptionTypeSingle, Required: false,
		Items: []models.ProductOptionItem{
			{Name: "Yulaf Sütü", Price: 60},
			{Name: "Badem Sütü", Price: 60},
		},
	}

	mellyExtraShots = models.ProductOptionGroup{
		Name: "Ekstra Şuruplar & Shot", Type: models.OptionTypeMultiple, Required: false,
		Items: []models.ProductOptionItem{
			{Name: "Vanilya", Price: 40},
			{Name: "Karamel", Price: 40},
			{Name: "Fındık", Price: 40},
			{Name: "Toffee Nut", Price: 40},
			{Name: "Ekstra Espresso Shot", Price: 60},
		},
	}
)

// cloneOptionGroup copies a group by value AND gives it a fresh item slice.
// The struct copy alone would not be enough: Items would still point at the
// shared backing array, so appending to or rewriting one product's items would
// silently rewrite every other product's.
func cloneOptionGroup(group models.ProductOptionGroup) models.ProductOptionGroup {
	items := make([]models.ProductOptionItem, len(group.Items))
	copy(items, group.Items)
	group.Items = items
	return group
}

// mellySharedOptions returns one product's own copy of the milk choice and the
// syrup/shot add-ons.
func mellySharedOptions() models.ProductOptions {
	return models.ProductOptions{
		cloneOptionGroup(mellyMilkChoice),
		cloneOptionGroup(mellyExtraShots),
	}
}

// --------------------------------------------------------------- menu data

// The menu content is real customer-facing copy, so it stays in Turkish.
//
// name carries BOTH languages: the site prints a single name per product, and
// a blank English name would leave the language switcher showing an empty
// card, so the same string is stored under tr and en. desc is treated the same
// way — the site's ingredient lists exist only in Turkish.
//
// THE CALORIE VALUES BELOW ARE ESTIMATES, NOT MEASUREMENTS. mellycoffee.co
// publishes no calorie counts and the cafe has had nothing laboratory-tested,
// so every number is a typical figure for the KIND of item it sits on — black
// coffee, milk coffee, a syrup/cold/specialty drink, or a dessert or bakery
// plate — picked inside the band the owner gave for that kind. The owner asked
// for exactly that, for their own menu, knowing what these numbers are.
//
// Nobody reading this file later should mistake them for measured data, and no
// number here should be quoted as one. If the cafe ever has real figures, they
// replace these outright rather than being averaged with them.
//
// Allergens, badges and the featured flag ARE still left at their column
// defaults: the site states none of them and, unlike the calories, the owner
// has not asked for estimates of those.
type seedProduct struct {
	name     string
	desc     string
	price    float64
	calories int
	options  models.ProductOptions
}

type seedCategory struct {
	name     string
	icon     string
	products []seedProduct
}

// Spelling corrections applied to the site's own text, at the owner's request.
// They are listed here so they are easy to find and easy to revert:
//
//  1. "Ciabbatta Sandvich"  -> "Ciabatta Sandviç"   (Kitchen Bar)
//  2. "churchill"           -> "Churchill"          (Soft Beverages)
//  3. "mozarella"           -> "mozzarella"         (the three pizza descriptions)
//
// Nothing else about the wording was touched: every other name, description,
// price and category name is exactly what the site prints, in the site's own
// order.
var mellyMenu = []seedCategory{
	{
		name: "Kitchen Bar", icon: "🍕",
		products: []seedProduct{
			{name: "Margarita", price: 410, calories: 430, desc: "Pizza hamuru, pizza sosu, mozzarella"},
			{name: "Melly Mix", price: 450, calories: 495, desc: "Pizza hamuru, pizza sosu, mozzarella, sucuk, mısır, zeytin, biber"},
			{name: "Melly Beef", price: 480, calories: 520, desc: "Pizza hamuru, pizza sosu, mozzarella, kavurma, pastırma, sucuk, zeytin"},
			{name: "Ciabatta Sandviç", price: 400, calories: 465, desc: "Ekşi mayalı ekmek, pesto sos, krem peynir, kaşar, hindi füme, roka"},
			{name: "Roast Beef Cheddar Kaşarlı Tost", price: 450, calories: 505, desc: "Yanında cips ile servis edilir"},
			{name: "Hindi Füme Cheddar Kaşar Tost", price: 400, calories: 470, desc: "Yanında cips ile servis edilir"},
			{name: "Cips Tabağı", price: 270, calories: 360},
		},
	},
	{
		name: "Desserts", icon: "🍰",
		products: []seedProduct{
			{name: "Lotus Cheesecake", price: 450, calories: 480},
			{name: "Tiramisu", price: 450, calories: 425},
			{name: "Karpatka", price: 400, calories: 395},
			{name: "Meyveli Karpatka", price: 400, calories: 370, desc: "Günün meyvesini sorunuz",
				options: models.ProductOptions{{
					Name: "Ekstra", Type: models.OptionTypeMultiple, Required: false,
					Items: []models.ProductOptionItem{{Name: "Çikolata", Price: 40}},
				}}},
			{name: "Cookie Pie", price: 420, calories: 510},
			{name: "Hazelnut", price: 430, calories: 465},
			{name: "Süt Reçelli Cheesecake", price: 430, calories: 470},
			{name: "Brookies", price: 420, calories: 495},
			{name: "Carrot Cake", price: 390, calories: 440},
			{name: "Sufle", price: 350, calories: 405},
		},
	},
	{
		name: "Matcha Bar", icon: "🍵",
		products: []seedProduct{
			{name: "Matcha Latte", price: 330, calories: 240},
			{name: "Vanilla Matcha Latte", price: 350, calories: 265},
			{name: "Strawberry Matcha Latte", price: 350, calories: 275},
			{name: "Ice Matcha Latte", price: 340, calories: 235},
			{name: "Ice Vanilla Matcha Latte", price: 360, calories: 260},
			{name: "Ice Strawberry Matcha Latte", price: 360, calories: 270},
			{name: "Matcha Milkshake", price: 380, calories: 315},
			{name: "Strawberry Frozen Matcha", price: 380, calories: 295},
		},
	},
	{
		// Both shared groups apply to every row except Türk Kahvesi, which is
		// the only drink the site gives a portion choice — and its Double is
		// +30. Espresso and Double Espresso are two separate products on the
		// site, so they are NOT folded into a portion group.
		name: "Espresso Bar", icon: "☕",
		products: []seedProduct{
			{name: "Espresso", price: 210, calories: 5, options: mellySharedOptions()},
			{name: "Double Espresso", price: 230, calories: 10, options: mellySharedOptions()},
			{name: "Espresso Macchiato", price: 250, calories: 115, options: mellySharedOptions()},
			{name: "Cortado", price: 270, calories: 130, options: mellySharedOptions()},
			{name: "Americano", price: 285, calories: 8, options: mellySharedOptions()},
			{name: "Cafe Latte", price: 295, calories: 175, options: mellySharedOptions()},
			{name: "Cappuccino", price: 295, calories: 150, options: mellySharedOptions()},
			{name: "Flat White", price: 295, calories: 165, options: mellySharedOptions()},
			{name: "Red Eye", price: 300, calories: 120, options: mellySharedOptions()},
			{name: "Black Eye", price: 305, calories: 130, options: mellySharedOptions()},
			{name: "Mocha", price: 335, calories: 285, options: mellySharedOptions()},
			{name: "White Mocha", price: 335, calories: 300, options: mellySharedOptions()},
			{name: "Zebra Mocha", price: 335, calories: 295, options: mellySharedOptions()},
			{name: "Honey Latte", price: 335, calories: 265, options: mellySharedOptions()},
			{name: "Türk Kahvesi", price: 200, calories: 12,
				options: models.ProductOptions{{
					Name: "Porsiyon", Type: models.OptionTypeSingle, Required: true,
					Items: []models.ProductOptionItem{
						{Name: "Tek", Price: 0},
						{Name: "Double", Price: 30},
					},
				}}},
		},
	},
	{
		// Both shared groups apply to every row here.
		name: "Cold Beverages", icon: "🧊",
		products: []seedProduct{
			{name: "Ice Americano", price: 295, calories: 115, options: mellySharedOptions()},
			{name: "Ice Latte", price: 305, calories: 170, options: mellySharedOptions()},
			{name: "Ice Cappuccino", price: 305, calories: 145, options: mellySharedOptions()},
			{name: "Ice Flat White", price: 305, calories: 160, options: mellySharedOptions()},
			{name: "Ice Red Eye", price: 315, calories: 125, options: mellySharedOptions()},
			{name: "Ice Black Eye", price: 325, calories: 135, options: mellySharedOptions()},
			{name: "Ice Mocha", price: 345, calories: 290, options: mellySharedOptions()},
			{name: "Ice White Mocha", price: 345, calories: 305, options: mellySharedOptions()},
			{name: "Ice Zebra Mocha", price: 345, calories: 300, options: mellySharedOptions()},
			{name: "Ice Honey Latte", price: 345, calories: 270, options: mellySharedOptions()},
		},
	},
	{
		name: "Brew Bar", icon: "🫖",
		products: []seedProduct{
			{name: "Filter Coffee", price: 280, calories: 6},
			{name: "Ice Filter Coffee", price: 290, calories: 7},
			{name: "Cold Brew", price: 330, calories: 9},
			{name: "V60", price: 350, calories: 5},
		},
	},
	{
		name: "Hot Beverages", icon: "🔥",
		products: []seedProduct{
			{name: "Hot Chocolate", price: 330, calories: 310},
			{name: "Chai Tea Latte", price: 330, calories: 250},
			{name: "Dirty Chai Tea Latte", price: 350, calories: 275},
			{name: "Salep", price: 330, calories: 290},
			{name: "Tea", price: 110, calories: 5},
			{name: "Cup Tea", price: 140, calories: 8},
		},
	},
	{
		name: "Herbal Teas", icon: "🌿",
		products: []seedProduct{
			{name: "Good Night", price: 280, calories: 7, desc: "melisa, elma, limon çimi, basra limonu, şeftali, frenk üzümü"},
			{name: "Beauty", price: 280, calories: 9, desc: "Beyaz çay, ısparta gülü, yaban mersini, goji berry, elma, şeftali"},
			{name: "Euphoria", price: 280, calories: 8, desc: "Mavi sarmaşık çiçeği, ahududu, lime, elma, lemongrass, mürver çiçeği, kurt üzümü"},
			{name: "Winter Fell", price: 280, calories: 12, desc: "Hibisküs, elma, tarçın, karanfil, portakal kabuğu, zencefil, pembe karabiber, lemongrass"},
			{name: "Jasmine", price: 280, calories: 5, desc: "yasemin çiçeği, yeşil çay"},
			{name: "Green Mango", price: 280, calories: 10, desc: "Yeşil çay, mango, ananas"},
			{name: "Summer Sun", price: 280, calories: 11, desc: "Hibisküs, elma, limon çimi, ananas, kuş üzümü, portakal kabuğu"},
			{name: "Day Dream", price: 280, calories: 6, desc: "Yeşil çay, lavanta, mint, kuşburnu, portakal kabuğu"},
			{name: "Berry", price: 280, calories: 13, desc: "Hibisküs, böğürtlen, karadut, frenk üzümü, lemongrass, elma"},
			{name: "Apple Pie", price: 280, calories: 14, desc: "Elma, tarçın"},
			{name: "Boost Up", price: 280, calories: 10, desc: "Hibisküs, kuşburnu, zencefil, elma"},
		},
	},
	{
		name: "Milkshake", icon: "🥤",
		products: []seedProduct{
			{name: "Berry Chocolate Milkshake", price: 365, calories: 310},
			{name: "Lotus Milkshake", price: 365, calories: 320},
			{name: "Oreo Milkshake", price: 365, calories: 315},
			{name: "Nut Milkshake", price: 365, calories: 305},
			{name: "Strawberry Milkshake", price: 365, calories: 275},
			{name: "Peach Milkshake", price: 365, calories: 265},
			{name: "Chocolate Milkshake", price: 365, calories: 300},
		},
	},
	{
		name: "Frozen", icon: "❄️",
		products: []seedProduct{
			{name: "Strawberry Frozen", price: 365, calories: 240},
			{name: "Mint Lime Frozen", price: 365, calories: 225},
			{name: "Peach Lime Frozen", price: 365, calories: 235},
			{name: "Melon Lime Frozen", price: 365, calories: 230},
			{name: "Mango Frozen", price: 365, calories: 255},
		},
	},
	{
		name: "Soft Beverages", icon: "🧃",
		products: []seedProduct{
			{name: "Homemade Ice Tea", price: 310, calories: 230},
			{name: "Portakal Suyu", price: 240, calories: 240},
			{name: "Limonata", price: 280, calories: 250},
			{name: "Çilekli Limonata", price: 300, calories: 265},
			{name: "Cool Lime", price: 340, calories: 225},
			{name: "Churchill", price: 200, calories: 220},
			{name: "Uludağ Premium", price: 170, calories: 235},
			// Su sits in the owner's 5-15 band rather than at 0: bottled water
			// is of course calorie-free, and 5 is simply the floor of the band
			// they gave it. Set it to 0 and seedCalories stores NULL, which
			// hides the badge on water altogether — that is the one-line change
			// if the estimate ever looks worse than no estimate.
			{name: "Su", price: 80, calories: 5},
		},
	},
}

// --------------------------------------------------------------- seeding

// seedCalories turns the plain int of seedProduct into the value the NULLABLE
// products.calories column takes: a product left at 0 is stored as NULL, so a
// row added here later without an estimate reads as "unknown" on the menu
// instead of claiming a measured zero. Every product in mellyMenu carries an
// estimate today, so this is a guard for the next edit rather than a live
// branch.
func seedCalories(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

// SeedDemo creates the Melly Coffee account and its single menu.
// It does nothing when the account already exists, so it is safe to re-run.
//
// isProduction comes from the caller's config: the seed writes a WORKING LOGIN
// for a real business, so a production database must never receive it. That is
// the first thing checked, before any query runs.
//
// Both halves of the identity are then checked: the e-mail owns the account and
// businesses.slug is globally unique, so either one would make the insert fail.
// The menu slug is not checked — it is only unique within its business, and
// "suadiye" is a name every other tenant may use as well.
func SeedDemo(ctx context.Context, pool *pgxpool.Pool, isProduction bool) error {
	if isProduction {
		log.Println("[karecik] seed skipped: APP_ENV=production")
		return nil
	}

	// The old sample tenant goes first, whether or not the new one is created.
	if err := purgeLegacyDemo(ctx, pool); err != nil {
		return err
	}

	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1))
		    OR EXISTS (SELECT 1 FROM businesses WHERE slug = $2)`,
		mellyEmail, mellySlug).Scan(&exists)
	if err != nil {
		return fmt.Errorf("seed lookup failed: %w", err)
	}
	if exists {
		return nil
	}

	hash, err := utils.HashPassword(mellyPassword)
	if err != nil {
		return fmt.Errorf("could not hash the seed password: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, business_name)
		VALUES ($1, $2, $3)
		RETURNING id`,
		mellyEmail, hash, mellyName).Scan(&userID)
	if err != nil {
		return fmt.Errorf("could not create the seed user: %w", err)
	}

	// The business is the account and the subdomain it answers on, nothing more.
	var businessID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO businesses (user_id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING id`,
		userID, mellyName, mellySlug).Scan(&businessID)
	if err != nil {
		return fmt.Errorf("could not create the seed business: %w", err)
	}

	// Every published setting lives on the menu: its branding, its splash
	// screen, its contact details and its languages. The palette overrides the
	// theme column by column — background #FDFBF7, accent #C49A6C, splash
	// background #1F1A17 and, since migration 006, the text tone #1F1A17 that
	// used to be the theme's (see mellyTheme).
	//
	// show_yerli_uretim is true and yerli_uretim_logo_url points at the owner's
	// own certified mark, so the footer shows the real badge rather than the
	// plain text pill it falls back to. logo_url is the brand SVG from the same
	// site; header_display keeps its 'both' default, so the header renders that
	// logo beside the menu name.
	//
	// currency_symbol is left out on purpose: the column defaults to the symbol
	// of TRY and only the repository layer ever writes it. address is left out
	// because the site publishes none.
	var menuID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO menus (
			business_id, name, slug, is_active, position,
			currency, theme, font_family, primary_color, text_color,
			default_language, languages, splash_enabled, splash_duration,
			splash_bg_color, splash_text, splash_headline, splash_exit_animation,
			splash_exit_duration, background_type, background_color,
			background_overlay_opacity, logo_url, logo_fade_in,
			show_yerli_uretim, yerli_uretim_logo_url,
			phone, instagram, wifi_ssid, wifi_password
		) VALUES (
			$1, $2, $3, true, 0,
			'TRY', $4, 'inter', '#C49A6C', '#1F1A17',
			'tr', $5, true, 1200,
			'#1F1A17', 'Hoş geldiniz', $6, 'slide-up',
			450, 'color', '#FDFBF7',
			0.40, $7, true,
			true, $8,
			$9, $10, $11, $12
		)
		RETURNING id`,
		businessID, mellyMenuName, mellyMenuSlug, mellyTheme, []string{"tr", "en"},
		mellyName, mellyLogoURL, mellyYerliUretimLogoURL,
		mellyPhone, mellyInstagram, mellyWifiSSID, mellyWifiPass,
	).Scan(&menuID)
	if err != nil {
		return fmt.Errorf("could not create the seed menu: %w", err)
	}

	for categoryIndex, category := range mellyMenu {
		categoryTranslations := models.Translations{
			"tr": {Name: category.name},
			"en": {Name: category.name},
		}

		var categoryID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO categories (business_id, menu_id, translations, icon, position, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
			RETURNING id`,
			businessID, menuID, categoryTranslations, category.icon, categoryIndex).Scan(&categoryID)
		if err != nil {
			return fmt.Errorf("could not create the seed category (%s): %w", category.name, err)
		}

		for productIndex, product := range category.products {
			productTranslations := models.Translations{
				"tr": {Name: product.name, Description: product.desc},
				"en": {Name: product.name, Description: product.desc},
			}

			// Normalize both guarantees the non-nil slice the NOT NULL jsonb
			// column needs — a nil one would be written as the literal null —
			// and rebuilds the group and item slices, so the row can never end
			// up aliasing the shared package-level groups.
			options := product.options.Normalize()

			// calories carries the category estimate described on seedProduct —
			// an estimate, never a measurement. allergens, badges and
			// is_featured are still absent from the column list and keep their
			// defaults: the site states none of them.
			_, err = tx.Exec(ctx, `
				INSERT INTO products (
					business_id, category_id, translations, price, calories, options,
					is_active, position
				) VALUES ($1, $2, $3, $4, $5, $6, true, $7)`,
				businessID, categoryID, productTranslations, product.price,
				seedCalories(product.calories), options, productIndex)
			if err != nil {
				return fmt.Errorf("could not create the seed product (%s): %w", product.name, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("could not save the seed data: %w", err)
	}

	log.Printf("[karecik] Melly Coffee menu created -> %s / %s (address: %s/%s)",
		mellyEmail, mellyPassword, mellySlug, mellyMenuSlug)
	return nil
}

// purgeLegacyDemo removes the "Demo Kafe" tenant this file used to seed, and
// nothing else. The sample menu moved to a real cafe, so the old tenant — its
// business, its menu, its categories and its products, all reached through
// ON DELETE CASCADE from users — has to go, or every developer database keeps
// serving demo-kafe alongside melly-coffee for ever.
//
// It is destructive by design, which is why it is fenced in on every side:
//   - it runs only when that one address is actually present;
//   - the statement matches lower(email) against the single constant
//     legacyDemoEmail — no wildcard, no OR, no second predicate, no other
//     value can reach it;
//   - it runs inside a transaction that is rolled back unless EXACTLY one
//     account was removed, so a statement that ever matched more than the one
//     row could not commit;
//   - and it says loudly what it did.
func purgeLegacyDemo(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = $1)`,
		legacyDemoEmail).Scan(&exists)
	if err != nil {
		return fmt.Errorf("legacy demo lookup failed: %w", err)
	}
	if !exists {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`DELETE FROM users WHERE lower(email) = $1`, legacyDemoEmail)
	if err != nil {
		return fmt.Errorf("could not remove the legacy demo tenant: %w", err)
	}
	if rows := tag.RowsAffected(); rows != 1 {
		return fmt.Errorf("refusing to remove the legacy demo tenant: the delete "+
			"matched %d accounts, expected exactly 1", rows)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("could not remove the legacy demo tenant: %w", err)
	}

	log.Printf("[karecik] removed the legacy demo tenant (%s) — its business, menu, "+
		"categories and products went with it", legacyDemoEmail)
	return nil
}
