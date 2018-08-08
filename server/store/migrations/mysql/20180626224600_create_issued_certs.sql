-- +migrate Up
CREATE TABLE IF NOT EXISTS `issued_certs` (
  `key_id` varchar(255) NOT NULL,
  `principals` varchar(255) DEFAULT "[]",
  `created_at` datetime DEFAULT '1970-01-01 00:00:01',
  `expires_at` datetime DEFAULT '1970-01-01 00:00:01',
  `revoked` tinyint(1) DEFAULT 0,
  `raw_key` text,
  PRIMARY KEY (`key_id`)
);
CREATE INDEX `idx_expires_at` ON `issued_certs` (`expires_at`);
CREATE INDEX `idx_revoked_expires_at` ON `issued_certs` (`revoked`, `expires_at`);

-- +migrate Down
DROP TABLE `issued_certs`;
