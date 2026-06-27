# Senior 面试手写题示例

对应 `docs/interview/08-coding-senior/` 与题单 S-CODE-01～05。

| 题 ID | 目录 | 说明 | 测试 |
|-------|------|------|------|
| S-CODE-01 | [lru/](lru/) | 并发安全 LRU | `go test ./lru/...` |
| S-CODE-02 | [ratelimit/](ratelimit/) | 令牌桶限流 | `go test ./ratelimit/...` |
| S-CODE-03 | [graceful_shutdown/](graceful_shutdown/) | HTTP 优雅关闭 | `go run ./graceful_shutdown/` |
| S-CODE-04 | [errgroup/](errgroup/) | errgroup 语义 | `go test ./errgroup/...` |
| S-CODE-05 | [connpool/](connpool/) | channel 连接池 | `go test ./connpool/...` |

```bash
cd examples/senior
go test ./...
```
