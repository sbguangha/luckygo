# 压测记录

目标：一等奖库存 N，压测结束后中奖记录恰好 N。

命令示例（服务起来后）：

```
k6 run docs/k6-draw.js
```

当前尚未跑生产压测，数字待补。本地请先用 `go test ./internal/engine -count=1` 验证不超卖。
