-- Karecik — reverse script for 005_menus_as_primary_entities.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 005 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/005_menus_as_primary_entities.down.sql
--
-- ###########################################################################
-- #  IT CANNOT BRING THE BRANCHES BACK.                                     #
-- #                                                                         #
-- #  005 dropped branches, branch_menus and branch_product_prices, and no    #
-- #  copy of those rows survives anywhere in the database. This script       #
-- #  recreates the three tables EMPTY so the schema is structurally valid    #
-- #  again — every branch, every branch/menu link and every branch-specific  #
-- #  price is GONE FOREVER. The only way back is the dump you were told to   #
-- #  take before running 005:                                               #
-- #  C:\Users\BAHA\Desktop\karecik-backups\karecik-pre-005-menus-pivot.sql   #
-- ###########################################################################
--
-- What it DOES restore: every businesses column 005 dropped, refilled from
-- each business' first menu, plus menus.is_default and its partial unique
-- index, without which the older code cannot resolve a menu at all.
--
-- businesses.slug is NOT on that list, because 005 does not drop it: the
-- subdomain has always identified the TENANT, and rolling back does not
-- change that.
--
-- Menu slugs are deliberately LEFT ALONE, exactly as 005 leaves them. They are
-- path segments under the business subdomain, so no printed QR code breaks
-- rolling forward OR rolling back.
--
-- Statements are guarded and the copy-back is gated on the menus settings
-- columns still existing, so the script is safe to run twice.

-- ====================================================== 1. businesses columns
-- Re-added with the defaults 001, 002, 003 and 004 gave them. slug is missing
-- from the list on purpose — 005 never dropped it, so it is still there, still
-- NOT NULL and still globally unique.
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS logo_url  TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS cover_url TEXT;

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS currency      TEXT NOT NULL DEFAULT 'TRY';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS theme         TEXT NOT NULL DEFAULT 'modern-light';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS font_family   TEXT NOT NULL DEFAULT 'inter';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS primary_color TEXT NOT NULL DEFAULT '#1d4ed8';

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS default_language TEXT NOT NULL DEFAULT 'tr';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS languages        JSONB NOT NULL DEFAULT '["tr"]'::jsonb;

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_enabled  BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_duration INTEGER NOT NULL DEFAULT 1200;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_bg_color TEXT NOT NULL DEFAULT '#0f172a';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_text     TEXT NOT NULL DEFAULT '';

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS show_vat_note    BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS vat_note_text    TEXT NOT NULL DEFAULT 'Fiyatlarımıza KDV dahildir.';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS show_price_date  BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS price_updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS phone         TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS address       TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS instagram     TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS wifi_ssid     TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS wifi_password TEXT;

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_type            TEXT NOT NULL DEFAULT 'color';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_color           TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_image_url       TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_overlay_opacity NUMERIC(3,2) NOT NULL DEFAULT 0.40;

ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_logo_url       TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_headline       TEXT NOT NULL DEFAULT '';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_exit_animation TEXT NOT NULL DEFAULT 'fade';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_exit_duration  INTEGER NOT NULL DEFAULT 450;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_exit_easing    TEXT NOT NULL DEFAULT 'ease-in';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_display        TEXT NOT NULL DEFAULT 'both';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS header_display        TEXT NOT NULL DEFAULT 'both';

