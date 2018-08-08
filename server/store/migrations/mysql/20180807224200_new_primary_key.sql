-- +migrate Up
ALTER TABLE `issued_certs`
  DROP PRIMARY KEY,
  ADD COLUMN `id` INT PRIMARY KEY AUTO_INCREMENT FIRST,
  ADD UNIQUE INDEX `idx_key_id` (`key_id`);

-- +migrate Down
ALTER TABLE `issued_certs`
  DROP PRIMARY KEY,
  DROP COLUMN `id`,
  ADD PRIMARY KEY (`key_id`);
