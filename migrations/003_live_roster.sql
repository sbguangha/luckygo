-- 年会扫码名单（网关启动时也会 CREATE IF NOT EXISTS，本文件便于人工建库）
CREATE TABLE IF NOT EXISTS live_roster (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id VARCHAR(64) NOT NULL,
  name VARCHAR(32) NOT NULL,
  staff_no VARCHAR(32) NOT NULL DEFAULT '',
  source VARCHAR(16) NOT NULL DEFAULT 'form',
  openid VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  won_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_live_roster_user (user_id),
  KEY idx_live_roster_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
