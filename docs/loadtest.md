# 压测记录

目标：某奖项总名额 N，无论并发抽多少批，`status='won'` 的中奖记录恰好 ≤ N 且无重复参与者；定时开奖人数 ≤ 报名数。

当前尚未跑生产压测，数字待补。本地先用引擎单测验证原子性：

```
go test ./internal/engine -count=1
```

覆盖点：并发 SPOP 不重复、池不足报错、幂等键重放返回原批次、取消后批次重放被拒、名单版本变化触发重建、定时开奖种子确定性且流奖不循环发。

端到端冒烟（真实 MySQL/Redis + gateway）：

```
python scripts/smoke-live.py
```