-- The CHECK constraints went down with the columns; 001, 003 and the widened
-- 004 versions are re-attached here.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_currency_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_currency_check
            CHECK (currency IN ('TRY','USD','EUR','GBP','AZN','RUB','SAR','AED'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_splash_duration_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_splash_duration_check
            CHECK (splash_duration BETWEEN 300 AND 5000);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_background_type_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_background_type_check
            CHECK (background_type IN ('color','image'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_background_overlay_opacity_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_background_overlay_opacity_check
            CHECK (background_overlay_opacity >= 0 AND background_overlay_opacity <= 1);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_splash_exit_animation_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_splash_exit_animation_check
            CHECK (splash_exit_animation IN
                ('fade','slide-up','slide-down','slide-left','slide-right','zoom-in','zoom-out'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_splash_exit_duration_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_splash_exit_duration_check
            CHECK (splash_exit_duration BETWEEN 100 AND 2000);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_splash_exit_easing_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_splash_exit_easing_check
            CHECK (splash_exit_easing IN ('linear','ease','ease-in','ease-out','ease-in-out'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_splash_display_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_splash_display_check
            CHECK (splash_display IN ('logo','text','both'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'businesses_header_display_check'
          AND conrelid = 'businesses'::regclass
    ) THEN
        ALTER TABLE businesses
            ADD CONSTRAINT businesses_header_display_check
            CHECK (header_display IN ('logo','name','both'));
    END IF;
END $$;

-- ======================================================== 2. menus.is_default
-- BEFORE the copy-back, which orders by it. 005 dropped the column and its
-- partial unique index; the code that predates 005 cannot find a menu without
-- them, so every business gets its lowest-position menu flagged. A business
-- with no menus at all is left with nothing flagged, which the partial index
-- tolerates — the older code will simply see no default menu there.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;

UPDATE menus m
SET is_default = true
WHERE NOT EXISTS (
        SELECT 1 FROM menus d
        WHERE d.business_id = m.business_id AND d.is_default
      )
  AND m.id = (
        SELECT m2.id FROM menus m2
        WHERE m2.business_id = m.business_id
        ORDER BY m2.position, m2.id
        LIMIT 1
      );

-- Copied verbatim from 003, so a re-applied 003 finds exactly what it expects.
CREATE UNIQUE INDEX IF NOT EXISTS menus_one_default_idx
    ON menus (business_id) WHERE is_default;

-- ========================================================== 3. copy the values
-- back out of each business' default menu — falling back to its first menu
-- when no menu is flagged default. currency_symbol has no counterpart on
-- businesses: the old Go code derived it from currency on every read, so it is
-- simply dropped. splash_slide_fade has no counterpart either and is lost.
--
-- Gated on the menus settings columns still existing, so a second run of this
-- script is a no-op rather than a parse error.
DO $do$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'menus'::regclass
          AND attname  = 'theme'
          AND attnum   > 0
          AND NOT attisdropped
    ) THEN
        RETURN;
    END IF;

    EXECUTE $copy$
        UPDATE businesses b SET
            logo_url                   = m.logo_url,
            cover_url                  = m.cover_url,
            currency                   = m.currency,
            theme                      = m.theme,
            font_family                = m.font_family,
            primary_color              = m.primary_color,
            default_language           = m.default_language,
            languages                  = m.languages,
            splash_enabled             = m.splash_enabled,
            splash_duration            = m.splash_duration,
            splash_bg_color            = m.splash_bg_color,
            splash_text                = m.splash_text,
            splash_logo_url            = m.splash_logo_url,
            splash_headline            = m.splash_headline,
            splash_exit_animation      = m.splash_exit_animation,
            splash_exit_duration       = m.splash_exit_duration,
            splash_exit_easing         = m.splash_exit_easing,
            splash_display             = m.splash_display,
            background_type            = m.background_type,
            background_color           = m.background_color,
            background_image_url       = m.background_image_url,
            background_overlay_opacity = m.background_overlay_opacity,
            header_display             = m.header_display,
            show_vat_note              = m.show_vat_note,
            vat_note_text              = m.vat_note_text,
            show_price_date            = m.show_price_date,
            price_updated_at           = m.price_updated_at,
            phone                      = m.phone,
            address                    = m.address,
            instagram                  = m.instagram,
            wifi_ssid                  = m.wifi_ssid,
            wifi_password              = m.wifi_password
        FROM (
            SELECT DISTINCT ON (business_id) *
            FROM menus
            ORDER BY business_id, is_default DESC, position, id
        ) m
        WHERE m.business_id = b.id
    $copy$;
END
$do$;

-- ====================================================== 4. categories.menu_id
-- 003 deliberately left it nullable: an unassigned category still belonged to
-- the business and showed on its default menu.
ALTER TABLE categories ALTER COLUMN menu_id DROP NOT NULL;

-- =========================================================== 5. the menu slugs
-- Only an EARLIER build of 005 ever created a global unique index on
-- menus (slug); the current one never does and drops it if it finds one. This
-- is therefore a no-op on a database that only ever saw the current 005, and
-- the cleanup that database needs if it saw the old one. UNIQUE
-- (business_id, slug) from 003 is untouched throughout.
DROP INDEX IF EXISTS menus_slug_key;

-- ================================================= 6. the branch tables, EMPTY
-- Structure only — see the warning at the top of this file. The DDL is copied
-- verbatim from 003 so a re-applied 003 finds exactly what it expects.
CREATE TABLE IF NOT EXISTS branches (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id   UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    slug          TEXT NOT NULL UNIQUE,
    phone         TEXT,
    address       TEXT,
    wifi_ssid     TEXT,
    wifi_password TEXT,
    is_default    BOOLEAN NOT NULL DEFAULT false,
    is_active     BOOLEAN NOT NULL DEFAULT true,
    position      INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS branches_business_position_idx
    ON branches (business_id, position);

CREATE UNIQUE INDEX IF NOT EXISTS branches_one_default_idx
    ON branches (business_id) WHERE is_default;

DROP TRIGGER IF EXISTS branches_set_updated_at ON branches;
CREATE TRIGGER branches_set_updated_at BEFORE UPDATE ON branches
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

CREATE TABLE IF NOT EXISTS branch_menus (
    branch_id  UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    menu_id    UUID NOT NULL REFERENCES menus(id)    ON DELETE CASCADE,
    position   INTEGER NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (branch_id, menu_id)
);

CREATE INDEX IF NOT EXISTS branch_menus_menu_idx ON branch_menus (menu_id);

CREATE UNIQUE INDEX IF NOT EXISTS branch_menus_one_default_idx
    ON branch_menus (branch_id) WHERE is_default;

CREATE TABLE IF NOT EXISTS branch_product_prices (
    branch_id     UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    product_id    UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    price         NUMERIC(12,2) CHECK (price IS NULL OR price >= 0),
    compare_price NUMERIC(12,2) CHECK (compare_price IS NULL OR compare_price >= 0),
    is_available  BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (branch_id, product_id)
);

CREATE INDEX IF NOT EXISTS branch_product_prices_product_idx
    ON branch_product_prices (product_id);

DROP TRIGGER IF EXISTS branch_product_prices_set_updated_at ON branch_product_prices;
CREATE TRIGGER branch_product_prices_set_updated_at BEFORE UPDATE ON branch_product_prices
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

-- ============================================================ 7. menus columns
-- LAST: section 2 read every one of them. Dropping the columns also drops the
-- nine menus_*_check constraints 005 attached to them.
ALTER TABLE menus DROP COLUMN IF EXISTS phone;
ALTER TABLE menus DROP COLUMN IF EXISTS address;
ALTER TABLE menus DROP COLUMN IF EXISTS instagram;
ALTER TABLE menus DROP COLUMN IF EXISTS wifi_ssid;
ALTER TABLE menus DROP COLUMN IF EXISTS wifi_password;

ALTER TABLE menus DROP COLUMN IF EXISTS logo_url;
ALTER TABLE menus DROP COLUMN IF EXISTS cover_url;

ALTER TABLE menus DROP COLUMN IF EXISTS theme;
ALTER TABLE menus DROP COLUMN IF EXISTS font_family;
ALTER TABLE menus DROP COLUMN IF EXISTS primary_color;
ALTER TABLE menus DROP COLUMN IF EXISTS background_type;
ALTER TABLE menus DROP COLUMN IF EXISTS background_color;
ALTER TABLE menus DROP COLUMN IF EXISTS background_image_url;
ALTER TABLE menus DROP COLUMN IF EXISTS background_overlay_opacity;

ALTER TABLE menus DROP COLUMN IF EXISTS splash_enabled;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_logo_url;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_headline;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_text;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_bg_color;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_duration;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_exit_animation;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_exit_duration;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_exit_easing;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_display;
ALTER TABLE menus DROP COLUMN IF EXISTS splash_slide_fade;

ALTER TABLE menus DROP COLUMN IF EXISTS currency;
ALTER TABLE menus DROP COLUMN IF EXISTS currency_symbol;

ALTER TABLE menus DROP COLUMN IF EXISTS show_vat_note;
ALTER TABLE menus DROP COLUMN IF EXISTS vat_note_text;
ALTER TABLE menus DROP COLUMN IF EXISTS show_price_date;
ALTER TABLE menus DROP COLUMN IF EXISTS price_updated_at;

ALTER TABLE menus DROP COLUMN IF EXISTS header_display;
ALTER TABLE menus DROP COLUMN IF EXISTS default_language;
ALTER TABLE menus DROP COLUMN IF EXISTS languages;

-- ---------------------------------------------------------- bookkeeping
DELETE FROM schema_migrations WHERE version = '005_menus_as_primary_entities.sql';
