ALTER TABLE platforms
ADD COLUMN blocked_egress_ips_json TEXT NOT NULL DEFAULT '[]';
