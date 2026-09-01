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

const (
	demoSlug     = "demo-kafe"
	demoEmail    = "demo@karecik.com"
	demoPassword = "demo1234"
)

// The menu content below is real customer-facing copy, so it stays in Turkish.
type seedProduct struct {
	nameTR      string
	nameEN      string
	descTR      string
	descEN      string
	ingredients string
	price       float64
	allergens   []string
	featured    bool
}

type seedCategory struct {
	nameTR   string
	nameEN   string
	icon     string
	products []seedProduct
}

// demoMenu — the sample cafe menu shown in the iPhone preview on the landing page.
var demoMenu = []seedCategory{
	{
		nameTR: "Sıcak İçecekler", nameEN: "Hot Drinks", icon: "☕",
		products: []seedProduct{
			{nameTR: "Türk Kahvesi", nameEN: "Turkish Coffee", descTR: "Geleneksel közde pişirilmiş, lokum ikramıyla", descEN: "Traditional, served with Turkish delight", ingredients: "Türk kahvesi, su", price: 85, allergens: []string{"kafein"}, featured: true},
			{nameTR: "Espresso", nameEN: "Espresso", descTR: "Tek shot, yoğun aroma", descEN: "Single shot, intense aroma", ingredients: "Espresso", price: 75, allergens: []string{"kafein"}},
			{nameTR: "Latte", nameEN: "Caffè Latte", descTR: "Espresso ve buharda ısıtılmış süt", descEN: "Espresso with steamed milk", ingredients: "Espresso, süt", price: 145, allergens: []string{"sut", "kafein"}, featured: true},
			{nameTR: "Cappuccino", nameEN: "Cappuccino", descTR: "Bol köpüklü, tarçın seçeneğiyle", descEN: "Extra foam, cinnamon optional", ingredients: "Espresso, süt", price: 140, allergens: []string{"sut", "kafein"}},
			{nameTR: "Sahlep", nameEN: "Salep", descTR: "Tarçınlı, kış klasiği", descEN: "With cinnamon, a winter classic", ingredients: "Süt, salep, tarçın", price: 130, allergens: []string{"sut"}},
			{nameTR: "Bitki Çayı", nameEN: "Herbal Tea", descTR: "Ihlamur, nane-limon veya kuşburnu", descEN: "Linden, mint-lemon or rosehip", price: 70, allergens: []string{}},
		},
	},
	{
		nameTR: "Soğuk İçecekler", nameEN: "Cold Drinks", icon: "🥤",
		products: []seedProduct{
			{nameTR: "Ice Latte", nameEN: "Iced Latte", descTR: "Buz üzerine espresso ve soğuk süt", descEN: "Espresso over ice with cold milk", ingredients: "Espresso, süt, buz", price: 155, allergens: []string{"sut", "kafein"}, featured: true},
			{nameTR: "Limonata", nameEN: "Lemonade", descTR: "Taze sıkılmış, naneli", descEN: "Freshly squeezed, with mint", ingredients: "Limon, su, şeker, nane", price: 110, allergens: []string{}},
			{nameTR: "Ev Yapımı Buzlu Çay", nameEN: "Homemade Iced Tea", descTR: "Şeftali veya orman meyveli", descEN: "Peach or berry", price: 105, allergens: []string{}},
			{nameTR: "Milkshake", nameEN: "Milkshake", descTR: "Çilek, muz veya çikolata", descEN: "Strawberry, banana or chocolate", ingredients: "Süt, dondurma", price: 165, allergens: []string{"sut"}},
			{nameTR: "Maden Suyu", nameEN: "Sparkling Water", descTR: "Sade veya meyveli", descEN: "Plain or flavoured", price: 45, allergens: []string{}},
		},
	},
	{
		nameTR: "Yiyecekler", nameEN: "Food", icon: "🍽️",
		products: []seedProduct{
			{nameTR: "Serpme Kahvaltı", nameEN: "Turkish Breakfast", descTR: "İki kişilik, sınırsız çay ile", descEN: "For two, with unlimited tea", ingredients: "Peynir çeşitleri, zeytin, bal, kaymak, yumurta", price: 890, allergens: []string{"sut", "yumurta", "gluten"}, featured: true},
			{nameTR: "Kaşarlı Tost", nameEN: "Cheese Toastie", descTR: "Köy ekmeğinde, turşu ile", descEN: "On sourdough, served with pickles", ingredients: "Ekmek, kaşar peyniri, tereyağı", price: 175, allergens: []string{"gluten", "sut"}},
			{nameTR: "Menemen", nameEN: "Menemen", descTR: "Domates, biber ve yumurta", descEN: "Tomato, pepper and eggs", ingredients: "Yumurta, domates, biber, tereyağı", price: 245, allergens: []string{"yumurta", "sut"}},
			{nameTR: "Tavuklu Sezar Salata", nameEN: "Chicken Caesar Salad", descTR: "Izgara tavuk, parmesan, kruton", descEN: "Grilled chicken, parmesan, croutons", ingredients: "Marul, tavuk, parmesan, kruton, sezar sos", price: 320, allergens: []string{"gluten", "sut", "yumurta"}},
			{nameTR: "Kulüp Sandviç", nameEN: "Club Sandwich", descTR: "Patates kızartması eşliğinde", descEN: "Served with french fries", ingredients: "Tost ekmeği, tavuk, marul, domates", price: 295, allergens: []string{"gluten", "yumurta"}},
		},
	},
	{
		nameTR: "Tatlılar", nameEN: "Desserts", icon: "🍰",
		products: []seedProduct{
			{nameTR: "San Sebastian Cheesecake", nameEN: "San Sebastian Cheesecake", descTR: "Günlük yapım, dilim", descEN: "Made daily, per slice", ingredients: "Labne, krema, yumurta, şeker", price: 210, allergens: []string{"sut", "yumurta", "gluten"}, featured: true},
			{nameTR: "Magnolia", nameEN: "Magnolia", descTR: "Muzlu, bisküvili", descEN: "Banana and biscuit", ingredients: "Süt, muz, bisküvi", price: 165, allergens: []string{"sut", "gluten"}},
			{nameTR: "Brownie", nameEN: "Brownie", descTR: "Sıcak servis, dondurma ile", descEN: "Served warm with ice cream", ingredients: "Çikolata, tereyağı, yumurta, ceviz", price: 195, allergens: []string{"gluten", "sut", "yumurta", "findik"}},
			{nameTR: "Künefe", nameEN: "Künefe", descTR: "Antep fıstıklı, tek kişilik", descEN: "With pistachio, single portion", ingredients: "Kadayıf, peynir, şerbet, fıstık", price: 235, allergens: []string{"gluten", "sut", "findik"}},
		},
	},
}

