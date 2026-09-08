-- Karecik — password reset tokens
--
-- WHY THIS IS A TABLE WHEN SESSIONS ARE NOT
--
-- Sessions live in the API process's memory and a restart signs everyone out,
-- which is an accepted trade-off: the user is at the keyboard and can log in
-- again. A reset link is different. It is handed to an e-mail server and comes
-- back minutes or hours later, and a deploy in between is entirely normal. A
-- link that dies because the container restarted would be a link that fails
-- exactly when someone is already locked out, so these rows go on disk.
--
-- THE RAW TOKEN IS NEVER STORED HERE, for the same reason sessions never
-- stored theirs: token_hash holds the SHA-256 of the value in the e-mail. A
-- stolen database dump therefore yields nothing replayable — the server hashes
-- whatever arrives and compares. Storing the raw token would make every row in
-- this table a live password-reset credential for the account it points at.
--
-- SINGLE USE IS ENFORCED BY DELETION, not by a used_at flag. A consumed row is
-- gone, so "already used" and "never existed" are indistinguishable to the
-- caller — which is the answer we want to give anyway. A successful reset also
-- deletes every other outstanding token for that user, so an attacker who
-- requested a link in parallel cannot follow the real owner in.
--
-- Additive and idempotent, like every migration before it.

CREATE TABLE IF NOT EXISTS password_resets (
    -- SHA-256 (hex) of the token in the e-mailed link. PRIMARY KEY rather than
    -- a surrogate id: the hash IS the identity, and the lookup on every reset
    -- attempt is by hash, so this is the index that matters.
    token_hash TEXT PRIMARY KEY,

    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Deliberately short (see utils.PasswordResetTTL). A reset link is a
    -- bearer credential sitting in an inbox; the window it stays live in is
    -- the window a stolen inbox can take the account over in.
    expires_at TIMESTAMPTZ NOT NULL
);

-- Revoking every outstanding token of one user is the successful-reset path,
-- so it is a hot query rather than an occasional one.
CREATE INDEX IF NOT EXISTS password_resets_user_idx ON password_resets (user_id);

-- Housekeeping sweeps expired rows by this column.
CREATE INDEX IF NOT EXISTS password_resets_expires_idx ON password_resets (expires_at);
