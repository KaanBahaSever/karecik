-- Karecik — reverse script for 011_menu_contact_links.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 011 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/011_menu_contact_links.down.sql
--
-- IT IS DESTRUCTIVE: every custom link a business entered is lost, together
-- with each menu's contact display choice, and re-applying 011 afterwards
-- brings the columns back empty and on 'inline' on every menu. Take a dump
-- first.
--
-- Revert the code with it. repository.menuColumns names both columns, so every
-- menu read of the current code fails on a database without them.
--
-- menus_contact_display_check and menus_links_check need no separate DROP
-- CONSTRAINT: dropping the column a constraint checks takes the constraint
-- with it, and dropping them first would leave a half-rolled-back database if
-- a column drop then failed. This is the same reasoning the reverse scripts of
-- 006 and 007 write down.
--
-- Every column drop is IF EXISTS guarded and the delete matches nothing on a
-- second pass, so the script is safe to run twice.

-- ---------------------------------------------------------------- menus
ALTER TABLE menus DROP COLUMN IF EXISTS links;
ALTER TABLE menus DROP COLUMN IF EXISTS contact_display;

-- ---------------------------------------------------------- bookkeeping
DELETE FROM schema_migrations WHERE version = '011_menu_contact_links.sql';
