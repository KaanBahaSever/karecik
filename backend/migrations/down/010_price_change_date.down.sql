-- Karecik — reverse script for 010_price_change_date.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 010 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/010_price_change_date.down.sql
--
-- NO DATA IS LOST: 010 added a function and a trigger and wrote no row, so
-- every menu keeps the price_updated_at it has. What stops is the date moving
-- at all. The Go side stopped moving it itself when the trigger took over —
-- repository.TouchPriceUpdatedAt is gone — so reverting this migration alone
-- leaves the footer date frozen on every menu. Revert the code with it.
--
-- The trigger goes first: PostgreSQL refuses to drop a function a trigger
-- still depends on. Both drops are IF EXISTS guarded and the delete matches
-- nothing on a second pass, so the script is safe to run twice.

-- ------------------------------------------------------------- products
DROP TRIGGER IF EXISTS products_touch_menu_price_date ON products;
DROP FUNCTION IF EXISTS karecik_touch_menu_price_date();

-- ---------------------------------------------------------- bookkeeping
DELETE FROM schema_migrations WHERE version = '010_price_change_date.sql';
