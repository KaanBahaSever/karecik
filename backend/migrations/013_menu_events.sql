-- Karecik — visitor analytics of the customer menu
--
-- One row per thing a customer did on a published menu: opened it
-- ('menu_view'), opened a category ('category_view') or opened a product's
-- detail dialog ('product_view'). The customer page reports them to the public
-- POST /api/public/events endpoint (handlers/analytics.go), and the dashboard
-- reads them back aggregated through GET /api/analytics/summary and row by row
-- through GET /api/analytics/events.
--
-- PERSONAL DATA. ip and port identify a subscriber (under carrier-grade NAT the
-- address alone does not — the port is what makes it attributable), and the
-- user agent narrows it further. Under KVKK these rows are therefore kept only
-- as long as they are useful: cmd/api purges everything older than
-- ANALYTICS_RETENTION_DAYS (default 90, 0 keeps them forever) at start-up and
-- every 24 hours — see repository.PurgeMenuEvents. created_at_idx below is what
-- keeps that purge from scanning the table.
--
-- Additive and idempotent: CREATE ... IF NOT EXISTS throughout, so re-running
-- the file is harmless. The reverse script lives in migrations/down/ and is
-- NEVER executed automatically.

CREATE TABLE IF NOT EXISTS menu_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Both cascade: the events of a tenant, or of one of its menus, mean
    -- nothing once it is gone, and a KVKK erasure of a tenant must not leave
    -- visitor addresses behind.
    business_id  UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    menu_id      UUID NOT NULL REFERENCES menus(id) ON DELETE CASCADE,

    type         TEXT NOT NULL
                 CHECK (type IN ('menu_view', 'category_view', 'product_view')),

    -- NO foreign keys, deliberately. The handler proves both ids belong to the
    -- menu before it inserts, so a row never starts out pointing elsewhere; what
    -- an FK would add is behaviour on DELETE, and both options are wrong here:
    --
    --   * ON DELETE CASCADE would erase the traffic history of a product the
    --     moment the owner removes it — the totals would change retroactively.
    --   * ON DELETE SET NULL would make every category or product delete UPDATE
    --     an unbounded number of event rows inside its own transaction. Those
    --     deletes lock their rows in a documented order (repository.DeleteMenu,
    --     repository.DeleteCategory) that the lock-cycle suites pin down; a
    --     cascade into this table would add row locks nobody ordered, and a
    --     popular product would make its own deletion slow.
    --
    -- An FK would also make every visitor event take a KEY SHARE lock on a
    -- category and a product row, which a category delete's FOR UPDATE would
    -- then wait behind. So the ids are plain columns: an event of a deleted
    -- category or product keeps its id, the readers LEFT JOIN for the name and
    -- report it as null ("deleted"), and the retention purge removes it in time.
    category_id  UUID,
    product_id   UUID,

    -- visitor_id is what the customer page sends: a random id it keeps in the
    -- browser, at most 64 characters of [A-Za-z0-9_-]. visitor_key is what the
    -- unique-visitor counts group on, computed once on insert: "v:" + the
    -- visitor_id when there is one, otherwise "h:" + a hash of the address and
    -- the user agent (handlers.VisitorKey). Precomputing it keeps COUNT(DISTINCT)
    -- a plain column read.
    visitor_id   TEXT CHECK (visitor_id IS NULL OR visitor_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    visitor_key  TEXT NOT NULL,

    -- The address as middleware.ClientAddrOf resolved it — the same resolver
    -- the request log line uses. NULL when nothing could be established (the
    -- resolver's "-"); ip_source says how much the address is worth
    -- ("cloudflare", "cloudflare-unverified", "edge", "peer", ...).
    ip           TEXT,
    port         INTEGER CHECK (port IS NULL OR port BETWEEN 0 AND 65535),
    ip_source    TEXT NOT NULL DEFAULT 'unknown',

    -- Truncated to 300 characters by the handler; '' when absent.
    user_agent   TEXT NOT NULL DEFAULT '',

    -- The language the page was showing, one of utils.Languages, or NULL.
    language     TEXT,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The dashboard reads a tenant's events by time range (summary, event list)...
CREATE INDEX IF NOT EXISTS menu_events_business_created_idx
    ON menu_events (business_id, created_at);

-- ...or one menu's events by time range (the menu filter of both).
CREATE INDEX IF NOT EXISTS menu_events_menu_created_idx
    ON menu_events (menu_id, created_at);

-- The retention purge deletes by age alone, across every tenant.
CREATE INDEX IF NOT EXISTS menu_events_created_idx
    ON menu_events (created_at);
