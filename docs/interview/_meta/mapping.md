# 面试题 ↔ 可运行代码映射

| 题 ID | 文档 | 代码路径 | 说明 |
|-------|------|----------|------|
| S-CONC-05, S-CONC-06 | Channel | `basis/channel/main.go` | 无缓冲/有缓冲 channel |
| S-CONC-08, S-CONC-11 | Mutex / WaitGroup | `basis/sync/main.go` | Mutex vs atomic |
| S-CONC-01~04, S-CONC-16 | Goroutine | `basis/goroutine/main.go` | WaitGroup、任务调度 |
| S-CONC-17 | Pipeline | `gin-example/example_28/main.go` | errgroup 多服务 |
| S-MEM-07 | interface | `basis/struct/main.go` | 接口与嵌入 |
| S-MEM-05 | slice | `basis/point/main.go` | 指针与 slice 引用 |
| S-DB-05 | GORM | `gorm/demo/main.go` | 模型、钩子、Preload |
| S-DB-05 | sqlx | `gorm/sqlx/sqlx1/main.go`, `sqlx2/main.go` | 原生 SQL |
| S-NET-03 | Gin 校验 | `gin-example/example_12/main.go` | 自定义 validator |
| S-NET-03 | Gin 绑定 | `gin-example/example_3/main.go` | 嵌套结构体绑定 |
| S-NET-03 | Gin JSON | `gin-example/example_1/main.go` | AsciiJSON |
| S-CODE-01~05 | 手写题 | `examples/senior/lru`, `ratelimit`, `errgroup`, `connpool`, `graceful_shutdown` |
| — | 算法面 | `algorithm/lc_*` | LeetCode 参考实现 |

## 使用方式

1. 阅读 `docs/interview/` 下对应 Markdown。
2. 按上表进入代码目录：`go run .` 或 `go test`。
3. 资深面建议：**先口述再对照代码**，并补充自己的生产案例。
