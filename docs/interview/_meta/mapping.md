# 面试题 ↔ 可运行代码映射

| 题 ID | 文档 | 代码路径 | 说明 |
|-------|------|----------|------|
| S-CONC-05, S-CONC-06 | Channel | `basis/channel/main.go` | 无缓冲/有缓冲 channel |
| S-CONC-08, S-CONC-11 | Mutex / WaitGroup | `basis/sync/main.go` | Mutex vs atomic |
| S-CONC-01~04, S-CONC-16 | Goroutine | `basis/goroutine/main.go` | WaitGroup、任务调度 |
| S-CONC-17 | Pipeline | `gin-example/example_28/main.go` | errgroup 多服务 |
| S-MEM-07 | interface | `basis/struct/main.go` | 接口与嵌入 |
| S-MEM-05 | slice | `basis/point/main.go` | 指针与 slice 引用 |
| S-DB-05 | GORM | `gorm/demo/main.go` | 见 [middleware/mysql/S-DB-05](../middleware/mysql/S-DB-05-gorm-pitfalls.md) |
| S-DIST-01～03 | Redis | — | [middleware/redis/](../middleware/redis/) |
| S-DIST-04 | Kafka | — | [middleware/kafka/](../middleware/kafka/) |
| S-RMQ-01～03 | RocketMQ | — | [middleware/rocketmq/](../middleware/rocketmq/) |
| S-ES-01～03 | Elasticsearch | — | [middleware/elasticsearch/](../middleware/elasticsearch/) |
| S-DIST-05 | 分布式事务 | — | [middleware/distributed/](../middleware/distributed/) |
| S-DB-05 | sqlx | `gorm/sqlx/sqlx1/main.go`, `sqlx2/main.go` | 原生 SQL |
| S-NET-03 | Gin 校验 | `gin-example/example_12/main.go` | 自定义 validator |
| S-NET-03 | Gin 绑定 | `gin-example/example_3/main.go` | 嵌套结构体绑定 |
| S-NET-03 | Gin JSON | `gin-example/example_1/main.go` | AsciiJSON |
| S-CODE-01 | LRU | [S-CODE-01](../08-coding-senior/S-CODE-01-concurrent-lru.md) | `examples/senior/lru/` |
| S-CODE-02 | 令牌桶 | [S-CODE-02](../08-coding-senior/S-CODE-02-token-bucket.md) | `examples/senior/ratelimit/` |
| S-CODE-03 | 优雅关闭 | [S-CODE-03](../08-coding-senior/S-CODE-03-graceful-shutdown.md) | `examples/senior/graceful_shutdown/` |
| S-CODE-04 | errgroup | [S-CODE-04](../08-coding-senior/S-CODE-04-errgroup.md) | `examples/senior/errgroup/` |
| S-CODE-05 | 连接池 | [S-CODE-05](../08-coding-senior/S-CODE-05-connection-pool.md) | `examples/senior/connpool/` |
| — | 算法面 | `algorithm/lc_*` | LeetCode 参考实现 |

## 使用方式

1. 阅读 `docs/interview/` 下对应 Markdown。
2. 按上表进入代码目录：`go run .` 或 `go test`。
3. 资深面建议：**先口述再对照代码**，并补充自己的生产案例。
