-- +migrate Up
ALTER TABLE `issued_certs` DROP INDEX `idx_revoked_expires_at`;

-- +migrate Down
ALTER TABLE `issued_certs` ADD INDEX `idx_revoked_expires_at` (`revoked`,`expires_at`);
