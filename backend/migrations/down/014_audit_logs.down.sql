-- Karecik — reverse script for 014_audit_logs.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/014_audit_logs.down.sql
--
-- DESTRUCTIVE: the whole audit trail is lost. Take a dump first.
--
-- Revert the code with it: every audited write inserts into this table inside
-- its own transaction, so on a database without it those writes FAIL rather
-- than merely going unrecorded.
--
-- Safe to run twice: every drop is IF EXISTS guarded.

DROP INDEX IF EXISTS audit_logs_business_created_idx;
DROP TABLE IF EXISTS audit_logs;

DELETE FROM schema_migrations WHERE version = '014_audit_logs.sql';
