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

type seedProduct struct {
	tr, en      string
	descTR      string
	descEN      string
	ingredients string
	price       float64
	allergens   []string
	featured    bool
}

type seedCategory struct {
	tr, en   string
	icon     string
	products []seedProduct
}

// demoMenu — landing page'deki iPhone onizlemesinde gorunen ornek kafe menusu.
var demoMenu = []seedCategory{
	{
		tr: "Sıcak İçecekler", en: "Hot Drinks", icon: "☕",
		products: []seedProduct{
			{tr: "Türk Kahvesi", en: "Turkish Coffee", descTR: "Geleneksel közde pişirilmiş, lokum ikramıyla", descEN: "Traditional, served with Turkish delight", ingredients: "Türk kahvesi, su", price: 85, allergens: []string{"kafein"}, featured: true},
			{tr: "Espresso", en: "Espresso", descTR: "Tek shot, yoğun aroma", descEN: "Single shot, intense aroma", ingredients: "Espresso", price: 75, allergens: []string{"kafein"}},
			{tr: "Latte", en: "Caffè Latte", descTR: "Espresso ve buharda ısıtılmış süt", descEN: "Espresso with steamed milk", ingredients: "Espresso, süt", price: 145, allergens: []string{"sut", "kafein"}, featured: true},
			{tr: "Cappuccino", en: "Cappuccino", descTR: "Bol köpüklü, tarçın seçeneğiyle", descEN: "Extra foam, cinnamon optional", ingredients: "Espresso, süt", price: 140, allergens: []string{"sut", "kafein"}},
			{tr: "Sahlep", en: "Salep", descTR: "Tarçınlı, kış klasiği", descEN: "With cinnamon, a winter classic", ingredients: "Süt, salep, tarçın", price: 130, allergens: []string{"sut"}},
			{tr: "Bitki Çayı", en: "Herbal Tea", descTR: "Ihlamur, nane-limon veya kuşburnu", descEN: "Linden, mint-lemon or rosehip", price: 70, allergens: []string{}},
		},
	},
	{
		tr: "Soğuk İçecekler", en: "Cold Drinks", icon: "🥤",
		products: []seedProduct{
			{tr: "Ice Latte", en: "Iced Latte", descTR: "Buz üzerine espresso ve soğuk süt", descEN: "Espresso over ice with cold milk", ingredients: "Espresso, süt, buz", price: 155, allergens: []string{"sut", "kafein"}, featured: true},
			{tr: "Limonata", en: "Lemonade", descTR: "Taze sıkılmış, naneli", descEN: "Freshly squeezed, with mint", ingredients: "Limon, su, şeker, nane", price: 110, allergens: []string{}},
			{tr: "Ev Yapımı Buzlu Çay", en: "Homemade Iced Tea", descTR: "Şeftali veya orman meyveli", descEN: "Peach or berry", price: 105, allergens: []string{}},
			{tr: "Milkshake", en: "Milkshake", descTR: "Çilek, muz veya çikolata", descEN: "Strawberry, banana or chocolate", ingredients: "Süt, dondurma", price: 165, allergens: []string{"sut"}},
			{tr: "Maden Suyu", en: "Sparkling Water", descTR: "Sade veya meyveli", descEN: "Plain or flavoured", price: 45, allergens: []string{}},
		},
	},
	{
		tr: "Yiyecekler", en: "Food", icon: "🍽️",
		products: []seedProduct{
			{tr: "Serpme Kahvaltı", en: "Turkish Breakfast", descTR: "İki kişilik, sınırsız çay ile", descEN: "For two, with unlimited tea", ingredients: "Peynir çeşitleri, zeytin, bal, kaymak, yumurta", price: 890, allergens: []string{"sut", "yumurta", "gluten"}, featured: true},
			{tr: "Kaşarlı Tost", en: "Cheese Toastie", descTR: "Köy ekmeğinde, turşu ile", descEN: "On sourdough, served with pickles", ingredients: "Ekmek, kaşar peyniri, tereyağı", price: 175, allergens: []string{"gluten", "sut"}},
			{tr: "Menemen", en: "Menemen", descTR: "Domates, biber ve yumurta", descEN: "Tomato, pepper and eggs", ingredients: "Yumurta, domates, biber, tereyağı", price: 245, allergens: []string{"yumurta", "sut"}},
			{tr: "Tavuklu Sezar Salata", en: "Chicken Caesar Salad", descTR: "Izgara tavuk, parmesan, kruton", descEN: "Grilled chicken, parmesan, croutons", ingredients: "Marul, tavuk, parmesan, kruton, sezar sos", price: 320, allergens: []string{"gluten", "sut", "yumurta"}},
			{tr: "Kulüp Sandviç", en: "Club Sandwich", descTR: "Patates kızartması eşliğinde", descEN: "Served with french fries", ingredients: "Tost ekmeği, tavuk, marul, domates", price: 295, allergens: []string{"gluten", "yumurta"}},
		},
	},
	{
		tr: "Tatlılar", en: "Desserts", icon: "🍰",
		products: []seedProduct{
			{tr: "San Sebastian Cheesecake", en: "San Sebastian Cheesecake", descTR: "Günlük yapım, dilim", descEN: "Made daily, per slice", ingredients: "Labne, krema, yumurta, şeker", price: 210, allergens: []string{"sut", "yumurta", "gluten"}, featured: true},
			{tr: "Magnolia", en: "Magnolia", descTR: "Muzlu, bisküvili", descEN: "Banana and biscuit", ingredients: "Süt, muz, bisküvi", price: 165, allergens: []string{"sut", "gluten"}},
			{tr: "Brownie", en: "Brownie", descTR: "Sıcak servis, dondurma ile", descEN: "Served warm with ice cream", ingredients: "Çikolata, tereyağı, yumurta, ceviz", price: 195, allergens: []string{"gluten", "sut", "yumurta", "findik"}},
			{tr: "Künefe", en: "Künefe", descTR: "Antep fıstıklı, tek kişilik", descEN: "With pistachio, single portion", ingredients: "Kadayıf, peynir, şerbet, fıstık", price: 235, allergens: []string{"gluten", "sut", "findik"}},
		},
	},
}

