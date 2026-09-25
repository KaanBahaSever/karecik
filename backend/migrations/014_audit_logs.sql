-- Karecik — audit trail of the owner's own changes
--
-- One row per successful administrative write: a product created, a price
-- edited, a menu's phone number or logo replaced, the account's address
-- renamed, the password changed, a file uploaded. The dashboard lists them
-- through GET /api/audit-logs (handlers/audit.go).
--
-- WHO AND WHERE. user_id and user_email come from the session that made the
-- change; user_email is a SNAPSHOT taken when the row is written, so the trail
-- still reads correctly after the account's address changes. ip, port and
-- ip_source come from middleware.ClientAddrOf — the resolver the request log
-- line uses — so an audit row and the log line of the same request always name
-- the same address and say equally honestly how well it is established.
--
-- WHAT. changes is {"<field>": {"old": ..., "new": ...}} holding only the
-- fields that really changed (translations are spelled per language and field,
-- "translations.en.name"). Two values are never stored as they are: a Wi-Fi
-- password is written as "••••", and a password change records that it
-- happened and nothing else — neither the old nor the new password, in any
-- form. entity_label is a snapshot of the record's name, so a deleted product
-- is still identifiable in the trail.
--
-- WHEN IT IS WRITTEN. Inside the transaction of the write it describes, as the
-- last statement (repository.WriteHook): the change and its record commit or
-- roll back together, and a write that repository.RetryOnConflict runs again
-- leaves exactly one row, because the aborted run's row was rolled back with
-- it. The only exception is an upload, which writes no row of its own to share
-- a transaction with (handlers.Upload records it best-effort afterwards).
--
-- Additive and idempotent: CREATE ... IF NOT EXISTS throughout. The reverse
-- script lives in migrations/down/ and is NEVER executed automatically.

CREATE TABLE IF NOT EXISTS audit_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    business_id  UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,

    -- SET NULL rather than CASCADE: the trail outlives the user row it names,
    -- and user_email still says who it was.
    user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    user_email   TEXT NOT NULL DEFAULT '',

    -- Both lists are exactly the constants of internal/audit. A value outside
    -- them is a programming error, and the CHECK turns it into a failed write
    -- rather than a row the dashboard cannot label.
    action       TEXT NOT NULL CHECK (action IN (
                     'product.create', 'product.update', 'product.delete',
                     'product.price', 'product.bulk_price', 'product.reorder',
                     'category.create', 'category.update', 'category.delete',
                     'category.reorder',
                     'menu.create', 'menu.update', 'menu.delete',
                     'business.update', 'account.password_change',
                     'upload.create')),
    entity_type  TEXT NOT NULL CHECK (entity_type IN (
                     'product', 'category', 'menu', 'business', 'account', 'upload')),

    -- TEXT, not UUID: an upload is identified by its file name. NULL for a
    -- write that spans several records (a reorder, a bulk price update).
    entity_id    TEXT,
    entity_label TEXT NOT NULL DEFAULT '',

    changes      JSONB NOT NULL DEFAULT '{}'::jsonb
                 CHECK (jsonb_typeof(changes) = 'object'),

    ip           TEXT,
    port         INTEGER CHECK (port IS NULL OR port BETWEEN 0 AND 65535),
    ip_source    TEXT NOT NULL DEFAULT 'unknown',

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The only read: one tenant's trail, newest first.
CREATE INDEX IF NOT EXISTS audit_logs_business_created_idx
    ON audit_logs (business_id, created_at DESC);
