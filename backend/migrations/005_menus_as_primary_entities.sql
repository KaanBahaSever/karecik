-- Karecik — menus become the primary entity, branches are removed
--
-- THIS MIGRATION IS DESTRUCTIVE AND STRICTLY ORDER-DEPENDENT. Read the header
-- before touching a single line of it.
--
-- The branch concept is gone. What a customer opens is
--
--     {business-slug}.karecik.com / {menu-slug}
--      └── identifies the TENANT    └── identifies ONE MENU inside that tenant
--
-- so businesses.slug STAYS: it is the subdomain, it is globally unique and it
-- is reserved-checked because it is a hostname label. menus.slug is only a
-- path segment and is unique WITHIN its business — two tenants are both
-- entitled to publish 'kahvalti'.
--
-- Every published setting that used to sit on the business — branding,
-- background, splash screen, contact details, currency, languages, footer
-- notices — moves onto the menu, and `businesses` is left holding nothing but
-- the account and its address: id, user_id, name, slug, created_at,
-- updated_at.
--
-- There is no default menu any more. is_default and menus_one_default_idx go,
-- deleting a menu never promotes another one, and a business may own zero
-- menus.
--
-- The order of the six sections below is not negotiable:
--
--   1. ADD every settings column to menus (all nullable or defaulted, so the
--      ADD can never fail on an existing row);
--   2. COPY the values across from businesses, deriving currency_symbol with a
--      CASE that matches utils.CurrencySymbol exactly;
--   3. make sure menus carries the COMPOSITE UNIQUE (business_id, slug) that
--      003 declared, and drop is_default;
--   4. DROP the branch tables;
--   5. make categories.menu_id NOT NULL;
--   6. only NOW strip businesses down to the account columns.
--
-- Copy before drop. Section 6 destroys the source of section 2.
--
-- Idempotent throughout. Section 2 reads columns that section 6 removes, so it
-- cannot be written as plain SQL — a second run would fail while parsing it,
-- before it ever got the chance to skip it. It lives inside a DO block that
-- checks pg_attribute for businesses.theme and returns without doing anything
-- once that column is gone, and the statement itself is EXECUTEd so PL/pgSQL
-- never resolves the doomed column names on a run that skips it. The sentinel
-- is theme and NOT slug, precisely because slug is the one column section 6
-- leaves behind.
--
-- CHECK constraints cannot be added with IF NOT EXISTS, so they are attached
-- inside guarded DO blocks, exactly the way 003 and 004 do it.
--
-- The reverse script lives in migrations/down/ and is NEVER executed
-- automatically — see the note at the top of that file. It cannot bring the
-- branch rows back.

-- ============================================================ 1. menus columns
-- Every published setting of a menu. All nullable or defaulted, so the ADD is
-- harmless on the rows that already exist; section 2 immediately replaces the
-- defaults with whatever the business used to hold.

-- Contact details
ALTER TABLE menus ADD COLUMN IF NOT EXISTS phone         TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS address       TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS instagram     TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS wifi_ssid     TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS wifi_password TEXT;

-- Imagery. logo_url feeds header_display = 'logo' and the splash logo
-- fallback; cover_url is carried straight through to PublicBusiness.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS logo_url  TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS cover_url TEXT;

-- Appearance
ALTER TABLE menus ADD COLUMN IF NOT EXISTS theme         TEXT NOT NULL DEFAULT 'modern-light';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS font_family   TEXT NOT NULL DEFAULT 'inter';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS primary_color TEXT NOT NULL DEFAULT '#1d4ed8';

-- Menu background: either a flat colour or an image with a darkening overlay.
-- background_color NULL means "inherit the theme background".
ALTER TABLE menus ADD COLUMN IF NOT EXISTS background_type            TEXT NOT NULL DEFAULT 'color';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS background_color           TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS background_image_url       TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS background_overlay_opacity NUMERIC(3,2) NOT NULL DEFAULT 0.40;

