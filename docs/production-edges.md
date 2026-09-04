# LuckyGo 生产级边界（不是 Demo 清单）

抽奖系统的事故几乎都出在「库存、时间、重试、并发」上。下面每条都要在代码里有对应实现或明确拒绝，不允许只在 happy path 上过一下。

> V2 变更：玩法从「即时抽（奖品桶 LPOP）」改为「现场大屏抽（live，名单池 SPOP）+ 到期开奖（scheduled，种子洗牌）」。
> 名单统一收进 `participants` 表（Excel 导入 source=import / C 端报名 source=register），两种玩法的抽取对象都是这张表。

## 1. 不超发（硬约束）

- live 模式的名额真相在 Redis 名单池：每奖项一个 SET `lg:live:pool:{aid}:{prizeId}`，只通过 Lua `SPOP pool N` 原子发放，同一参与者不可能被同一批或不同批抽中两次。
- 池子与 MySQL 的一致性：`lg:live:rosterver:{aid}` 版本号，名单变更（导入/删除/报名）时 INCR；Lua 发现池构建版本与当前版本不一致返回 STALE，网关从 MySQL 重算可抽名单重建池后再抽。
- 可抽名单的口径：奖项 `is_all=0` 排除已中过任何奖的人；`is_all=1` 只排除已中过本奖的人；始终排除有未决落库补偿的人（防重复中奖）。
- 每条中奖记录的 `prize_token` 全局唯一，`draw_records.prize_token` 唯一索引兜底，重复落库直接失败。
- 验收：某奖项总名额 N，无论抽多少批、取消多少次重抽，`status='won'` 的记录数最多 N 且不重复。

## 2. Redis 已弹出、MySQL 没写上

错误做法：直接把参与者塞回池且保留幂等键 → 会「凭空再中一次」或「占着奖却查不到」。

正确做法：

1. Lua 一次完成：校验活动状态/时间窗 → 池版本 → `SPOP` → 写幂等键（值为 `drawId:id1,id2,...`，7 天 TTL）。
2. MySQL 插入失败：进程内重试 3 次（主键冲突视为成功）。
3. 仍失败：写 `persist_failures` 补偿表（带 participant_id），由 Worker 重放插入；**默认不回池**（名额已预定给该参与者，池重建时也会被补偿表排除）。
4. 仅当主持人主动「取消本次」时作废：批次记录翻 `undone` + 名单版本号 INCR（下次抽取重建池自动回池）+ 幂等键标记 `UNDONE`（重放该批次直接拒绝）。

## 3. 客户端重试 / 双击 / 超时重放

- 主持人每次点「停止」生成幂等键（UUID），同一批抽取重试带同一把钥匙。
- Lua 先查幂等键：命中返回原中奖名单，不再 `SPOP`；已取消的批次返回 UNDONE 错误。
- 「取消」操作幂等：重复取消返回成功但不重复翻状态（批次已是 UNDONE 直接短路）。

## 4. 奖项与名额

- live/scheduled 模式的奖项即名额：`stock` = 该奖总中奖人数，`per_round_count` = 单次抽取个数（live，1-50 且 ≤ stock）。
- 没有「谢谢参与」和概率权重：大屏抽取人人有奖，抽中谁才是随机的；不再要求权重合计 10000。
- 发布时校验：至少 1 个奖项；类型仅 virtual/physical；stock ≥ 1。
- 发布后奖项配置冻结；要改配置只能回到 draft（重新编辑整体替换奖项）或开新活动。

## 5. 活动状态机

`draft → published → running ⇄ paused → ended / cancelled`（scheduled 开奖后另有 `drawn`）

- 仅 `running` 可抽；`paused` 立即写 Redis meta，Lua 拒绝。
- `published` 到达 `start_at` 由 Worker 翻成 `running`（也允许读路径惰性对照时间翻转，但以 DB 状态为准并 CAS）。
- 到 `end_at` 后 Lua 与网关都拒绝；定时开奖活动进入开奖流程而不是继续抽。
- 禁止跳状态；后台每个动作校验 `version` 乐观锁，防两个管理员同时发布。

