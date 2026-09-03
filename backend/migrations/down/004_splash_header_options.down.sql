-- Karecik — reverse script for 004_splash_header_options.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND when you really
-- want to roll 004 back:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/004_splash_header_options.down.sql
--
-- IT IS DESTRUCTIVE: every splash easing, splash display and header display
-- choice is lost, and any business using one of the three exit animations 004
-- added falls back to 'fade'. Take a dump first.
--
-- Statements are IF EXISTS guarded and the coercion matches nothing on a
-- second pass, so the script is safe to run twice.

-- --------------------------------------------- splash exit animations
-- Restore the four-value 003 constraint. Rows carrying slide-left, zoom-in or
-- zoom-out would violate it, so they are coerced back to the default 'fade'
-- BEFORE the constraint is attached — otherwise ADD CONSTRAINT aborts the
-- whole script on any database that ever used a new value.
UPDATE businesses
SET splash_exit_animation = 'fade'
WHERE splash_exit_animation NOT IN ('slide-up','slide-down','slide-right','fade');

ALTER TABLE businesses DROP CONSTRAINT IF EXISTS businesses_splash_exit_animation_check;

ALTER TABLE businesses
    ADD CONSTRAINT businesses_splash_exit_animation_check
    CHECK (splash_exit_animation IN ('slide-up','slide-down','slide-right','fade'));

-- ----------------------------------------------------------- businesses
-- Dropping the columns also drops the three businesses_*_check constraints
-- that 004 attached to them.
ALTER TABLE businesses DROP COLUMN IF EXISTS header_display;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_display;
ALTER TABLE businesses DROP COLUMN IF EXISTS splash_exit_easing;

-- ---------------------------------------------------------- bookkeeping
DELETE FROM schema_migrations WHERE version = '004_splash_header_options.sql';
