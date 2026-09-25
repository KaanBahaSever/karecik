-- Karecik — reverse script for 013_menu_events.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/013_menu_events.down.sql
--
-- DESTRUCTIVE: every recorded visitor event is lost. Take a dump first if the
-- history matters.
--
-- Revert the code with it: the public events endpoint, the analytics endpoints
-- and the retention sweep in cmd/api all read or write this table.
--
-- Safe to run twice: every drop is IF EXISTS guarded.

DROP INDEX IF EXISTS menu_events_created_idx;
DROP INDEX IF EXISTS menu_events_menu_created_idx;
DROP INDEX IF EXISTS menu_events_business_created_idx;
DROP TABLE IF EXISTS menu_events;

DELETE FROM schema_migrations WHERE version = '013_menu_events.sql';
