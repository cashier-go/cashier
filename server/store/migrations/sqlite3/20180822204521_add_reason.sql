-- +migrate Up
ALTER TABLE `issued_certs` ADD COLUMN `message` TEXT NOT NULL DEFAULT "";

-- +migrate Down
CREATE TABLE `issued_certs_new` (
  `id` INTEGER PRIMARY KEY,
  `key_id` varchar(255) UNIQUE NOT NULL,
  `principals` varchar(255) DEFAULT '[]',
  `created_at` datetime DEFAULT '1970-01-01 00:00:01',
  `expires_at` datetime DEFAULT '1970-01-01 00:00:01',
  `revoked` tinyint(1) DEFAULT '0',
  `raw_key` text
);
INSERT INTO `issued_certs_new` (key_id, principals, created_at, expires_at, revoked, raw_key)
  SELECT key_id, principals, created_at, expires_at, revoked, raw_key FROM `issued_certs`;
DROP TABLE `issued_certs`;
ALTER TABLE `issued_certs_new` RENAME TO `issued_certs`;
CREATE INDEX `idx_expires_at` ON `issued_certs` (`expires_at`);