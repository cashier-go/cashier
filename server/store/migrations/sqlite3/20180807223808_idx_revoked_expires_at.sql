-- +migrate Up
DROP INDEX `idx_revoked_expires_at`;

-- +migrate Down
CREATE INDEX `idx_revoked_expires_at` ON `issued_certs` (`revoked`, `expires_at`);
