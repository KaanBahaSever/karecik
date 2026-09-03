-- Karecik — reverse script for 003_branches_badges_branding.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 003 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/003_branches_badges_branding.down.sql
--
-- IT IS DESTRUCTIVE: every branch, menu, branch price, product badge and
-- calorie value is lost, together with the branding columns. Take a dump
-- first.
--
-- Statements are ordered by reverse dependency and are all IF EXISTS guarded,
-- so the script is safe to run twice.

-- ------------------------------------------------ branch_product_prices
DROP TABLE IF EXISTS branch_product_prices;

-- --------------------------------------------------------- branch_menus
DROP TABLE IF EXISTS branch_menus;

-- ------------------------------------------------------------- branches
DROP TABLE IF EXISTS branches;

-- --------------------------------------------------- categories.menu_id
-- Dropped before the menus table because of the foreign key. The column drop
-- takes categories_menu_position_idx with it.
ALTER TABLE categories DROP COLUMN IF EXISTS menu_id;

-- ---------------------------------------------------------------- menus
DROP TABLE IF EXISTS menus;

-- ------------------------------------------------------------- products
-- Dropping the columns also drops products_calories_check.
ALTER TABLE products DROP COLUMN IF EXISTS badges;
ALTER TABLE products DROP COLUMN IF EXISTS calories;

-- ----------------------------------------------------------- businesses
-- Dropping the columns also drops the four businesses_*_check constraints.
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_exit_duration;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_exit_animation;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_headline;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_logo_url;
ALTER TABLE businesses DROP COLUMN IF EXISTS wifi_ssid;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_overlay_opacity;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_image_url;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_color;
ALTER TABLE businesses DROP COLUMN IF EXISTS background_type;

-- ---------------------------------------------------------- bookkeeping
DELETE FROM schema_migrations WHERE version = '003_branches_badges_branding.sql';
