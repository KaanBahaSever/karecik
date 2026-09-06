package database

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"karecik/backend/internal/models"
	"karecik/backend/internal/utils"
)

// The seed tenant is Melly Coffee, a real cafe whose owner asked for its own
// menus to be the sample ones. It runs two locations and publishes one menu
// each: the Suadiye menu comes from https://mellycoffee.co/menu and the
// Cihangir menu from https://mellycoffee.co/cihangir-menu. Every category,
// product, price and description below is reproduced verbatim from those two
// pages: nothing here is invented, rounded or embellished, and no product the
// site does not sell is added.
//
// The two menus are deliberately NOT folded into one another. They are the same
// business but different locations, and Cihangir is cheaper across the board
// (Espresso 180 against 210, Cafe Latte 225 against 295, Türk Kahvesi 170
// against 200) with its own add-on prices as well. That is exactly why products
// and prices hang off a menu rather than off the business.
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

	// A menu slug is the PATH segment under the subdomain, so the addresses are
	// melly-coffee.karecik.com/suadiye and melly-coffee.karecik.com/cihangir.
	// The tenant now has TWO of them, which is why the subdomain root lists both
	// instead of redirecting straight into the only one.
	mellyMenuName = "Suadiye"
	mellyMenuSlug = "suadiye"

	// Cihangir is the second location: a second MENU on the same business, not
	// a second tenant. One account, one subdomain, two menus, each with its own
	// products, its own prices and its own add-on prices.
	cihangirMenuName = "Cihangir"
	cihangirMenuSlug = "cihangir"

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

// SUADIYE's groups. They keep their original mellyXxx names; Cihangir's own set
// lives below under cihangirXxx and the two are never mixed, because the two
// locations charge different money for the same add-on.
//
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

// mellySharedOptions returns one SUADIYE product's own copy of the milk choice
// and the syrup/shot add-ons.
func mellySharedOptions() models.ProductOptions {
	return models.ProductOptions{
		cloneOptionGroup(mellyMilkChoice),
		cloneOptionGroup(mellyExtraShots),
	}
}

// CIHANGIR's groups. They are a SEPARATE set on purpose, not a reuse of the
// Suadiye ones: the Cihangir page charges its own money for the same add-ons.
//
//	syrups              Cihangir +30   Suadiye +40
//	extra espresso shot Cihangir +40   Suadiye +60
//	Decaf               Cihangir +50   Suadiye does not offer it
//	Double portion      Cihangir +10 (Espresso, Cortado) and +20 (Türk Kahvesi)
//	                    Suadiye +30 (Türk Kahvesi only)
//
// Only the milk choice happens to match at +60, and even that is left standing
// on its own rather than pointed at mellyMilkChoice: the day one page changes
// its milk surcharge, the other must not move with it.
//
// The Cihangir page also splits the note Suadiye prints as one line into two
// groups, "Ekstra Şuruplar" and "Ekstralar", and lists three syrups Suadiye
// does not have, so the shapes differ as well as the prices.
var (
	cihangirMilkChoice = models.ProductOptionGroup{
		Name: "Süt Tercihi", Type: models.OptionTypeSingle, Required: false,
		Items: []models.ProductOptionItem{
			{Name: "Yulaf Sütü", Price: 60},
			{Name: "Badem Sütü", Price: 60},
		},
	}

	cihangirSyrups = models.ProductOptionGroup{
		Name: "Ekstra Şuruplar", Type: models.OptionTypeMultiple, Required: false,
		Items: []models.ProductOptionItem{
			{Name: "Vanilya", Price: 30},
			{Name: "Karamel", Price: 30},
			{Name: "Fındık", Price: 30},
			{Name: "Toffee Nut", Price: 30},
			{Name: "Kurabiye", Price: 30},
			{Name: "Nane", Price: 30},
			{Name: "Balkabağı Baharatı", Price: 30},
		},
	}

	cihangirExtras = models.ProductOptionGroup{
		Name: "Ekstralar", Type: models.OptionTypeMultiple, Required: false,
		Items: []models.ProductOptionItem{
			{Name: "Ekstra Espresso Shot", Price: 40},
			{Name: "Decaf", Price: 50},
		},
	}

	// The page's own footnote is "Double (Espresso, Cortado +10₺ / Türk Kahvesi
	// +20₺)", so the portion question is asked with two different surcharges.
	// Espresso and Cortado may be left as they come, which is why theirs is not
	// required; Türk Kahvesi has to be ordered as one size or the other, so its
	// group is — exactly as the Suadiye menu marks its own.
	cihangirPortionDouble10 = models.ProductOptionGroup{
		Name: "Porsiyon", Type: models.OptionTypeSingle, Required: false,
		Items: []models.ProductOptionItem{
			{Name: "Tek", Price: 0},
			{Name: "Double", Price: 10},
		},
	}

	cihangirPortionDouble20 = models.ProductOptionGroup{
		Name: "Porsiyon", Type: models.OptionTypeSingle, Required: true,
		Items: []models.ProductOptionItem{
			{Name: "Tek", Price: 0},
			{Name: "Double", Price: 20},
		},
	}
)

