-- Reverse of 008_sessions.sql
--
-- NOT RUN AUTOMATICALLY. migrations/embed.go matches *.sql at the top level
-- only, so nothing in down/ ever reaches the runner. Apply it by hand:
--
--   psql -U postgres -h localhost -w -d karecik -f migrations/down/008_sessions.down.sql
--
-- DESTRUCTIVE: dropping the table signs out every user everywhere, because the
-- session rows ARE the sessions. There is nothing to preserve — the cookies
-- become meaningless the moment the rows are gone — but nobody stays logged in.
--
-- Reverting this migration alone also leaves the application unable to
-- authenticate anyone: the code that reads these rows does not fall back to the
-- old JWT path, which 008 replaced. Revert the code with it.

DROP INDEX IF EXISTS sessions_expires_idx;
DROP INDEX IF EXISTS sessions_user_idx;
DROP TABLE IF EXISTS sessions;

DELETE FROM schema_migrations WHERE version = '008_sessions.sql';
