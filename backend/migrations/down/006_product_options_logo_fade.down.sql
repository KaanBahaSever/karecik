-- Karecik — reverse script for 006_product_options_logo_fade.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 006 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/006_product_options_logo_fade.down.sql
--
-- IT IS DESTRUCTIVE: every option group and every option item a business
-- entered is lost, together with each menu's logo fade-in choice, its text
-- colour and its uploaded "Yerli Üretim" badge. Take a dump first.
--
-- Every column drop is IF EXISTS guarded and the delete matches nothing on a
-- second pass, so the script is safe to run twice.
--
-- menus_text_color_check needs no separate DROP CONSTRAINT: dropping the
-- column it constrains takes it with it, and dropping it first would leave a
-- half-rolled-back database if the column drop then failed.

-- ------------------------------------------------------------- products
ALTER TABLE products DROP COLUMN IF EXISTS options;

-- ---------------------------------------------------------------- menus
ALTER TABLE menus DROP COLUMN IF EXISTS yerli_uretim_logo_url;
ALTER TABLE menus DROP COLUMN IF EXISTS show_yerli_uretim;
ALTER TABLE menus DROP COLUMN IF EXISTS text_color;
ALTER TABLE menus DROP COLUMN IF EXISTS logo_fade_in;

-- ---------------------------------------------------------- bookkeeping
DELETE FROM schema_migrations WHERE version = '006_product_options_logo_fade.sql';
