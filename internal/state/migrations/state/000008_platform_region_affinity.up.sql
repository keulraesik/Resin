ALTER TABLE platforms
ADD COLUMN region_failover_order_json TEXT NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS account_regions (
	platform_id    TEXT    NOT NULL,
	account        TEXT    NOT NULL,
	primary_region TEXT    NOT NULL,
	created_at_ns  INTEGER NOT NULL,
	updated_at_ns  INTEGER NOT NULL,
	PRIMARY KEY (platform_id, account)
);

CREATE INDEX IF NOT EXISTS idx_account_regions_platform
ON account_regions(platform_id);
