DROP INDEX IF EXISTS idx_account_regions_platform;
DROP TABLE IF EXISTS account_regions;
ALTER TABLE platforms DROP COLUMN region_failover_order_json;
