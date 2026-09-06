-- Karecik — reverse script for 007_menu_slogan.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 007 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/007_menu_slogan.down.sql
--
-- IT IS DESTRUCTIVE: every slogan a business typed is lost, together with each
-- menu's splash entrance choice, and re-applying 007 afterwards brings the
-- columns back empty and back on 'fade' on every menu. Take a dump first.
--
-- Every column drop is IF EXISTS guarded and the delete matches nothing on a
-- second pass, so the script is safe to run twice.
--
-- menus_splash_entrance_check needs no separate DROP CONSTRAINT: dropping the
-- column it constrains takes it with it, and dropping it first would leave a
-- half-rolled-back database if the column drop then failed. This is the same
-- reasoning 006's reverse script writes down for menus_text_color_check.

-- ---------------------------------------------------------------- menus
ALTER TABLE menus DROP COLUMN IF EXISTS splash_entrance;
ALTER TABLE menus DROP COLUMN IF EXISTS slogan;

-- ---------------------------------------------------------- bookkeeping
DELETE FROM schema_migrations WHERE version = '007_menu_slogan.sql';