// cihangirSharedOptions returns one CIHANGIR product's own copy of the three
// add-on groups the page prints under Espresso Bar and again under Cold
// Beverages. Every drink in both categories carries them except Türk Kahvesi,
// which the page gives a portion choice instead.
func cihangirSharedOptions() models.ProductOptions {
	return models.ProductOptions{
		cloneOptionGroup(cihangirMilkChoice),
		cloneOptionGroup(cihangirSyrups),
		cloneOptionGroup(cihangirExtras),
	}
}

// cihangirPortionOptions puts the portion question first — it is asked about
// the drink itself, not about what goes into it — and then the three add-on
// groups. The result is a fresh slice holding fresh groups every call, so no
// two products ever share a backing array with each other or with the
// package-level vars above.
func cihangirPortionOptions(portion models.ProductOptionGroup) models.ProductOptions {
	shared := cihangirSharedOptions()
	options := make(models.ProductOptions, 0, 1+len(shared))
	options = append(options, cloneOptionGroup(portion))
	return append(options, shared...)
}

// --------------------------------------------------------------- menu data

// The menu content is real customer-facing copy, so it stays in Turkish.
//
// name carries BOTH languages: the site prints a single name per product, and
// a blank English name would leave the language switcher showing an empty
// card, so the same string is stored under tr and en. desc is treated the same
// way — the site's ingredient lists exist only in Turkish — unless descEN is
// filled in, which is the one place the two pages really do print two
// languages.
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
	name string
	desc string
	// descEN overrides the English description. It is empty for all but one
	// product, and an empty value means "the same string as desc".
	descEN   string
	price    float64
	calories int
	options  models.ProductOptions
}

type seedCategory struct {
	name     string
	icon     string
	products []seedProduct
}

// seedMenu is one whole menu: the identity it is published under and the
// categories that hang off it, in the order the page lists them. The look, the
// languages and the contact details are NOT here — both menus share those and
// insertSeedMenu writes them.
type seedMenu struct {
	name       string
	slug       string
	categories []seedCategory
}

