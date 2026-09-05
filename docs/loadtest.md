# 压测记录

## 报名热路径 · 2026-09-05 08:40 CST

工具：WSL Ubuntu `wrk` 4.1 + `scripts/lottery-join.lua`（每请求独立 `user_id` + 随机中文姓名）

```
wrk -t2 -c16 -d8s --latency -s scripts/lottery-join.lua \
  http://172.28.32.1:8888/api/lottery/join
```

| 指标 | 数值 |
|------|------|
| HTTP QPS | 6823.35 |
| 总请求 | 55195（8.09s） |
| 延迟 avg / P50 / P99 / max | 2.49ms / 2.25ms / 9.45ms / 62.06ms |
| HTTP 2xx | 909 |
| HTTP 503（令牌桶 100 req/s） | 54286 |
| 压测后池人数 | 1319 |

原始输出：`docs/loadtest/wrk-join-20260905.log`  
结果图：`docs/loadtest/wrk-join-result.png`  
大屏图：`docs/loadtest/live-concurrent-names.png`（在线 1319 人，格子一人一名）

结论：网关能吃下约 6.8k HTTP QPS 且 P99 &lt; 10ms；业务限流把写入钉在 100/s，过载 503，进程不崩。不要把 6823 写成「成功报名 QPS」。

复现：先开网关 8888 和大屏 `/live`，再执行 `scripts/wrk-join.ps1` 或 `-Stress`。

## 抽奖正确性（非 HTTP 压测）

```
go test ./internal/engine -count=1
```

覆盖：并发 SPOP 不重复、池不足、幂等重放、取消后重放被拒、名单版本变化触发重建、定时开奖种子确定性且流奖不循环发。

端到端冒烟：

```
python scripts/smoke-live.py
```
