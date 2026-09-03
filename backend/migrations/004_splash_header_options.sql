-- Karecik — splash easing, splash/header display modes
--
-- This migration is ADDITIVE: it adds three businesses columns and widens one
-- CHECK constraint. No table is dropped, no row is deleted and no user content
-- is overwritten.
--
-- The single DROP statement below is the documented exception. A CHECK
-- constraint cannot be widened in place, so
-- businesses_splash_exit_animation_check is dropped and immediately recreated
-- with three more allowed values. It touches no row data: every value the old
-- constraint accepted is still accepted by the new one.
--
-- The file is idempotent: every statement is guarded, so re-running it on a
-- partially migrated database is a no-op. CHECK constraints cannot be added
-- with IF NOT EXISTS, so they are attached inside guarded DO blocks, exactly
-- the way 003 does it.
--
-- categories.menu_id and categories_menu_position_idx already exist from 003
-- and are deliberately NOT re-created here.
--
-- The reverse script lives in migrations/down/ and is NEVER executed
-- automatically — see the note at the top of that file.

-- --------------------------------------------- splash exit animations
-- 003 allowed four values; slide-left, zoom-in and zoom-out are new. Dropping
-- first also makes the pair idempotent — a re-run simply recreates the same
-- widened constraint.
ALTER TABLE businesses DROP CONSTRAINT IF EXISTS businesses_splash_exit_animation_check;

ALTER TABLE businesses
    ADD CONSTRAINT businesses_splash_exit_animation_check
    CHECK (splash_exit_animation IN
        ('fade','slide-up','slide-down','slide-left','slide-right','zoom-in','zoom-out'));

-- ----------------------------------------------------------- businesses
-- splash_exit_easing: the CSS timing function of the splash exit animation.
-- splash_display:     what the splash screen shows — logo, text or both.
-- header_display:     what the customer menu header shows — logo, name or both.
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_exit_easing TEXT NOT NULL DEFAULT 'ease-in';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS splash_display TEXT NOT NULL DEFAULT 'both';
ALTER TABLE businesses ADD COLUMN IF NOT EXISTS header_display TEXT NOT NULL DEFAULT 'both';

DO $$
BEGIN
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