// ---------------------------------------------------------- Suadiye, the
// first location — https://mellycoffee.co/menu
//
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
var suadiyeMenu = []seedCategory{
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

// ---------------------------------------------------------- Cihangir, the
// second location — https://mellycoffee.co/cihangir-menu
//
// Nine categories in the page's own order, at the page's own prices. Cihangir
// is cheaper than Suadiye on almost every line they share; that is not a
// mistake to be tidied up, it is the whole point of a per-menu price.
//
// Spelling corrections applied to this page's own text, at the owner's request.
// They are listed here so they are easy to find and easy to revert:
//
//  1. "Espresso Machiato" -> "Espresso Macchiato"  (Espresso Bar)
//  2. "Latte Machiato"    -> "Latte Macchiato"     (Espresso Bar)
//  3. "Cappucino"         -> "Cappuccino"          (Espresso Bar)
//  4. "Ice Cappucino"     -> "Ice Cappuccino"      (Cold Beverages)
//  5. "Affagato"          -> "Affogato"            (Cold Beverages)
//  6. "Ciabbatta Sandviç" -> "Ciabatta Sandviç"    (Kitchen Bar)
//  7. "Lütfen barıştamızdan bilgi alınız"
//     -> "Fiyat bilgisi için baristamıza danışabilirsiniz."  (Coffee Pack)
//
// (7) turns a fragment into a sentence and fixes "barıştamızdan", which is a
// clear typo for "baristamızdan". (6) is the same Ciabbatta -> Ciabatta fix the
// Suadiye menu already carries, so the two stay consistent; "Churchill" is
// already capitalised on this page, so Suadiye's churchill -> Churchill fix has
// no counterpart here.
//
// Nothing else was touched. Every price, every product and every category name
// is exactly what the page prints, in the page's own order.
var cihangirMenu = []seedCategory{
	{
		name: "Desserts", icon: "🍰",
		products: []seedProduct{
			{name: "Karpatka", price: 340, calories: 395},
			{name: "Süt Reçelli Cheesecake", price: 340, calories: 470},
			{name: "Çilekli Crumble", price: 340, calories: 410},
			{name: "Banana Bread", price: 250, calories: 380},
			{name: "Chocolate Cookie", price: 200, calories: 425},
			{name: "Yulaflı Muzlu Kurabiye", price: 190, calories: 355},
			{name: "Raw", price: 190, calories: 345},
			{name: "Snickers", price: 190, calories: 430},
			{name: "Bounty", price: 190, calories: 415},
			{name: "Mixed Ball", price: 190, calories: 365},
			{name: "Carrot Cake", price: 310, calories: 440},
			{name: "Hazelnut Cream Cheese", price: 330, calories: 465},
			{name: "Klaad Kaka", price: 290, calories: 455},
			{name: "Banana Nuts", price: 280, calories: 400},
			{name: "Vegan Cake", price: 240, calories: 350},
			{name: "Brownie", price: 350, calories: 490},
		},
	},
	{
		// All three add-on groups apply to every row except Türk Kahvesi, which
		// takes the portion question instead. Espresso and Cortado take BOTH:
		// the page's footnote gives them a +10 Double on top of the add-ons.
		// Unlike Suadiye, this page does not sell a separate Double Espresso
		// product, so here the portion group really is how a double is ordered.
		name: "Espresso Bar", icon: "☕",
		products: []seedProduct{
			{name: "Espresso", price: 180, calories: 5,
				options: cihangirPortionOptions(cihangirPortionDouble10)},
			{name: "Espresso Macchiato", price: 190, calories: 115, options: cihangirSharedOptions()},
			{name: "Cortado", price: 190, calories: 130,
				options: cihangirPortionOptions(cihangirPortionDouble10)},
			{name: "Americano", price: 215, calories: 8, options: cihangirSharedOptions()},
			{name: "Cafe Latte", price: 225, calories: 175, options: cihangirSharedOptions()},
			{name: "Latte Macchiato", price: 225, calories: 160, options: cihangirSharedOptions()},
			{name: "Cappuccino", price: 225, calories: 150, options: cihangirSharedOptions()},
			{name: "Flat White", price: 225, calories: 165, options: cihangirSharedOptions()},
			{name: "Red Eye", price: 250, calories: 120, options: cihangirSharedOptions()},
			{name: "Black Eye", price: 260, calories: 130, options: cihangirSharedOptions()},
			{name: "Mocha", price: 255, calories: 285, options: cihangirSharedOptions()},
			{name: "White Mocha", price: 255, calories: 300, options: cihangirSharedOptions()},
			{name: "Zebra Mocha", price: 260, calories: 295, options: cihangirSharedOptions()},
			// The page gives Türk Kahvesi a portion choice and no add-ons at
			// all — no milk, no syrup, no second shot goes into it.
			{name: "Türk Kahvesi", price: 170, calories: 12,
				options: models.ProductOptions{cloneOptionGroup(cihangirPortionDouble20)}},
		},
	},
	{
		name: "Brew Bar", icon: "🫖",
		products: []seedProduct{
			{name: "Filter Coffee", price: 195, calories: 6},
			{name: "Small Filter Coffee", price: 150, calories: 5},
			{name: "Ice Filter Coffee", price: 210, calories: 7},
			{name: "Cold Brew", price: 245, calories: 9},
			{name: "Hario", price: 290, calories: 10},
		},
	},
	{
		// The page repeats the add-on note here, so all three groups apply to
		// every row.
		name: "Cold Beverages", icon: "🧊",
		products: []seedProduct{
			{name: "Ice Latte", price: 240, calories: 170, options: cihangirSharedOptions()},
			{name: "Ice Americano", price: 220, calories: 115, options: cihangirSharedOptions()},
			{name: "Ice Cappuccino", price: 240, calories: 145, options: cihangirSharedOptions()},
			{name: "Ice Mocha", price: 270, calories: 290, options: cihangirSharedOptions()},
			{name: "Ice White Mocha", price: 270, calories: 305, options: cihangirSharedOptions()},
			{name: "Ice Flat White", price: 245, calories: 160, options: cihangirSharedOptions()},
			{name: "Ice Zebra Mocha", price: 275, calories: 300, options: cihangirSharedOptions()},
			{name: "Ice Red Eye", price: 260, calories: 125, options: cihangirSharedOptions()},
			{name: "Ice Black Eye", price: 270, calories: 135, options: cihangirSharedOptions()},
			{name: "Ice Matcha Latte", price: 270, calories: 235, options: cihangirSharedOptions()},
			{name: "Ice Chai Latte", price: 260, calories: 245, options: cihangirSharedOptions()},
			{name: "Affogato", price: 280, calories: 265, options: cihangirSharedOptions()},
			{name: "Espresso Tonic", price: 270, calories: 230, options: cihangirSharedOptions()},
			{name: "Bumble", price: 280, calories: 250, options: cihangirSharedOptions()},
			{name: "Coffee Shake", price: 250, calories: 310, options: cihangirSharedOptions()},
			{name: "Milkshake Frozen", price: 270, calories: 315, options: cihangirSharedOptions()},
		},
	},
	{
		name: "Hot Beverages", icon: "🔥",
		products: []seedProduct{
			{name: "Chai Latte", price: 250, calories: 250},
			{name: "Dirty Chai Tea Latte", price: 310, calories: 275},
			{name: "Hot Chocolate", price: 250, calories: 310},
			{name: "Salep", price: 250, calories: 290},
			{name: "Matcha Latte", price: 260, calories: 240},
		},
	},
	{
		name: "Soft Beverages", icon: "🧃",
		products: []seedProduct{
			{name: "Cool Lime", price: 220, calories: 225},
			{name: "Homemade Ice Tea", price: 220, calories: 230},
			// Soda and Su are the two rows that share a number, and they share
			// it honestly: both are calorie-free bottled water, sparkling and
			// still, so both sit at 5, the floor of the owner's 5-15 band. See
			// the same note on Suadiye's Su for the one-line change that hides
			// the badge on water altogether.
			{name: "Soda", price: 100, calories: 5},
			{name: "Churchill", price: 150, calories: 220},
			{name: "Su", price: 50, calories: 5},
			{name: "Portakal Suyu", price: 200, calories: 240},
			{name: "Nar Suyu", price: 200, calories: 250},
		},
	},
	{
		// The Cihangir page lists the blends by name only. They are the same
		// blends Suadiye sells, so the ingredient lists and the estimates are
		// the Suadiye seed's, verbatim; only the price differs (240 here, 280
		// there).
		name: "Herbal Teas", icon: "🌿",
		products: []seedProduct{
			{name: "Good Night", price: 240, calories: 7, desc: "melisa, elma, limon çimi, basra limonu, şeftali, frenk üzümü"},
			{name: "Beauty", price: 240, calories: 9, desc: "Beyaz çay, ısparta gülü, yaban mersini, goji berry, elma, şeftali"},
			{name: "Euphoria", price: 240, calories: 8, desc: "Mavi sarmaşık çiçeği, ahududu, lime, elma, lemongrass, mürver çiçeği, kurt üzümü"},
			{name: "Winter Fell", price: 240, calories: 12, desc: "Hibisküs, elma, tarçın, karanfil, portakal kabuğu, zencefil, pembe karabiber, lemongrass"},
			{name: "Jasmine", price: 240, calories: 5, desc: "yasemin çiçeği, yeşil çay"},
			{name: "Green Mango", price: 240, calories: 10, desc: "Yeşil çay, mango, ananas"},
			{name: "Summer Sun", price: 240, calories: 11, desc: "Hibisküs, elma, limon çimi, ananas, kuş üzümü, portakal kabuğu"},
			{name: "Day Dream", price: 240, calories: 6, desc: "Yeşil çay, lavanta, mint, kuşburnu, portakal kabuğu"},
			{name: "Berry", price: 240, calories: 13, desc: "Hibisküs, böğürtlen, karadut, frenk üzümü, lemongrass, elma"},
			{name: "Apple Pie", price: 240, calories: 14, desc: "Elma, tarçın"},
			{name: "Boost Up", price: 240, calories: 10, desc: "Hibisküs, kuşburnu, zencefil, elma"},
		},
	},
	{
		// The one product on either page whose description is printed in two
		// languages. The Turkish is the Suadiye seed's wording for the same
		// sandwich; the English is that ingredient list rendered faithfully,
		// which is why descEN exists at all.
		name: "Kitchen Bar", icon: "🥪",
		products: []seedProduct{
			{name: "Ciabatta Sandviç", price: 250, calories: 465,
				desc:   "Ekşi mayalı ekmek, pesto sos, krem peynir, kaşar, hindi füme, roka",
				descEN: "Sourdough bread, pesto sauce, cream cheese, kashar cheese, smoked turkey, arugula"},
		},
	},
	{
		// Beans sold by the bag, priced at the counter. price 0 is what the
		// page says — it prints no number — and calories 0 makes seedCalories
		// store NULL, which is the honest answer for something that has no
		// serving: the menu shows no calorie badge instead of claiming a zero.
		name: "Coffee Pack", icon: "🫘",
		products: []seedProduct{
			{name: "Çekirdek Kahve", price: 0, calories: 0,
				desc: "Fiyat bilgisi için baristamıza danışabilirsiniz."},
		},
	},
}

// mellyMenus is every menu of the tenant, in the order the directory page and
// the menu switcher list them: menus.position is the index below.
var mellyMenus = []seedMenu{
	{name: mellyMenuName, slug: mellyMenuSlug, categories: suadiyeMenu},
	{name: cihangirMenuName, slug: cihangirMenuSlug, categories: cihangirMenu},
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

// SeedDemo creates the Melly Coffee account and every menu it publishes.
//
// isProduction comes from the caller's config: the seed writes a WORKING LOGIN
// for a real business, so a production database must never receive it. That is
// the first thing checked, before any query runs.
//
// It is safe to re-run, and it TOPS UP rather than starting over. Both halves of
// the tenant identity are checked — the e-mail owns the account and
// businesses.slug is globally unique — and then:
//
//   - neither present: the account, the business and every menu are created;
//   - both present: only the menus this business does not carry yet are added,
//     matched on menus.slug within the business;
//   - exactly one present: nothing happens at all.
//
// The middle case is the one that matters now the tenant has two menus. Every
// developer database seeded before Cihangir existed already holds the account
// and the Suadiye menu, so the old "the account exists, do nothing" check would
// have left all of them one menu short for ever, short of dropping the database.
// Topping up hands them Cihangir on the next boot and still never writes a menu
// twice, because the slug — unique per business — is what is matched. A menu
// that is already there is left completely alone, including whatever the owner
// has since changed about its products.
//
// The lopsided third case is skipped on purpose: a database holding the e-mail
// but not the business, or the reverse, is a half-state this function did not
// create and must not repair blindly.
//
// Everything runs in ONE transaction, so a failure part-way through leaves the
// database exactly as it was.
func SeedDemo(ctx context.Context, pool *pgxpool.Pool, isProduction bool) error {
	if isProduction {
		log.Println("[karecik] seed skipped: APP_ENV=production")
		return nil
	}

	// The old sample tenant goes first, whether or not the new one is created.
	if err := purgeLegacyDemo(ctx, pool); err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var accountExists, businessExists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1)),
		       EXISTS (SELECT 1 FROM businesses WHERE slug = $2)`,
		mellyEmail, mellySlug).Scan(&accountExists, &businessExists)
	if err != nil {
		return fmt.Errorf("seed lookup failed: %w", err)
	}
	if accountExists != businessExists {
		return nil
	}

	var businessID uuid.UUID
	if businessExists {
		err = tx.QueryRow(ctx,
			`SELECT id FROM businesses WHERE slug = $1`, mellySlug).Scan(&businessID)
		if err != nil {
			return fmt.Errorf("seed lookup failed: %w", err)
		}
	} else {
		businessID, err = insertSeedTenant(ctx, tx)
		if err != nil {
			return err
		}
	}

	created, err := insertMissingMenus(ctx, tx, businessID)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("could not save the seed data: %w", err)
	}

	if len(created) == 0 {
		return nil
	}
	log.Printf("[karecik] Melly Coffee ready -> %s / %s", mellyEmail, mellyPassword)
	for _, slug := range created {
		log.Printf("[karecik] menu created -> address: %s/%s", mellySlug, slug)
	}
	return nil
}

// insertSeedTenant creates the account and the business, and nothing else: the
// business is the login and the subdomain it answers on. The menus that hang
// off it are inserted separately, which is what lets a tenant that already
// exists be topped up with a menu it does not have yet.
func insertSeedTenant(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
	hash, err := utils.HashPassword(mellyPassword)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not hash the seed password: %w", err)
	}

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, business_name)
		VALUES ($1, $2, $3)
		RETURNING id`,
		mellyEmail, hash, mellyName).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not create the seed user: %w", err)
	}

	var businessID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO businesses (user_id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING id`,
		userID, mellyName, mellySlug).Scan(&businessID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not create the seed business: %w", err)
	}

	return businessID, nil
}

// insertMissingMenus writes every menu of mellyMenus the business does not
// already carry, and returns the slugs it created in mellyMenus order.
//
// The index in mellyMenus becomes menus.position, so a menu keeps the same
// place in the directory whether it arrived on a fresh database or was added to
// one that already had the other.
func insertMissingMenus(ctx context.Context, tx pgx.Tx, businessID uuid.UUID) ([]string, error) {
	created := make([]string, 0, len(mellyMenus))

	for position, menu := range mellyMenus {
		var exists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM menus WHERE business_id = $1 AND slug = $2)`,
			businessID, menu.slug).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("seed menu lookup failed (%s): %w", menu.slug, err)
		}
		if exists {
			continue
		}

		if err := insertSeedMenu(ctx, tx, businessID, menu, position); err != nil {
			return nil, err
		}
		created = append(created, menu.slug)
	}

	return created, nil
}

