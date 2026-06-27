# 面试题索引

> 5 年+ Go 后端 | 题单元数据：[questions.yaml](./_meta/questions.yaml)

## P0 必过（55 题，已全部撰写）

### [01 并发与运行时](./01-runtime-concurrency/) — 20 题

GMP、Channel、Context、sync 原语、泄漏排查、并发模式。

### [02 内存与 GC](./02-memory-gc/) — 15 题

三色标记、逃逸分析、slice/map/interface、pprof、GOGC 调优。

### [03 系统设计](./03-system-design/) — 20 题

高 QPS 读服务、秒杀、幂等、缓存、限流、MQ、订单状态机、可观测性。

## P1 模块（15 题，已发布）

### [04 分布式与中间件](./04-distributed-middleware/) — 5 题

Redis、分布式锁、热点 Key、Kafka、分布式事务。

### [05 数据库与存储](./05-database-storage/) — 5 题

索引、MVCC、慢查询、分库分表、GORM 陷阱。

### [06 网络与服务治理](./06-network-governance/) — 5 题

gRPC、连接池、Gin 中间件、JWT、WebSocket。

## P2 模块（6 题，已发布）

### [07 工程与领导力](./07-engineering-leadership/) — 3 题

事故复盘、技术债、Code Review 与带团队（Tech Lead / Staff 面）。

### [09 云原生](./09-cloud-native/) — 3 题

K8s 调度、Docker 多阶段构建、OpenTelemetry（可选，JD 含云原生时必刷）。

### [08 手写题](./08-coding-senior/) — 5 题（正文 + `examples/senior/`）

LRU、令牌桶、优雅关闭、errgroup、连接池。

## 高频 Top 10（优先背诵口述）

1. [S-CONC-01 GMP 全貌](./01-runtime-concurrency/S-CONC-01-gmp-overview.md)
2. [S-CONC-05 Channel 选型](./01-runtime-concurrency/S-CONC-05-channel.md)
3. [S-CONC-12 Context](./01-runtime-concurrency/S-CONC-12-context.md)
4. [S-CONC-13 goroutine 泄漏](./01-runtime-concurrency/S-CONC-13-goroutine-leak.md)
5. [S-MEM-01 三色标记 GC](./02-memory-gc/S-MEM-01-tri-color-gc.md)
6. [S-MEM-04 逃逸分析](./02-memory-gc/S-MEM-04-escape-analysis.md)
7. [S-ARCH-02 秒杀](./03-system-design/S-ARCH-02-seckill.md)
8. [S-ARCH-04 幂等](./03-system-design/S-ARCH-04-idempotency.md)
9. [S-ARCH-06 缓存三大问题](./03-system-design/S-ARCH-06-cache-failure-modes.md)
10. [S-ARCH-10 MQ 语义](./03-system-design/S-ARCH-10-mq-semantics.md)