// SeedDemo, demo kafe hesabini ve menusunu olusturur.
// Hesap zaten varsa hicbir sey yapmaz (tekrar calistirilabilir).
func SeedDemo(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM businesses WHERE slug = $1)`, demoSlug).Scan(&exists)
	if err != nil {
		return fmt.Errorf("demo kontrolu basarisiz: %w", err)
	}
	if exists {
		return nil
	}

	hash, err := utils.HashPassword(demoPassword)
	if err != nil {
		return fmt.Errorf("demo sifresi hashlenemedi: %w", err)
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
		return fmt.Errorf("demo kullanicisi olusturulamadi: %w", err)
	}

	var businessID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO businesses (
			user_id, name, slug, currency, theme, font_family, primary_color,
			default_language, languages, splash_enabled, splash_duration,
			splash_bg_color, splash_text, address, phone, instagram, wifi_password
		) VALUES (
			$1, $2, $3, 'TRY', 'modern-light', 'inter', '#1a7f5a',
			'tr', $4, true, 1200,
			'#0f172a', 'Hoş geldiniz', $5, $6, $7, $8
		)
		RETURNING id`,
		userID, "Demo Kafe", demoSlug, []string{"tr", "en"},
		"Moda Cd. No:12, Kadıköy / İstanbul", "+90 555 000 00 00", "demokafe", "kahve2026",
	).Scan(&businessID)
	if err != nil {
		return fmt.Errorf("demo isletmesi olusturulamadi: %w", err)
	}

	for ci, cat := range demoMenu {
		catTranslations := models.Translations{
			"tr": {Name: cat.tr},
			"en": {Name: cat.en},
		}

		var categoryID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO categories (business_id, translations, icon, position, is_active)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id`,
			businessID, catTranslations, cat.icon, ci).Scan(&categoryID)
		if err != nil {
			return fmt.Errorf("demo kategorisi olusturulamadi (%s): %w", cat.tr, err)
		}

		for pi, p := range cat.products {
			prodTranslations := models.Translations{
				"tr": {Name: p.tr, Description: p.descTR, Ingredients: p.ingredients},
				"en": {Name: p.en, Description: p.descEN},
			}
			allergens := p.allergens
			if allergens == nil {
				allergens = []string{}
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO products (
					business_id, category_id, translations, price,
					allergens, is_active, is_featured, position
				) VALUES ($1, $2, $3, $4, $5, true, $6, $7)`,
				businessID, categoryID, prodTranslations, p.price,
				allergens, p.featured, pi)
			if err != nil {
				return fmt.Errorf("demo urunu olusturulamadi (%s): %w", p.tr, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("demo verisi kaydedilemedi: %w", err)
	}

	log.Printf("[karecik] demo menu olusturuldu -> %s / %s (slug: %s)",
		demoEmail, demoPassword, demoSlug)
	return nil
}
