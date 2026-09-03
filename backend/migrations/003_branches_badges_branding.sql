-- Karecik — branches, menus, product badges and menu branding
--
-- This migration is ADDITIVE ONLY: it adds columns, tables and indexes and
-- backfills the new rows from the existing ones. Nothing is dropped and no
-- user content is deleted or overwritten.
--
-- It is also idempotent: every statement is guarded, so re-running the file on
-- a partially migrated database is a no-op. CHECK constraints cannot be added
-- with IF NOT EXISTS, so they are attached inside guarded DO blocks.
--
-- The reverse script lives in migrations/down/ and is NEVER executed
-- automatically — see the note at the top of that file.

-- ------------------------------------------------------------- products
-- calories: kcal per serving, NULL when the business did not enter one.
-- badges:   free-form labels the business designs itself. The element shape is
--           validated in Go, not by the database:
--           {"id":"b1","text":"Şefin Önerisi","icon":"chef-hat",
--            "bg_color":"#1d4ed8","text_color":"#ffffff"}
ALTER TABLE products ADD COLUMN IF NOT EXISTS calories INTEGER;
ALTER TABLE products ADD COLUMN IF NOT EXISTS badges JSONB NOT NULL DEFAULT '[]'::jsonb;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'products_calories_check'
          AND conrelid = 'products'::regclass
    ) THEN
        ALTER TABLE products
            ADD CONSTRAINT products_calories_check
            CHECK (calories IS NULL OR (calories >= 0 AND calories <= 20000));
    END IF;
END $$;

-- ----------------------------------------------------------- businesses
-- Menu background: either a flat colour or an image with a darkening overlay.
-- background_color NULL means "inherit the theme background".
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_type TEXT NOT NULL DEFAULT 'color';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_color TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_image_url TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS background_overlay_opacity NUMERIC(3,2) NOT NULL DEFAULT 0.40;

-- wifi_password already exists; the network name did not.
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS wifi_ssid TEXT;

-- Splash screen. splash_text keeps its meaning — the tagline under the
-- headline; splash_headline is the new larger line and splash_duration stays
-- the hold time before the exit animation starts.
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_logo_url TEXT;
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_headline TEXT NOT NULL DEFAULT '';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_exit_animation TEXT NOT NULL DEFAULT 'fade';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_exit_duration INTEGER NOT NULL DEFAULT 450;

DO $$
BEGIN
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
            CHECK (splash_exit_animation IN ('slide-up','slide-down','slide-right','fade'));
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
END $$;

-- ---------------------------------------------------------------- menus
-- A business may publish several menus (kahvaltı, akşam, bar...). Every menu
-- owns a set of categories; at most one menu per business is the default.
CREATE TABLE IF NOT EXISTS menus (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_default  BOOLEAN NOT NULL DEFAULT false,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    position    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, slug)
);

CREATE INDEX IF NOT EXISTS menus_business_position_idx
    ON menus (business_id, position);

-- One business has at most one default menu
CREATE UNIQUE INDEX IF NOT EXISTS menus_one_default_idx
    ON menus (business_id) WHERE is_default;

DROP TRIGGER IF EXISTS menus_set_updated_at ON menus;
CREATE TRIGGER menus_set_updated_at BEFORE UPDATE ON menus
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

-- ------------------------------------------------------------- branches
-- slug is GLOBALLY unique because it is a subdomain: {branch}.karecik.com must
-- resolve without a business qualifier. Branch slugs and business slugs share
-- one namespace and the backend resolves the branch slug first.
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

-- One business has at most one default branch
CREATE UNIQUE INDEX IF NOT EXISTS branches_one_default_idx
    ON branches (business_id) WHERE is_default;

DROP TRIGGER IF EXISTS branches_set_updated_at ON branches;
CREATE TRIGGER branches_set_updated_at BEFORE UPDATE ON branches
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

-- --------------------------------------------------------- branch_menus
-- A menu may be shared by several branches and a branch may serve several
-- menus, so the link is many-to-many.
CREATE TABLE IF NOT EXISTS branch_menus (
    branch_id  UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    menu_id    UUID NOT NULL REFERENCES menus(id)    ON DELETE CASCADE,
    position   INTEGER NOT NULL DEFAULT 0,
    is_default BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (branch_id, menu_id)
);

CREATE INDEX IF NOT EXISTS branch_menus_menu_idx ON branch_menus (menu_id);

-- One branch has at most one default menu
CREATE UNIQUE INDEX IF NOT EXISTS branch_menus_one_default_idx
    ON branch_menus (branch_id) WHERE is_default;

-- ------------------------------------------------ branch_product_prices
-- Branch-specific pricing and availability. A NULL price means "inherit the
-- product's own price".
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

-- --------------------------------------------------- categories.menu_id
-- Deliberately NULLABLE: an unassigned category still belongs to the business
-- and must keep working, so it stays visible on the default menu.
ALTER TABLE categories ADD COLUMN IF NOT EXISTS menu_id UUID REFERENCES menus(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS categories_menu_position_idx
    ON categories (menu_id, position);

-- ------------------------------------------------------------- backfill
-- The order matters: default menu, default branch, the link between them, then
-- the categories. Every step is guarded so a second run changes nothing.

-- 1) One default menu per business that has none
INSERT INTO menus (business_id, name, slug, is_default, position)
SELECT b.id, 'Ana Menü', 'ana-menu', true, 0
FROM businesses b
WHERE NOT EXISTS (SELECT 1 FROM menus m WHERE m.business_id = b.id);

-- 2) One default branch per business that has none. Reusing the business slug
--    as the branch slug is intentional and safe: branch lookup runs BEFORE
--    business lookup and this branch points at the business' default menu, so
--    every existing <slug>.karecik.com URL keeps resolving to the same menu.
--
--    The contact columns are deliberately left NULL rather than copied from the
--    business. BuildPublicMenu treats a non-NULL branch phone/address/wifi as an
--    override of the business value, so copying them here would freeze whatever
--    the business happened to have at migration time: every later edit in
--    Settings would save correctly yet never reach the customer menu. NULL means
--    "this branch has nothing of its own", which is exactly right for a branch
--    the system invented on the owner's behalf.
INSERT INTO branches (business_id, name, slug, is_default, position)
SELECT b.id, 'Merkez', b.slug, true, 0
FROM businesses b
WHERE NOT EXISTS (SELECT 1 FROM branches br WHERE br.business_id = b.id);

-- 3) Link every default branch to its business' default menu
INSERT INTO branch_menus (branch_id, menu_id, is_default, position)
SELECT br.id, m.id, true, 0
FROM branches br
JOIN menus m ON m.business_id = br.business_id AND m.is_default
WHERE br.is_default
ON CONFLICT (branch_id, menu_id) DO NOTHING;

-- 4) Attach orphan categories to their business' default menu
UPDATE categories c
SET menu_id = m.id
FROM menus m
WHERE m.business_id = c.business_id AND m.is_default AND c.menu_id IS NULL;