// SeedDemo creates the demo cafe account and its menu.
// It does nothing when the account already exists, so it is safe to re-run.
func SeedDemo(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM businesses WHERE slug = $1)`, demoSlug).Scan(&exists)
	if err != nil {
		return fmt.Errorf("demo lookup failed: %w", err)
	}
	if exists {
		return nil
	}

	hash, err := utils.HashPassword(demoPassword)
	if err != nil {
		return fmt.Errorf("could not hash the demo password: %w", err)
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
		demoEmail, hash, "Demo Kafe").Scan(&userID)
	if err != nil {
		return fmt.Errorf("could not create the demo user: %w", err)
	}

	var businessID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO businesses (
			user_id, name, slug, currency, theme, font_family, primary_color,
			default_language, languages, splash_enabled, splash_duration,
			splash_bg_color, splash_text, address, phone, instagram, wifi_password
		) VALUES (
			$1, $2, $3, 'TRY', 'modern-light', 'inter', '#1d4ed8',
			'tr', $4, true, 1200,
			'#0f172a', 'Hoş geldiniz', $5, $6, $7, $8
		)
		RETURNING id`,
		userID, "Demo Kafe", demoSlug, []string{"tr", "en"},
		"Moda Cd. No:12, Kadıköy / İstanbul", "+90 555 000 00 00", "demokafe", "kahve2026",
	).Scan(&businessID)
	if err != nil {
		return fmt.Errorf("could not create the demo business: %w", err)
	}

	for categoryIndex, category := range demoMenu {
		categoryTranslations := models.Translations{
			"tr": {Name: category.nameTR},
			"en": {Name: category.nameEN},
		}

		var categoryID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO categories (business_id, translations, icon, position, is_active)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id`,
			businessID, categoryTranslations, category.icon, categoryIndex).Scan(&categoryID)
		if err != nil {
			return fmt.Errorf("could not create the demo category (%s): %w", category.nameTR, err)
		}

		for productIndex, product := range category.products {
			productTranslations := models.Translations{
				"tr": {Name: product.nameTR, Description: product.descTR, Ingredients: product.ingredients},
				"en": {Name: product.nameEN, Description: product.descEN},
			}
			allergens := product.allergens
			if allergens == nil {
				allergens = []string{}
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO products (
					business_id, category_id, translations, price,
					allergens, is_active, is_featured, position
				) VALUES ($1, $2, $3, $4, $5, true, $6, $7)`,
				businessID, categoryID, productTranslations, product.price,
				allergens, product.featured, productIndex)
			if err != nil {
				return fmt.Errorf("could not create the demo product (%s): %w", product.nameTR, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("could not save the demo data: %w", err)
	}

	log.Printf("[karecik] demo menu created -> %s / %s (slug: %s)",
		demoEmail, demoPassword, demoSlug)
	return nil
}