## 6. 时间与时钟

- 所有时间存 UTC；活动带 `timezone` 仅用于展示。
- 开奖/收尾用 Redis 延迟队列的 score=unix，Worker 以 DB `end_at` 再校验一次，防队列提前触发。
- 允许最多 2 秒时钟误差；过早开奖拒绝。

## 7. 名单与报名

- 名单幂等：`participants (activity_id, uid)` 唯一，重复导入/重复报名走 upsert，不产生重复人头。
- 报名（register 来源）：活动进行中、未重复报名（enrollments 唯一约束）、未达人数上限、未在黑名单；报名成功即 upsert 进名单并 INCR 名单版本。
- 已中奖的参与者禁止从名单删除（防名单与中奖记录对不上）；删除未中奖者同步 INCR 名单版本。
- live 模式没有单人限次概念（抽取权在主持人）；防刷落在报名侧（唯一报名 + 黑名单）。

## 8. 定时开奖

- 参与者 = `participants` 全表（导入 + 报名统一），种子洗牌一次性抽完。
- 开奖：`SET lock:{activity} NX EX 120`；事务内 CAS 状态到 `drawn` 成功才落名单。
- 洗牌：种子派生（SHA256 链式）；seed 与参与者 id 快照写入 `draw_audits`，可复盘。
- 中奖人数 > 参与人数：按参与人数开，缺的奖记「流奖」而不是循环发。
- 崩溃重试：事务回滚则锁过期后可再开；已 `drawn` 直接返回原名单（幂等）。
- 注册来源（有 user_id）的中奖者自动签发兑换码；导入名单中奖者走线下核销。

## 9. 核销

- 兑换码高熵随机，唯一索引；只在抽取响应里给一次明文，库里存哈希 + 前 8 位前缀供客服查。
- 核销：`UPDATE ... SET status=used, used_at=? WHERE code_hash=? AND status=unused AND tenant_id=?`，影响行数 0 则失败。
- 导入名单中奖者没有账号和兑换码：管理端在中奖名单上点「线下核销」，补一条 `code_prefix='OFFLINE'` 的已核销记录（`draw_ref` 唯一约束防重复核销）。
- 跨租户核销码无效（必须带 tenant）。

## 10. 多租户隔离

- 所有业务表有 `tenant_id`；查询条件强制带上。
- JWT 含 `tenant_id` + `uid` + `role`；C 端活动 public_id 反查租户，与登录用户租户不一致则 403。
- 管理端列表禁止「不带 tenant 的全表扫描接口」。
- 大屏页（/live/:id）必须 admin JWT；上传文件按租户 ID 前缀隔离命名。

## 11. 风控（V1 要落地的最小集）

- 黑名单表：报名/参与时命中直接拒绝。
- 管理端操作写审计日志（谁发布、谁抽取、谁取消、谁核销、谁导名单）。
- 上传仅图片 MIME（服务端嗅探前 512 字节），≤5MB；静态读取拒绝路径穿越。

## 12. 可观测与故障

- 每条抽取日志带 `trace_id, tenant_id, activity_id, drawId`。
- 指标：`live_draw_total{result}`、`persist_retry`、`oversell_suspect`（某奖项中奖数 > 名额则为 1，应永远 0）。
- 健康检查：MySQL / Redis ping；活动 running 但名单池长期 STALE 则告警。

## 13. 安全

- 密码 bcrypt；不返回内部自增 id 给 C 端（活动用 `public_id`）。
- 中奖播报与公示名单脱敏（姓名只留首字）。
- 奖品名、规则文案输出转义（防 XSS）。
- 管理端 JWT 短过期 + 刷新；C 端同。
- 不在日志打印兑换码明文。
