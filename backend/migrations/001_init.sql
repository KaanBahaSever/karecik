-- Karecik — initial schema
-- Requires PostgreSQL 13+ (gen_random_uuid() ships in core, no extension needed).
--
-- NOTE: default values such as the VAT notice are customer-facing copy and are
-- therefore stored in Turkish on purpose.

-- Trigger function that refreshes the updated_at column on every UPDATE
CREATE OR REPLACE FUNCTION karecik_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------- users
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    business_name TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'owner',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Email uniqueness must be case-insensitive
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON users (lower(email));

DROP TRIGGER IF EXISTS users_set_updated_at ON users;
CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

-- ----------------------------------------------------------- businesses
-- One user = one business (tenant). The subdomain is resolved from 'slug'.
CREATE TABLE IF NOT EXISTS businesses (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    slug             TEXT NOT NULL UNIQUE,

    logo_url         TEXT,
    cover_url        TEXT,

    -- Currency
    currency         TEXT NOT NULL DEFAULT 'TRY'
                     CHECK (currency IN ('TRY','USD','EUR','GBP','AZN','RUB','SAR','AED')),

    -- Appearance
    theme            TEXT NOT NULL DEFAULT 'modern-light',
    font_family      TEXT NOT NULL DEFAULT 'inter',
    primary_color    TEXT NOT NULL DEFAULT '#1d4ed8',

    -- Languages
    default_language TEXT NOT NULL DEFAULT 'tr',
    languages        JSONB NOT NULL DEFAULT '["tr"]'::jsonb,

    -- Splash screen — shown only in the customer menu
    splash_enabled   BOOLEAN NOT NULL DEFAULT true,
    splash_duration  INTEGER NOT NULL DEFAULT 1200 CHECK (splash_duration BETWEEN 300 AND 5000),
    splash_bg_color  TEXT NOT NULL DEFAULT '#0f172a',
    splash_text      TEXT NOT NULL DEFAULT '',

    -- Legal notices in the footer
    show_vat_note    BOOLEAN NOT NULL DEFAULT true,
    vat_note_text    TEXT NOT NULL DEFAULT 'Fiyatlarımıza KDV dahildir.',
    show_price_date  BOOLEAN NOT NULL DEFAULT true,
    price_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Contact details
    phone            TEXT,
    address          TEXT,
    instagram        TEXT,
    wifi_password    TEXT,

    is_active        BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS businesses_slug_idx ON businesses (slug);

DROP TRIGGER IF EXISTS businesses_set_updated_at ON businesses;
CREATE TRIGGER businesses_set_updated_at BEFORE UPDATE ON businesses
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

-- ----------------------------------------------------------- categories
-- translations: {"tr": {"name": "...", "description": "..."}, "en": {...}}
CREATE TABLE IF NOT EXISTS categories (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id  UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    translations JSONB NOT NULL DEFAULT '{}'::jsonb,
    icon         TEXT,
    image_url    TEXT,
    position     INTEGER NOT NULL DEFAULT 0,
    is_active    BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS categories_business_position_idx
    ON categories (business_id, position);

DROP TRIGGER IF EXISTS categories_set_updated_at ON categories;
CREATE TRIGGER categories_set_updated_at BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

-- ------------------------------------------------------------- products
-- translations: {"tr": {"name": "...", "description": "...", "ingredients": "..."}}
-- allergens: ["gluten", "sut", ...]
CREATE TABLE IF NOT EXISTS products (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id   UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    category_id   UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    translations  JSONB NOT NULL DEFAULT '{}'::jsonb,
    price         NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (price >= 0),
    compare_price NUMERIC(12,2) CHECK (compare_price IS NULL OR compare_price >= 0),
    image_url     TEXT,
    allergens     JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active     BOOLEAN NOT NULL DEFAULT true,
    is_featured   BOOLEAN NOT NULL DEFAULT false,
    position      INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS products_category_position_idx
    ON products (category_id, position);
CREATE INDEX IF NOT EXISTS products_business_idx ON products (business_id);

DROP TRIGGER IF EXISTS products_set_updated_at ON products;
CREATE TRIGGER products_set_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION karecik_set_updated_at();

-- --------------------------------------------------- price_update_logs
-- Audit trail of the bulk price updates
CREATE TABLE IF NOT EXISTS price_update_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    percentage  NUMERIC(6,2) NOT NULL,
    rounding    TEXT NOT NULL DEFAULT 'none',
    affected    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS price_update_logs_business_idx
    ON price_update_logs (business_id, created_at DESC);
