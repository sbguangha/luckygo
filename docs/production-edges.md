# LuckyGo 生产级边界（不是 Demo 清单）

抽奖系统的事故几乎都出在「库存、时间、重试、并发」上。下面每条都要在代码里有对应实现或明确拒绝，不允许只在 happy path 转盘上过一下。

## 1. 不超卖（硬约束）

- 即时抽的库存真相在 Redis 奖品桶：只通过 Lua `LPOP` 发放。
- 每个桶元素是 `{prize_id, token}`，`token` 全局唯一；`draw_records.prize_token` 唯一索引，重复落库直接失败。
- 压测验收：一等奖库存 N，结束后中奖记录恰好 N，多一条即事故。

## 2. Redis 已弹出、MySQL 没写上

错误做法：直接 `LPUSH` 回桶且保留幂等键 → 用户重试会「凭空再中一次」或「占着奖却查不到」。

正确做法：

1. Lua 一次完成：校验活动状态/时间/限次 → `LPOP` → 限次 +1 → 写入幂等键。
2. MySQL 插入失败：进程内重试 3 次（主键冲突视为成功）。
3. 仍失败：写 `persist_failures` 补偿表，由 Worker 重放插入；**默认不回桶**（名额已预定给该用户）。
4. 仅当判定该次抽奖作废（例如用户非法）时，用第二条 Lua 原子撤销：幂等键仍指向该 token 才 `LPUSH` + 限次 -1 + 删幂等键。

## 3. 客户端重试 / 双击 / 超时重放

- 客户端每次点击生成 `Idempotency-Key`（UUID），超时重试带同一把钥匙。
- 先查 Redis 幂等再 Lua；命中则返回同一结果，不再 `LPOP`。
- 网关对同一用户抽奖接口额外并发限制（单用户 in-flight = 1），避免双开两个 Tab 绕过「点得快」。

## 4. 概率与库存

- 权重用整数万分比，禁止 float。
- 发布时：各奖 `weight` 之和 = 10000；真实奖 `stock >= 1`；「谢谢参与」必须存在。
- 桶长度 = 各奖 `stock` 之和（谢谢参与也是有限库存）。抽完即活动抽空，不再暗中按概率乱补（暗补会导致超卖或概率与展示不符）。
- 发布后奖品配置冻结，改库存必须先下架再走新活动（或「追加库存」走单独的补桶接口，补桶加分布式锁）。

## 5. 活动状态机

`draft → published → running ⇄ paused → ended / cancelled`

- 仅 `running` 可抽；`paused` 立即写 Redis meta，Lua 拒绝。
- `published` 到达 `start_at` 由 Worker 翻成 `running`（也允许首次抽奖时惰性对照时间翻转，但以 DB 状态为准并 CAS）。
- 到 `end_at` 后 Lua 与网关都拒绝；定时开奖活动进入开奖流程而不是继续抽。
- 禁止跳状态；后台每个动作校验 `version` 乐观锁，防两个管理员同时发布。

## 6. 时间与时钟

- 所有时间存 UTC；活动带 `timezone` 仅用于展示。
- 开奖/收尾用 Redis 延迟队列的 score=unix，Worker 以 DB `end_at` 再校验一次，防队列提前触发。
- 允许最多 2 秒时钟误差；过早开奖拒绝。

## 7. 限次语义

- `max_draws_per_user` 是活动期内总次数，不是「每天」（若要每日限次另加 `max_draws_per_user_per_day` + 按日 Redis key）。
- 限次与扣桶必须在同一 Lua 里，禁止「先查次数再 LPOP」（TOCTOU）。
- 谢谢参与也计次（防刷接口）。

## 8. 定时开奖

- 报名：活动进行中、未重复报名、未达人数上限。
- 开奖：`SET lock:{activity} NX EX 120`，事务内 `SELECT ... FOR UPDATE` 活动行，状态必须是 `ended` 且 `drawn_at IS NULL`。
- 洗牌：`crypto/rand`；把 seed 与参与者 id 快照写入 `draw_audits`，可复盘。
- 中奖人数 > 报名人数：按报名人数开，缺的奖记「流奖」而不是循环发。
- 崩溃重试：事务回滚则锁过期后可再开；已 `drawn` 直接返回原名单（幂等）。

## 9. 核销

- 兑换码高熵随机，唯一索引；响应里只给中奖人看一次明文（库里存哈希也可以，V1 存密文+展示码分离：`code_hash` + 创建时返回明文）。
- 核销：`UPDATE ... SET status=used, used_at=? WHERE code_hash=? AND status=unused AND tenant_id=?`，影响行数 0 则失败。
- 跨租户核销码无效（必须带 tenant）。

## 10. 多租户隔离

- 所有业务表有 `tenant_id`；查询条件强制带上。
- JWT 含 `tenant_id` + `uid` + `role`；C 端活动 public_id 反查租户，与登录用户租户不一致则 403。
- 管理端列表禁止「不带 tenant 的全表扫描接口」。

## 11. 风控（V1 要落地的最小集）

- 单用户抽奖 QPS 限制 + 活动级 QPS。
- 可选：绑定 IP 当日上限（防简单脚本）；不做人脸。
- 黑名单表：命中直接拒绝。
- 管理端操作写审计日志（谁发布、谁核销、谁暂停）。

## 12. 可观测与故障

- 每条抽奖日志带 `trace_id, tenant_id, activity_id, user_id, prize_token`。
- 指标：`draw_total{result}`、`bucket_lpop_latency`、`persist_retry`、`oversell_suspect`（中奖数>库存则为 1，应永远 0）。
- 健康检查：MySQL / Redis ping；桶 key 不存在但活动 running 则告警。

## 13. 安全

- 密码 bcrypt；不返回内部自增 id 给 C 端（活动用 `public_id`）。
- 奖品名、规则文案输出转义（防 XSS）。
- 管理端 JWT 短过期 + 刷新；C 端同。
- 不在日志打印兑换码明文。
