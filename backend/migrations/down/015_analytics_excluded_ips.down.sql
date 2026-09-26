-- Karecik — reverse script for 015_analytics_excluded_ips.sql
--
-- THIS FILE IS NOT RUN AUTOMATICALLY. migrations/embed.go embeds `*.sql`,
-- which matches the top level of migrations/ only, so nothing inside down/ is
-- ever picked up by the migration runner. Apply it BY HAND:
--
--     psql -U postgres -d karecik -v ON_ERROR_STOP=1 \
--          -f backend/migrations/down/015_analytics_excluded_ips.down.sql
--
-- DESTRUCTIVE: every business's exclusion list is lost, and so are the audit
-- rows that recorded changes to it — the restored CHECK lists of migration 014
-- do not admit them, so they have to go before the lists can come back. The
-- visits those lists kept out were never stored and are not restored either.
-- Take a dump first if the trail matters.
--
-- Revert the code with it: the events endpoint reads this table on every
-- visit, and the panel's exclusion endpoints write it.
--
-- Safe to run twice: every drop is IF EXISTS guarded, and the CHECKs are
-- dropped before they are re-added.

DROP INDEX IF EXISTS analytics_excluded_ips_business_idx;
DROP TABLE IF EXISTS analytics_excluded_ips;
DROP FUNCTION IF EXISTS karecik_try_inet(TEXT);

DELETE FROM audit_logs
WHERE action IN ('analytics.exclude_ip.add', 'analytics.exclude_ip.remove')
   OR entity_type = 'analytics_exclusion';

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_action_check;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_action_check CHECK (action IN (
    'product.create', 'product.update', 'product.delete',
    'product.price', 'product.bulk_price', 'product.reorder',
    'category.create', 'category.update', 'category.delete',
    'category.reorder',
    'menu.create', 'menu.update', 'menu.delete',
    'business.update', 'account.password_change',
    'upload.create'));

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_entity_type_check;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_entity_type_check CHECK (entity_type IN (
    'product', 'category', 'menu', 'business', 'account', 'upload'));

DELETE FROM schema_migrations WHERE version = '015_analytics_excluded_ips.sql';