// insertSeedMenu writes one whole menu: the menu row, then its categories in
// the page's own order, then each category's products in the page's own order.
//
// Every published setting lives on the menu — its branding, its splash screen,
// its contact details and its languages — and both menus are given the same
// look, because they are one brand in two rooms. The palette overrides the
// theme column by column: background #FDFBF7, accent #C49A6C, splash background
// #1F1A17 and, since migration 006, the text tone #1F1A17 that used to be the
// theme's (see mellyTheme).
//
// show_yerli_uretim is true and yerli_uretim_logo_url points at the owner's own
// certified mark, so the footer shows the real badge rather than the plain text
// pill it falls back to. logo_url is the brand SVG from the same site;
// header_display keeps its 'both' default, so the header renders that logo
// beside the menu name.
//
// The contact columns belong to the tenant rather than to the location: neither
// page publishes a phone number, an Instagram handle or a Wi-Fi password per
// branch, so both menus start from the same values instead of a second set
// being invented, and the owner edits them per menu in Menü Ayarları.
//
// currency_symbol is left out on purpose: the column defaults to the symbol of
// TRY and only the repository layer ever writes it. address is left out because
// neither page publishes one, and slogan because the owner types their own in
// Menü Ayarları — it keeps the empty-string default of migration 007, so the
// header prints no tagline until they do.
func insertSeedMenu(ctx context.Context, tx pgx.Tx, businessID uuid.UUID, menu seedMenu, position int) error {
	var menuID uuid.UUID
	err := tx.QueryRow(ctx, `
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
			$1, $2, $3, true, $4,
			'TRY', $5, 'inter', '#C49A6C', '#1F1A17',
			'tr', $6, true, 1200,
			'#1F1A17', 'Hoş geldiniz', $7, 'slide-up',
			450, 'color', '#FDFBF7',
			0.40, $8, true,
			true, $9,
			$10, $11, $12, $13
		)
		RETURNING id`,
		businessID, menu.name, menu.slug, position, mellyTheme, []string{"tr", "en"},
		mellyName, mellyLogoURL, mellyYerliUretimLogoURL,
		mellyPhone, mellyInstagram, mellyWifiSSID, mellyWifiPass,
	).Scan(&menuID)
	if err != nil {
		return fmt.Errorf("could not create the seed menu (%s): %w", menu.slug, err)
	}

	for categoryIndex, category := range menu.categories {
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
			return fmt.Errorf("could not create the seed category (%s / %s): %w",
				menu.slug, category.name, err)
		}

		for productIndex, product := range category.products {
			// Almost every product prints one string for both languages, so the
			// English falls back to the Turkish rather than to a blank card.
			descriptionEN := product.descEN
			if descriptionEN == "" {
				descriptionEN = product.desc
			}

			productTranslations := models.Translations{
				"tr": {Name: product.name, Description: product.desc},
				"en": {Name: product.name, Description: descriptionEN},
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
				return fmt.Errorf("could not create the seed product (%s / %s): %w",
					menu.slug, product.name, err)
			}
		}
	}

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
