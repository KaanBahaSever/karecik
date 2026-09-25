-- Karecik — reverse script for 012_menu_contact_in_footer.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 012 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/012_menu_contact_in_footer.down.sql
--
-- Revert the code with it: repository.menuColumns names contact_in_footer, so
-- every menu read of the current code fails on a database without it.
--
-- The rollback is lossless for every row 012 could have produced: a menu with
-- contact_display = 'hidden' and contact_in_footer = true is exactly the legacy
-- 'footer' mode, so it is written back as 'footer'. An 'inline' or 'list' menu
-- whose owner switched contact_in_footer on afterwards loses only that switch —
-- under 011 those two modes already carried the footer list.
--
-- 012 leaves 011's constraint in place, but a database migrated by an earlier
-- build of 012 carries a narrower one that refuses 'footer'. Whichever is
-- there is dropped before the rewrite — the constraint has to ACCEPT 'footer'
-- by the time the UPDATE writes it — and 011's definition, which admits all
-- four modes, goes back on once the rewrite is done.
--
-- Safe to run twice: the constraint drop is IF EXISTS guarded, the UPDATE is
-- skipped when the column is already gone, and the column drop is IF EXISTS.

ALTER TABLE menus DROP CONSTRAINT IF EXISTS menus_contact_display_check;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'menus' AND column_name = 'contact_in_footer'
    ) THEN
        UPDATE menus
           SET contact_display = 'footer'
         WHERE contact_display = 'hidden' AND contact_in_footer = true;
    END IF;
END
$$;

ALTER TABLE menus DROP COLUMN IF EXISTS contact_in_footer;

ALTER TABLE menus
    ADD CONSTRAINT menus_contact_display_check
    CHECK (contact_display IN ('inline', 'list', 'footer', 'hidden'));

DELETE FROM schema_migrations WHERE version = '012_menu_contact_in_footer.sql';
