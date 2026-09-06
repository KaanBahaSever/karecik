-- Karecik — the customer menu header slogan and the splash entrance animation
--
-- This migration is ADDITIVE ONLY: it adds two columns to menus and one CHECK
-- constraint on a column it introduces itself, and nothing else. No table is
-- dropped, no existing constraint is changed, and no user content is deleted
-- or overwritten.
--
-- It is a NEW file and not an edit to 006 because 006 has ALREADY been applied
-- to the live database — schema_migrations there lists it, verified read-only
-- before this file was written, where the same query stopped at 005 one round
-- ago. database/migrate.go skips every file already recorded in
-- schema_migrations, so a column added to 006 would never appear on any
-- database that has run it: a silent no-op that would then break every scanMenu
-- at runtime against a column that is not there. New columns need a new file.
--
-- splash_entrance was added to THIS file rather than to a 008 for the same
-- reason read the other way round: 007 is NOT recorded on the live database —
-- schema_migrations there still stops at 006, verified read-only before this
-- edit — so it has not run anywhere real yet and extending it in place still
-- reaches every database. Exactly the judgement 006 wrote down when it grew
-- its own three menus columns.
--
-- It is also idempotent: every ADD COLUMN is IF NOT EXISTS guarded, exactly the
-- way 003, 004, 005 and 006 add theirs, and the one CHECK constraint is
-- attached through the same pg_constraint lookup those files use, because a
-- CHECK constraint cannot be added with IF NOT EXISTS. Re-running the whole
-- file on a partially migrated database is a no-op.
--
-- The reverse script lives in migrations/down/ and is NEVER executed
-- automatically — see the note at the top of that file.

-- ---------------------------------------------------------------- menus
-- slogan: the one-line tagline the customer menu header prints under the
--         business name, in the owner's own words ("Kahvenin en iyi hali").
--         It is theirs to write, so it starts out empty on every existing
--         menu — and an empty slogan renders nothing at all.
--
--         NOT NULL DEFAULT '' rather than a nullable column: the header treats
--         "no slogan" and "empty slogan" identically, so a pointer would add a
--         nil case for nothing. scanMenu reads a plain string, and the empty
--         string is a VALID value rather than a missing one, because clearing
--         the field is how a slogan is removed.
--
--         No CHECK on the length. The 120-character cap lives in
--         handlers/menu.go, next to the Turkish message that explains it; a
--         constraint here would turn an over-long slogan into a 500 where the
--         caller has already earned a 422. Compare menus_text_color_check in
--         006, which exists precisely because the handler enforces exactly the
--         same pattern and the two can therefore never disagree.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS slogan TEXT NOT NULL DEFAULT '';

-- splash_entrance: how the splash screen brings its logo and text IN. The way
--                  it goes OUT has been configurable since 004 — seven exit
--                  animations, five easings and a duration — but the entrance
--                  was hardcoded in SplashScreen.jsx and no business could
--                  change it.
--
--                  'fade' is today's behaviour to the millisecond: the
--                  karecikSplashIn keyframes, opacity 0 -> 1 with a small
--                  scale(0.94) -> 1, 320 ms, ease-out. 'none' draws the
--                  content with no animation property at all — not a
--                  zero-duration animation and not an opacity keyframe — the
--                  same off state logo_fade_in already uses in 006.
--
--                  DEFAULT 'fade' precisely so this migration changes NO
--                  existing menu's appearance: every splash on the live
--                  database already fades in, so every row that this ALTER
--                  backfills keeps exactly the look it has today. NOT NULL
--                  rather than nullable, so scanMenu reads a plain string and
--                  the frontend never has to reason about a missing value.
ALTER TABLE menus ADD COLUMN IF NOT EXISTS splash_entrance TEXT NOT NULL DEFAULT 'fade';

-- ---------------------------------------------------------- constraints
-- menus_splash_entrance_check keeps the column to the two entrances the
-- product actually draws. It follows the menus_<column>_check naming and the
-- pg_constraint guard 003, 004, 005 and 006 use, and it is attached separately
-- from the ADD COLUMN because CHECK constraints have no IF NOT EXISTS form.
--
-- The two ids are exactly utils.SplashEntrances, which handlers/menu.go
-- validates with utils.IsValidSplashEntrance before any UPDATE is built, so
-- the database and the API can never disagree: a payload the handler has
-- already blessed always satisfies this CHECK, and a rejected one earns a 422
-- rather than reaching here and turning into a 500.
--
-- Only the column this migration introduces is constrained; slogan carries no
-- CHECK, for the reason spelled out above it.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'menus_splash_entrance_check'
          AND conrelid = 'menus'::regclass
    ) THEN
        ALTER TABLE menus
            ADD CONSTRAINT menus_splash_entrance_check
            CHECK (splash_entrance IN ('fade','none'));
    END IF;
END $$;
