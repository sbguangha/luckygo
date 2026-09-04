# LuckyGo

生产向的多租户营销抽奖 SaaS：商家配奖，用户转盘即时抽或报名后到期开奖。库存走预生成奖品桶，而不是「随机后再扣库存」。

边界条件见 [docs/production-edges.md](docs/production-edges.md)，立项见 [docs/project-charter.md](docs/project-charter.md)。

## 和 Demo 的差别

- 即时抽：Lua 一次完成状态/时间/限次/`LPOP`/幂等，超卖在引擎单测里锁死。
- Redis 弹出后 MySQL 失败：重试落库 + `persist_failures` Worker 补偿，默认不回桶。
- 兑换码只在中奖弹窗展示一次，库里存哈希，核销用 `UPDATE ... WHERE unused`。
- 定时开奖：分布式锁 + 乐观锁，种子写入审计表可复盘；报名人数不足则流奖，不循环发。
- 所有业务表带 `tenant_id`；C 端用户不能跨租户抽。

## 本地启动

1. 启动 MySQL / Redis（Docker Desktop 打开后）：

```bash
docker compose -f deploy/docker-compose.yml up -d mysql redis
```

2. 后端：

```bash
go test ./internal/engine -count=1
go run ./app/gateway -f app/gateway/etc/luckygo-api.yaml
go run ./app/worker -f app/worker/etc/worker.yaml
```

3. 前端：

```bash
cd web/console
npm install
npm run dev
```

浏览器打开 http://localhost:5173/login 注册租户，创建活动（权重之和 10000）后点发布，再打开 C 端链接抽奖。

## 目录

- `app/gateway` go-zero REST
- `app/worker` 到期开奖 / 状态翻转 / 落库补偿
- `internal/engine` 奖品桶与 Lua（可单测）
- `internal/service` 业务
- `web/console` 商家后台 + C 端转盘
