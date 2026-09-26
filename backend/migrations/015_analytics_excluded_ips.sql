-- Karecik — addresses whose visits the analytics never store
--
-- The owner opening their own menu from the shop's Wi-Fi, a waiter showing it
-- to a table from the counter: counted as visitors, they are the loudest
-- traffic a small menu has. Each business keeps a list of addresses and CIDR
-- ranges here (panel -> Analitik, GET/POST/DELETE /api/analytics/excluded-ips),
-- and POST /api/public/events answers a view from any of them with its usual
-- 204 and stores nothing — before the repeat rule and the daily cap look at
-- it, so such a view spends none of the business's allowance
-- (handlers.TrackEvent, package ipexclude).
--
-- Two more mechanisms do the same without a row here: the platform-wide
-- ANALYTICS_EXCLUDED_IPS list (the operator's, never shown to any tenant) and a
-- per-browser opt-out cookie. See docs/API.md.
--
-- The same file extends the audit vocabulary of migration 014 with the two
-- actions that change this list, and adds karecik_try_inet(), which the
-- history delete and the match count need to compare menu_events.ip — a TEXT
-- column — with a range.
--
-- Additive and idempotent: CREATE ... IF NOT EXISTS, CREATE OR REPLACE, and
-- CHECK constraints dropped and re-added under fixed names, so re-running the
-- file is harmless. The reverse script lives in migrations/down/ and is NEVER
-- executed automatically.

CREATE TABLE IF NOT EXISTS analytics_excluded_ips (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- The list is the tenant's own and dies with it.
    business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,

    -- Always normalised by the application before it gets here
    -- (ipexclude.Parse): host bits masked — which the cidr type insists on
    -- anyway — IPv4-mapped IPv6 unmapped, never broader than /16 for IPv4 or
    -- /48 for IPv6. A single address is stored as a /32 or a /128.
    cidr        CIDR NOT NULL,

    -- A name the owner gives the entry ("Kasa", "Ev"); '' when none.
    label       TEXT NOT NULL DEFAULT '' CHECK (char_length(label) <= 60),

    -- Who added it. SET NULL rather than CASCADE: the entry keeps working after
    -- the user row it names is gone, and the audit trail still says who it was.
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One entry per range per business. The handler also refuses a range an
    -- existing entry already covers; this is the part of that rule a race
    -- between two identical requests cannot get past.
    CONSTRAINT analytics_excluded_ips_business_cidr_key UNIQUE (business_id, cidr)
);

-- Every read is one business's list in the order it was built — the panel's
-- listing and the per-business cache of the events endpoint alike.
CREATE INDEX IF NOT EXISTS analytics_excluded_ips_business_idx
    ON analytics_excluded_ips (business_id, created_at);

-- karecik_try_inet reads a menu_events.ip value as an address, or NULL when it
-- is not one.
--
-- menu_events.ip is TEXT. The events endpoint only ever writes a normalised
-- address or NULL there, but nothing in the schema says so, and a row holding
-- 'unknown', '-' or '' — written by hand, by an older build, by a restore —
-- must not turn "delete my own visits" into a 500: a plain ip::inet fails the
-- whole statement on the first such row.
--
-- What counts as an address: a bare address the inet type reads — no
-- "/prefix", no spaces, no zone. NULL in, NULL out. Both bodies below answer
-- exactly that; they differ only in speed.
--
-- WHY TWO BODIES. The function runs once per stored event of the business —
-- up to 90 days at the daily cap, close to a million rows — for every
-- match-count the add dialog asks and every history delete, the latter while
-- the business's exclusion lock is held. The obvious guarded cast, a PL/pgSQL
-- EXCEPTION block, opens a subtransaction on every row it enters and is not
-- PARALLEL SAFE: on a busy tenant that made the count take seconds where a
-- plain cast takes a tenth of one.
--
--   PostgreSQL 16+   pg_input_is_valid() asks the inet type whether it would
--                    read the value without raising anything, so the body is
--                    one CASE expression in LANGUAGE sql. Declared STABLE
--                    (pg_input_is_valid is) and not STRICT (a CASE is not),
--                    the planner inlines it into the calling statement: no
--                    per-row function call and no subtransaction, and the scan
--                    may run in parallel. tests/analytics_exclusion_test.go
--                    reads the plan to hold it to that.
--                    The cast sits OUTSIDE the CASE on purpose. Inlined, a
--                    constant argument — a literal, or a parameter the
--                    planner substitutes into a custom plan — would have a
--                    `'unknown'::inet` inside the THEN branch folded at plan
--                    time, raising before the WHEN is ever consulted; a CASE
--                    that yields the text leaves the cast nothing constant to
--                    fold.
--   older servers    README supports PostgreSQL 13+. The PL/pgSQL body stays,
--                    with a fast path in front of the guard: a dotted quad of
--                    0-255 octets without leading zeros is always readable, so
--                    the IPv4 addresses the events endpoint writes skip the
--                    subtransaction; anything else made only of address
--                    characters still goes through the guarded cast, which
--                    catches the rest ('1.2.3.999', ':::'). IMMUTABLE, since
--                    the result depends on the argument alone; not PARALLEL
--                    SAFE, because a parallel worker may not start the
--                    subtransaction.
--
-- The version is read when this file runs. A server upgraded to 16+ later
-- keeps the older body — still correct, only slower — until this definition
-- is run again by hand; it is idempotent.
DO $migration$
BEGIN
    IF current_setting('server_version_num')::int >= 160000 THEN
        -- EXECUTE, so an older server never even parses a call to a function
        -- it does not have.
        EXECUTE $create$
            CREATE OR REPLACE FUNCTION karecik_try_inet(value TEXT) RETURNS INET
                LANGUAGE sql STABLE PARALLEL SAFE
            AS $body$
                SELECT (CASE
                    WHEN strpos(value, '/') = 0 AND pg_input_is_valid(value, 'inet')
                        THEN value
                END)::inet
            $body$
        $create$;
    ELSE
        EXECUTE $create$
            CREATE OR REPLACE FUNCTION karecik_try_inet(value TEXT) RETURNS INET
                LANGUAGE plpgsql IMMUTABLE STRICT
            AS $body$
            BEGIN
                IF value ~ '^(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])([.](25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])){3}$' THEN
                    RETURN value::inet;
                END IF;
                IF value !~ '^[0-9A-Fa-f:.]{2,45}$' THEN
                    RETURN NULL;
                END IF;
                BEGIN
                    RETURN value::inet;
                EXCEPTION WHEN data_exception THEN
                    RETURN NULL;
                END;
            END;
            $body$
        $create$;
    END IF;
END
$migration$;

-- The audit vocabulary. Migration 014 declared both lists as column CHECKs,
-- whose names PostgreSQL chose; every single-column CHECK on action or
-- entity_type is dropped here whatever it is called, and the lists come back
-- under fixed names, so this block leaves exactly one constraint per column
-- however often it runs. Both lists are exactly the constants of
-- internal/audit (tests/audit_log_test.go writes each value to prove it).
DO $$
DECLARE
    stale TEXT;
BEGIN
    FOR stale IN
        SELECT c.conname
        FROM pg_constraint c
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
        WHERE c.conrelid = 'audit_logs'::regclass
          AND c.contype = 'c'
          AND array_length(c.conkey, 1) = 1
          AND a.attname IN ('action', 'entity_type')
    LOOP
        EXECUTE format('ALTER TABLE audit_logs DROP CONSTRAINT %I', stale);
    END LOOP;
END
$$;

ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_action_check CHECK (action IN (
    'product.create', 'product.update', 'product.delete',
    'product.price', 'product.bulk_price', 'product.reorder',
    'category.create', 'category.update', 'category.delete',
    'category.reorder',
    'menu.create', 'menu.update', 'menu.delete',
    'business.update', 'account.password_change',
    'upload.create',
    'analytics.exclude_ip.add', 'analytics.exclude_ip.remove'));

ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_entity_type_check CHECK (entity_type IN (
    'product', 'category', 'menu', 'business', 'account', 'upload',
    'analytics_exclusion'));
