CREATE TABLE IF NOT EXISTS tenants (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name          VARCHAR(64) NOT NULL,
    status        TINYINT NOT NULL DEFAULT 1 COMMENT '1=active 0=disabled',
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_tenants_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS users (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    role          VARCHAR(16) NOT NULL COMMENT 'admin|user',
    account       VARCHAR(64) NOT NULL,
    password_hash VARCHAR(100) NOT NULL,
    nickname      VARCHAR(64) NOT NULL DEFAULT '',
    status        TINYINT NOT NULL DEFAULT 1,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_users_tenant_account (tenant_id, account),
    KEY idx_users_tenant_role (tenant_id, role)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS activities (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id          BIGINT UNSIGNED NOT NULL,
    public_id          VARCHAR(16) NOT NULL,
    title              VARCHAR(128) NOT NULL,
    mode               VARCHAR(16) NOT NULL COMMENT 'instant|scheduled',
    status             VARCHAR(16) NOT NULL COMMENT 'draft published running paused ended cancelled drawn',
    timezone           VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    start_at           DATETIME(3) NOT NULL,
    end_at             DATETIME(3) NOT NULL,
    max_draws_per_user INT NOT NULL DEFAULT 1,
    max_enrollments    INT NOT NULL DEFAULT 0 COMMENT '0=unlimited, scheduled only',
    version            INT NOT NULL DEFAULT 0,
    published_at       DATETIME(3) NULL,
    drawn_at           DATETIME(3) NULL,
    draw_seed          VARCHAR(64) NULL COMMENT 'scheduled draw audit seed',
    created_at         DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at         DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_activities_public (public_id),
    KEY idx_activities_tenant_status (tenant_id, status),
    KEY idx_activities_end (status, end_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS prizes (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    activity_id   BIGINT UNSIGNED NOT NULL,
    name          VARCHAR(64) NOT NULL,
    kind          VARCHAR(16) NOT NULL COMMENT 'thank_you|virtual|physical',
    stock         INT NOT NULL,
    weight        INT NOT NULL COMMENT 'basis points, sum per activity = 10000',
    image_url     VARCHAR(255) NOT NULL DEFAULT '',
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    KEY idx_prizes_activity (activity_id),
    CONSTRAINT chk_prizes_stock CHECK (stock >= 0),
    CONSTRAINT chk_prizes_weight CHECK (weight >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS draw_records (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id       BIGINT UNSIGNED NOT NULL,
    activity_id     BIGINT UNSIGNED NOT NULL,
    user_id         BIGINT UNSIGNED NOT NULL,
    prize_id        BIGINT UNSIGNED NOT NULL,
    prize_token     VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(64) NOT NULL,
    kind            VARCHAR(16) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'won' COMMENT 'won|pending_persist',
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_draw_token (prize_token),
    UNIQUE KEY uk_draw_idemp (activity_id, user_id, idempotency_key),
    KEY idx_draw_user (tenant_id, user_id, created_at),
    KEY idx_draw_activity (activity_id, prize_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS persist_failures (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id       BIGINT UNSIGNED NOT NULL,
    activity_id     BIGINT UNSIGNED NOT NULL,
    user_id         BIGINT UNSIGNED NOT NULL,
    prize_id        BIGINT UNSIGNED NOT NULL,
    prize_token     VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(64) NOT NULL,
    kind            VARCHAR(16) NOT NULL,
    payload         JSON NOT NULL,
    retry_count     INT NOT NULL DEFAULT 0,
    last_error      VARCHAR(255) NOT NULL DEFAULT '',
    next_retry_at   DATETIME(3) NOT NULL,
    resolved_at     DATETIME(3) NULL,
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_persist_token (prize_token),
    KEY idx_persist_retry (resolved_at, next_retry_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS enrollments (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    activity_id   BIGINT UNSIGNED NOT NULL,
    user_id       BIGINT UNSIGNED NOT NULL,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_enroll_user (activity_id, user_id),
    KEY idx_enroll_activity (activity_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS scheduled_winners (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    activity_id   BIGINT UNSIGNED NOT NULL,
    user_id       BIGINT UNSIGNED NOT NULL,
    prize_id      BIGINT UNSIGNED NOT NULL,
    prize_token   VARCHAR(64) NOT NULL,
    rank_no       INT NOT NULL,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_sched_token (prize_token),
    UNIQUE KEY uk_sched_user (activity_id, user_id),
    KEY idx_sched_activity (activity_id, rank_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS redemptions (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    activity_id   BIGINT UNSIGNED NOT NULL,
    user_id       BIGINT UNSIGNED NOT NULL,
    prize_id      BIGINT UNSIGNED NOT NULL,
    draw_ref      VARCHAR(64) NOT NULL COMMENT 'draw_records.prize_token or scheduled token',
    code_hash     CHAR(64) NOT NULL,
    code_prefix   VARCHAR(8) NOT NULL COMMENT 'first 8 chars for support search, not enough to guess',
    status        VARCHAR(16) NOT NULL DEFAULT 'unused' COMMENT 'unused|used|void',
    address       VARCHAR(255) NOT NULL DEFAULT '',
    contact_name  VARCHAR(32) NOT NULL DEFAULT '',
    contact_phone VARCHAR(32) NOT NULL DEFAULT '',
    used_at       DATETIME(3) NULL,
    used_by       BIGINT UNSIGNED NULL,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_redeem_hash (code_hash),
    UNIQUE KEY uk_redeem_draw (draw_ref),
    KEY idx_redeem_tenant_status (tenant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS blacklist (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    user_id       BIGINT UNSIGNED NOT NULL,
    reason        VARCHAR(255) NOT NULL DEFAULT '',
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_bl_user (tenant_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS audit_logs (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    actor_id      BIGINT UNSIGNED NOT NULL,
    action        VARCHAR(32) NOT NULL,
    target_type   VARCHAR(32) NOT NULL,
    target_id     VARCHAR(64) NOT NULL,
    detail        JSON NULL,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    KEY idx_audit_tenant (tenant_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS draw_audits (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    activity_id   BIGINT UNSIGNED NOT NULL,
    seed          VARCHAR(64) NOT NULL,
    participant_snapshot JSON NOT NULL,
    winner_snapshot JSON NOT NULL,
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_draw_audit_activity (activity_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
