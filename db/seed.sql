CREATE DATABASE IF NOT EXISTS `certs`;

USE `certs`;

CREATE TABLE `issued_certs` (
  `key_id` varchar(255) NOT NULL,
  `principals` varchar(255) DEFAULT "[]",
  `created_at` datetime DEFAULT '1970-01-01 00:00:01',
  `expires_at` datetime DEFAULT '1970-01-01 00:00:01',
  `revoked` tinyint(1) DEFAULT 0,
  `raw_key` text,
  PRIMARY KEY (`key_id`)
);