-- Splash screen. splash_text is the tagline under the headline,
-- splash_headline the larger line and splash_duration the hold time before the
-- exit animation starts.
--
-- splash_slide_fade is new and has no counterpart on businesses: true (the
-- default, today's behaviour) fades the panel to zero opacity while it slides
-- away, false keeps it fully opaque so it leaves like a solid curtain. It only
-- applies to the four slide-* exit animations.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_enabled        BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_logo_url       TEXT;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_headline       TEXT NOT NULL DEFAULT '';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_text           TEXT NOT NULL DEFAULT '';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_bg_color       TEXT NOT NULL DEFAULT '#0f172a';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_duration       INTEGER NOT NULL DEFAULT 1200;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_exit_animation TEXT NOT NULL DEFAULT 'fade';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_exit_duration  INTEGER NOT NULL DEFAULT 450;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_exit_easing    TEXT NOT NULL DEFAULT 'ease-in';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_display        TEXT NOT NULL DEFAULT 'both';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_slide_fade     BOOLEAN NOT NULL DEFAULT true;

-- Currency. currency_symbol is a derived copy of currency and the repository
-- layer is its ONLY writer: CreateMenu and UpdateMenu set it from
-- utils.CurrencySymbol whenever currency is written. No handler and no API
-- payload may touch it — it is read-only to the outside world.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS currency        TEXT NOT NULL DEFAULT 'TRY';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS currency_symbol TEXT NOT NULL DEFAULT '₺';

-- Legal notices in the footer
ALTER TABLE menus ADD COLUMN IF NOT EXISTS show_vat_note    BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS vat_note_text    TEXT NOT NULL DEFAULT 'Fiyatlarımıza KDV dahildir.';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS show_price_date  BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE menus ADD COLUMN IF NOT EXISTS price_updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Header and languages
ALTER TABLE menus ADD COLUMN IF NOT EXISTS header_display   TEXT NOT NULL DEFAULT 'both';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS default_language TEXT NOT NULL DEFAULT 'tr';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS languages        JSONB NOT NULL DEFAULT '["tr"]'::jsonb;

-- The constraint names follow the menus_<column>_check pattern and mirror the
-- businesses_<column>_check set that 001, 003 and 004 attached.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_background_type_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_background_type_check
            CHECK (background_type IN ('color','image'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_background_overlay_opacity_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_background_overlay_opacity_check
            CHECK (background_overlay_opacity >= 0 AND background_overlay_opacity <= 1);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_splash_duration_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_splash_duration_check
            CHECK (splash_duration BETWEEN 300 AND 5000);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_splash_exit_animation_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_splash_exit_animation_check
            CHECK (splash_exit_animation IN
                ('fade','slide-up','slide-down','slide-left','slide-right','zoom-in','zoom-out'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_splash_exit_duration_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_splash_exit_duration_check
            CHECK (splash_exit_duration BETWEEN 100 AND 2000);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_splash_exit_easing_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_splash_exit_easing_check
            CHECK (splash_exit_easing IN ('linear','ease','ease-in','ease-out','ease-in-out'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_splash_display_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_splash_display_check
            CHECK (splash_display IN ('logo','text','both'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_currency_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_currency_check
            CHECK (currency IN ('TRY','USD','EUR','GBP','AZN','RUB','SAR','AED'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_header_display_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_header_display_check
            CHECK (header_display IN ('logo','name','both'));
    END IF;
END $$;

-- ============================================================== 2. copy across
-- One pass over every menu, pulling the settings from its business. This is
-- the only chance to read them: section 6 drops the source columns.
--
-- currency_symbol is derived here with a CASE over the currency code so that
-- it matches utils.CurrencySymbol byte for byte — the Go side never re-derives
-- a stored symbol, it just prints it.
--
-- The whole statement is gated on businesses.theme still existing. slug would
-- be the wrong sentinel now that section 6 keeps it: the gate would never
-- close and the second run of this migration would fail on b.theme. Once
-- section 6 has run the copy cannot run again, and it must not: the menus
-- already hold the authored values and businesses has nothing left to give.
DO $do$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'businesses'::regclass
          AND attname  = 'theme'
          AND attnum   > 0
          AND NOT attisdropped
    ) THEN
        RETURN;
    END IF;

    EXECUTE $copy$
        UPDATE menus m SET
            phone                      = b.phone,
            address                    = b.address,
            instagram                  = b.instagram,
            wifi_ssid                  = b.wifi_ssid,
            wifi_password              = b.wifi_password,
            logo_url                   = b.logo_url,
            cover_url                  = b.cover_url,
            theme                      = b.theme,
            font_family                = b.font_family,
            primary_color              = b.primary_color,
            background_type            = b.background_type,
            background_color           = b.background_color,
            background_image_url       = b.background_image_url,
            background_overlay_opacity = b.background_overlay_opacity,
            splash_enabled             = b.splash_enabled,
            splash_logo_url            = b.splash_logo_url,
            splash_headline            = b.splash_headline,
            splash_text                = b.splash_text,
            splash_bg_color            = b.splash_bg_color,
            splash_duration            = b.splash_duration,
            splash_exit_animation      = b.splash_exit_animation,
            splash_exit_duration       = b.splash_exit_duration,
            splash_exit_easing         = b.splash_exit_easing,
            splash_display             = b.splash_display,
            currency                   = b.currency,
            currency_symbol            = CASE b.currency
                                             WHEN 'TRY' THEN '₺'
                                             WHEN 'USD' THEN '$'
                                             WHEN 'EUR' THEN '€'
                                             WHEN 'GBP' THEN '£'
                                             WHEN 'AZN' THEN '₼'
                                             WHEN 'RUB' THEN '₽'
                                             WHEN 'SAR' THEN '﷼'
                                             WHEN 'AED' THEN 'د.إ'
                                             ELSE '₺'
                                         END,
            show_vat_note              = b.show_vat_note,
            vat_note_text              = b.vat_note_text,
            show_price_date            = b.show_price_date,
            price_updated_at           = b.price_updated_at,
            header_display             = b.header_display,
            default_language           = b.default_language,
            languages                  = b.languages
        FROM businesses b
        WHERE b.id = m.business_id
    $copy$;
END
$do$;

-- ================================================ 3. the menu slug namespace
-- A menu slug is a PATH segment under its tenant's subdomain, never a
-- subdomain of its own, so it is unique only WITHIN its business. Menu slugs
-- are therefore left exactly as their owners named them: nothing is rewritten
-- and nothing is de-duplicated. Printed QR codes keep resolving because the
-- BUSINESS slug - which is what they encoded as the host - survives section 6.
--
-- 003 already declared UNIQUE (business_id, slug) inline on the menus table,
-- which PostgreSQL named menus_business_id_slug_key. This only re-creates the
-- constraint for a hand-patched database that somehow lost it.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_business_id_slug_key'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_business_id_slug_key UNIQUE (business_id, slug);
    END IF;
END $$;

-- An earlier build of THIS migration created a GLOBAL unique index on
-- menus (slug), back when a menu slug was the subdomain. It must not survive:
-- under the tenant-subdomain model two businesses are both entitled to a menu
-- called 'kahvalti'. Dropped IF EXISTS, so a database that never saw that
-- build is untouched, and by name, so the composite constraint's own index is
-- never at risk.
DROP INDEX IF EXISTS menus_slug_key;

-- --------------------------------------------------------------- no default
-- There is no default menu any more: the customer always names one in the
-- path, deleting a menu never promotes another one, and a business may own
-- zero menus. Dropped AFTER the copy in section 2 - which never read it - and
-- BEFORE section 5, which no longer orders by it.
DROP INDEX IF EXISTS menus_one_default_idx;
ALTER TABLE menus DROP COLUMN IF EXISTS is_default;

-- ============================================================ 4. drop branches
-- Reverse dependency order. branch_prices is the name the original brief used
-- for the table 003 actually created as branch_product_prices; it is dropped
-- too, IF EXISTS, in case an older database really carries one.
DROP TABLE IF EXISTS branch_product_prices;
DROP TABLE IF EXISTS branch_prices;
DROP TABLE IF EXISTS branch_menus;
DROP TABLE IF EXISTS branches;

-- ====================================================== 5. categories.menu_id
-- Every category belongs to exactly one menu from here on, so the old "menu_id
-- IS NULL means the default menu" branch disappears from the query layer.
-- Stragglers are attached to their business' FIRST menu by position — there is
-- no default to prefer any more, and section 3 has just dropped the column
-- that used to break the tie — and only then is the column tightened. 003
-- already backfilled both a menu and a menu_id for every business that existed
-- when it ran, so this is a safety net rather than the main event.
UPDATE categories c
SET menu_id = m.id
FROM (
    SELECT DISTINCT ON (business_id) business_id, id
    FROM menus
    ORDER BY business_id, position, id
) m
WHERE m.business_id = c.business_id AND c.menu_id IS NULL;

-- categories_menu_id_fkey already exists from 003 with ON DELETE CASCADE; it
-- is only created when a hand-patched database is missing it.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'categories_menu_id_fkey'
          AND conrelid = 'categories'::regclass
    ) THEN
        ALTER TABLE categories
            ADD CONSTRAINT categories_menu_id_fkey
            FOREIGN KEY (menu_id) REFERENCES menus(id) ON DELETE CASCADE;
    END IF;
END $$;

ALTER TABLE categories ALTER COLUMN menu_id SET NOT NULL;

-- ========================================================= 6. strip businesses
-- LAST. Everything above reads these columns; from here on `businesses` is the
-- account and its public address and nothing else: id, user_id, name, slug,
-- created_at, updated_at.
--
-- slug STAYS. It is the tenant subdomain {business-slug}.karecik.com, so it is
-- globally unique and reserved-checked as a hostname label. 001 declared it
-- NOT NULL UNIQUE, which already gave it an index named businesses_slug_key;
-- the statement below only re-creates that index for a hand-patched database
-- that lost it, and businesses_slug_idx from 001 is deliberately left alone.
CREATE UNIQUE INDEX IF NOT EXISTS businesses_slug_key ON businesses (slug);

-- Status — a tenant is visible when it has active menus, so is_active goes
ALTER TABLE businesses DROP COLUMN IF EXISTS is_active;

-- Imagery
ALTER TABLE businesses DROP COLUMN IF EXISTS logo_url;
ALTER TABLE businesses DROP COLUMN IF EXISTS cover_url;

-- Appearance and background
ALTER TABLE businesses DROP COLUMN IF EXISTS theme;
ALTER TABLE businesses DROP COLUMN IF EXISTS font_family;
ALTER TABLE businesses DROP COLUMN IF EXISTS primary_color;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_type;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_color;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_image_url;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_overlay_opacity;

-- Splash screen
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_enabled;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_logo_url;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_headline;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_text;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_bg_color;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_duration;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_exit_animation;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_exit_duration;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_exit_easing;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_display;

-- Currency, footer notices and the header
ALTER TABLE businesses DROP COLUMN IF EXISTS currency;
ALTER TABLE businesses DROP COLUMN IF EXISTS show_vat_note;
ALTER TABLE businesses DROP COLUMN IF EXISTS vat_note_text;
ALTER TABLE businesses DROP COLUMN IF EXISTS show_price_date;
ALTER TABLE businesses DROP COLUMN IF EXISTS price_updated_at;
ALTER TABLE businesses DROP COLUMN IF EXISTS header_display;

-- Languages
ALTER TABLE businesses DROP COLUMN IF EXISTS default_language;
ALTER TABLE businesses DROP COLUMN IF EXISTS languages;

-- Contact details
ALTER TABLE businesses DROP COLUMN IF EXISTS phone;
ALTER TABLE businesses DROP COLUMN IF EXISTS address;
ALTER TABLE businesses DROP COLUMN IF EXISTS instagram;
ALTER TABLE businesses DROP COLUMN IF EXISTS wifi_ssid;
ALTER TABLE businesses DROP COLUMN IF EXISTS wifi_password;
