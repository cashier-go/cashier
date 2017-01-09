CREATE DATABASE IF NOT EXISTS `certs`;

USE `certs`;

CREATE TABLE `issued_certs` (
  `key_id` varchar(255) NOT NULL,
  `principals` varchar(255) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `expires_at` datetime DEFAULT NULL,
  `revoked` tinyint(1) DEFAULT NULL,
  `raw_key` text,
  PRIMARY KEY (`key_id`)
);
