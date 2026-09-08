-- Karecik — server-side sessions, replacing the stateless JWT
--
-- ┌───────────────────────────────────────────────────────────────────────────┐
-- │ THIS TABLE IS NO LONGER READ OR WRITTEN.                                  │
-- │                                                                           │
-- │ Sessions moved into the API process's memory (backend/internal/session).  │
-- │ Nothing in the application touches these rows any more; the store is the  │
-- │ only place a live session exists.                                         │
-- │                                                                           │
-- │ It is left in place rather than dropped because dropping it destroys      │
-- │ data to buy nothing: an empty table costs no query time, and keeping it   │
-- │ means going back to database-backed sessions is a code change rather      │
-- │ than a code change plus a migration. Applied migrations are also never    │
-- │ re-run, so editing this file out would not remove the table from any      │
-- │ database that already has it — only from fresh ones, which would make     │
-- │ the two shapes differ for no reason.                                      │
-- │                                                                           │
-- │ Everything below describes what the table WAS for. It is kept as the      │
-- │ record of that design, not as a description of how the app works today.   │
-- └───────────────────────────────────────────────────────────────────────────┘
--
-- A JWT could not be revoked. Signing one out meant deleting it from the
-- browser's localStorage and hoping: a copy taken before that kept working
-- until it expired, and localStorage is readable by any script that reaches the
-- page. A session row can be deleted, which is what makes "log out everywhere"
-- and "invalidate other sessions after a password change" real rather than
-- decorative, and an HttpOnly cookie is not readable from JavaScript at all.
--
-- THE COOKIE VALUE IS NEVER STORED HERE. token_hash holds the SHA-256 of it.
-- A stolen database dump therefore yields no usable session: the hash cannot be
-- replayed, because the server hashes whatever arrives and compares. Storing the
-- raw token would make every row in this table a live credential.
--
-- Additive and idempotent, like every migration before it.

CREATE TABLE IF NOT EXISTS sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- SHA-256 (hex) of the value in the cookie. UNIQUE both to enforce one row
    -- per token and to give the lookup on every authenticated request an index.
    token_hash TEXT NOT NULL UNIQUE,

    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,

    -- Recorded so an owner can be shown where a session was opened from. They
    -- are nullable on purpose: a proxy may strip either, and a missing value
    -- must never stop someone logging in.
    ip_address TEXT,
    user_agent TEXT
);

-- Deleting every session of one user is the password-change path, so it is a
-- hot query and not an occasional one.
CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions (user_id);

-- Housekeeping sweeps expired rows by this column.
CREATE INDEX IF NOT EXISTS sessions_expires_idx ON sessions (expires_at);
