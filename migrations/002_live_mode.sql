-- 002_live_mode.sql
-- 现场大屏抽奖（live）模式：统一参与者名单 + 装修配置 + 单次抽取人数

-- 参与者名单：live/scheduled 统一名单源；source=import 为 Excel 导入，register 为 C 端报名上球
CREATE TABLE IF NOT EXISTS participants (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    tenant_id     BIGINT UNSIGNED NOT NULL,
    activity_id   BIGINT UNSIGNED NOT NULL,
    uid           VARCHAR(64) NOT NULL COMMENT '工号/编号；register 来源为 U{user_id}',
    name          VARCHAR(64) NOT NULL,
    department    VARCHAR(64) NOT NULL DEFAULT '',
    identity      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '身份/职务',
    avatar_url    VARCHAR(255) NOT NULL DEFAULT '',
    source        VARCHAR(16) NOT NULL DEFAULT 'import' COMMENT 'import|register',
    user_id       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'register 来源关联 users.id，导入名单为 0',
    created_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_participants_uid (activity_id, uid),
    KEY idx_participants_activity (activity_id, id),
    KEY idx_participants_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 活动：名单来源开关 + 界面装修配置（JSON）
ALTER TABLE activities
    ADD COLUMN roster_source VARCHAR(16) NOT NULL DEFAULT 'register' COMMENT 'import|register|both' AFTER mode,
    ADD COLUMN ui_config JSON NULL COMMENT '大屏界面装修配置' AFTER max_enrollments;

-- 奖项：单次抽取个数 + 是否全员参与（false=只从未中奖者抽，true=未中过本奖即可再抽）
ALTER TABLE prizes
    ADD COLUMN per_round_count INT NOT NULL DEFAULT 1 COMMENT '单次抽取个数' AFTER stock,
    ADD COLUMN is_all TINYINT NOT NULL DEFAULT 0 COMMENT '1=全员参与（未中过本奖即可）' AFTER image_url;

-- 中奖记录：关联名单；user_id 允许 0（导入名单无账号）；status 增加 undone（主持人取消本次）
ALTER TABLE draw_records
    ADD COLUMN participant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER user_id,
    MODIFY COLUMN status VARCHAR(16) NOT NULL DEFAULT 'won' COMMENT 'won|pending_persist|undone',
    ADD KEY idx_draw_participant (participant_id);

-- 落库补偿：带上 participant_id，名单重建时排除未决补偿，防重复中奖
ALTER TABLE persist_failures
    ADD COLUMN participant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER user_id;

-- 定时开奖名单：关联名单（user_id 允许 0）
ALTER TABLE scheduled_winners
    ADD COLUMN participant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER user_id;
