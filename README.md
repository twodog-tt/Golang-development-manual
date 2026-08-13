# Go · Agent · Web3 工程知识库

面向 **5 年+ Go 后端 + AI Agent Platform / Crypto Agent Ecosystem + 区块链/Web3 架构** 的工程知识沉淀（**235 篇正文**），并配套可运行示例。

**在线阅读**：https://twodog-tt.github.io/Golang-development-manual/

![gopher](./gopher.png)

> **定位**：Go 运行时与系统设计 + 链上工程（Solidity）+ 链下工程（Go RPC/索引）+ 解决方案架构 + 微服务治理 + AI 工程。  
> **⭐ Web3 交易所 / 钱包方向**：[docs/web3-exchange-wallet-focus.md](./docs/web3-exchange-wallet-focus.md)

## 快速开始

| 步骤 | 链接 |
|------|------|
| 1. 学习路线（4 周 / 8 周 / 架构师 / Web3） | [docs/learning-path-senior.md](./docs/learning-path-senior.md) |
| 2. **专题自测**（随机抽专题 + 熟练度） | [docs/topic-quiz.md](./docs/topic-quiz.md) |
| 3. 专题总索引 | [docs/topic-catalog.md](./docs/topic-catalog.md) |
| 4. Web3 交易所重点专题 | [docs/web3-exchange-wallet-focus.md](./docs/web3-exchange-wallet-focus.md) |
| 5. 角色化优先级与证据标签 | [docs/topics/_meta/role-priority-matrix.md](./docs/topics/_meta/role-priority-matrix.md) |
| 6. P0 技术纠错审计 | [docs/topics/_meta/technical-corrections-audit.md](./docs/topics/_meta/technical-corrections-audit.md) |
| 7. 专题元数据 | [docs/topics/_meta/topics.yaml](./docs/topics/_meta/topics.yaml) |
| 8. 代码 ↔ 专题映射 | [docs/topics/_meta/mapping.md](./docs/topics/_meta/mapping.md) |
| 9. 来源与引用规范 | [docs/sources.md](./docs/sources.md) |

## 导航结构（由易到难 · 三级菜单）

站点左侧导航为 **分组 → 模块 → 专题标题** 三级结构：

| 层级 | 模块 | 篇数 |
|------|------|------|
| **基础 · Go 语言与生产工程** | 01 并发 → 02 内存 → 16 Go 生产工程 → 08 编码练习 | 48 |
| **进阶 · 网络与中间件** | 06 Linux/TCP/网络 + MySQL/PostgreSQL/Redis/MQ/ES/分布式事务 | 33 |
| **高阶 · 系统设计与架构** | 03 系统设计 → 09 云原生 → 11 解决方案架构 → **15 微服务（交易所）** | 45 |
| **专题 · Web3 核心基础设施** | 12 EVM/Rollup/跨链 → 17 多链钱包 → 18 支付 → 19 节点/RPC → 20 协议/共识 → 21 安全工程 → 13 Solidity → 14 交易所/预测市场 | 90 |
| **综合 · 领导力与 AI** | 07 工程领导力 → 10 AI 工程 | 19 |

## 模块入口

### 基础 · Go 语言与生产工程

| 模块 | 篇数 | 入口 |
|------|------|------|
| [01 并发与运行时](./docs/topics/01-runtime-concurrency/index.md) | 20 | GMP、Channel、Context、泄漏 |
| [02 内存与 GC](./docs/topics/02-memory-gc/index.md) | 15 | 三色标记、逃逸、pprof |
| [16 Go 生产工程](./docs/topics/16-go-production-engineering/index.md) | 6 | 错误契约、接口、测试、工具链、供应链 |
| [08 编码练习](./docs/topics/08-coding-senior/index.md) | 7 | LRU、限流、连接池、Singleflight、批处理 |

### 进阶 · 网络与中间件

| 模块 | 篇数 | 入口 |
|------|------|------|
| [06 网络与服务治理](./docs/topics/06-network-governance/index.md) | 7 | Linux FD/epoll、TCP、gRPC、Gin、WebSocket |
| [中间件与数据库](./docs/topics/middleware/index.md) | 26 | MySQL(7)、PostgreSQL(3)、Redis(3)、Kafka(4)、RocketMQ(4)、RabbitMQ(1)、ES(3)、分布式事务(1) |

### 高阶 · 系统设计与架构

| 模块 | 篇数 | 入口 |
|------|------|------|
| [03 系统设计](./docs/topics/03-system-design/index.md) | 21 | 秒杀、幂等、缓存、MQ、多活、CDC/Flink 实时风控 |
| [09 云原生](./docs/topics/09-cloud-native/index.md) | 10 | K8s、Terraform、Helm/GitOps、OTel |
| [11 解决方案架构](./docs/topics/11-solution-architecture/index.md) | 8 | DDD、演进、评审、45min 白板 |
| [15 微服务（交易所场景）](./docs/topics/15-microservices-exchange/index.md) | 6 | 服务拆分、gRPC、WAL、网关、事件总线 |

### 专题 · Web3 核心基础设施

| 模块 | 篇数 | 入口 |
|------|------|------|
| [12 区块链与 Web3（Go）](./docs/topics/12-blockchain-web3/index.md) | 14 | EVM 公链全景、RPC、索引、Rollup、跨链安全、4337、MPC |
| [17 多链钱包与托管](./docs/topics/17-multichain-wallet/index.md) | 12 | BTC、TRON/TRC20、Solana/Cosmos/Aptos/Sui Go 实战、归集、MPC |
| [18 Web3 支付与稳定币](./docs/topics/18-web3-payments-stablecoin/index.md) | 6 | 支付、账本、合规、机构托管、DvP、RWA/ISO 20022 |
| [19 节点、RPC 与 Staking](./docs/topics/19-node-rpc-staking/index.md) | 10 | EL/CL、RPC HA、canonical/ClickHouse/lakehouse、非 EVM 在线可靠性 |
| [20 协议、共识与安全](./docs/topics/20-protocol-consensus-security/index.md) | 5 | PoS/BFT、fork choice、PeerDAS、状态迁移、经典共识对照 |
| [21 Web3 安全工程](./docs/topics/21-security-engineering/index.md) | 4 | 威胁模型、signer fencing、SBOM/provenance、事件响应 |
| [13 Solidity 与合约工程](./docs/topics/13-solidity-contracts/index.md) | 8 | 安全、ERC、Proxy、DeFi |
| [14 DEX / CEX 交易所工程](./docs/topics/14-dex-cex-engineering/index.md) | 31 | AMM/Uniswap/Pancake、Staking/Farm、DEX TL 白板、撮合/WAL、预测市场 |

### 综合 · 领导力与 AI

| 模块 | 篇数 | 入口 |
|------|------|------|
| [07 工程与领导力](./docs/topics/07-engineering-leadership/index.md) | 5 | 复盘、技术债、Staff 战略与跨团队迁移 |
| [10 AI 工程与编程](./docs/topics/10-ai-engineering/index.md) | 14 | 工作流/HITL、MCP/A2A、Agent 身份/Commerce、开放平台/Launchpad |

### Web3 架构师速查

| Go 工程 | 钱包与支付 | 节点与数据 | 合约与交易所 |
|---------|------------|------------|--------------|
| [16 Go 生产工程](./docs/topics/16-go-production-engineering/index.md) · [21 安全工程](./docs/topics/21-security-engineering/index.md) | [17 多链钱包](./docs/topics/17-multichain-wallet/index.md) · [18 支付](./docs/topics/18-web3-payments-stablecoin/index.md) | [19 节点/RPC](./docs/topics/19-node-rpc-staking/index.md) · [20 协议/共识](./docs/topics/20-protocol-consensus-security/index.md) | [13 Solidity](./docs/topics/13-solidity-contracts/index.md) · [14 DEX/CEX](./docs/topics/14-dex-cex-engineering/index.md) |

<!-- QUESTION_TABLE_START -->
## 专题全表（235 篇）

> 序号按 **基础 → 进阶 → 高阶 → 专题 → 综合** 排列；文档 ID（如 `S-CONC-01`）。点击标题可跳转至 Markdown 正文。

### 基础 · Go 语言与生产工程

| 序号 | 文档 ID | 标题 |
|------|------|------|
| 1 | `S-CONC-01` | [GMP 模型与 1.14 以来抢占式调度](./docs/topics/01-runtime-concurrency/S-CONC-01-gmp-overview.md) |
| 2 | `S-CONC-02` | [G、M、P 角色与 P 被移除时会发生什么](./docs/topics/01-runtime-concurrency/S-CONC-02-gmp-roles.md) |
| 3 | `S-CONC-03` | [Goroutine 栈增长与 OS 线程栈对比](./docs/topics/01-runtime-concurrency/S-CONC-03-goroutine-stack.md) |
| 4 | `S-CONC-04` | [GOMAXPROCS 调优与容器环境](./docs/topics/01-runtime-concurrency/S-CONC-04-gomaxprocs.md) |
| 5 | `S-CONC-05` | [Channel 内部实现与有缓冲/无缓冲选型](./docs/topics/01-runtime-concurrency/S-CONC-05-channel.md) |
| 6 | `S-CONC-06` | [Channel 死锁场景与排查](./docs/topics/01-runtime-concurrency/S-CONC-06-channel-deadlock.md) |
| 7 | `S-CONC-07` | [select 语义、公平性与 default 陷阱](./docs/topics/01-runtime-concurrency/S-CONC-07-select.md) |
| 8 | `S-CONC-08` | [Mutex、RWMutex 与 atomic 选型](./docs/topics/01-runtime-concurrency/S-CONC-08-sync-primitives.md) |
| 9 | `S-CONC-09` | [sync.Map 适用场景与误用](./docs/topics/01-runtime-concurrency/S-CONC-09-sync-map.md) |
| 10 | `S-CONC-10` | [sync.Pool 与 GC 交互](./docs/topics/01-runtime-concurrency/S-CONC-10-sync-pool.md) |
| 11 | `S-CONC-11` | [WaitGroup、Once 与 Cond](./docs/topics/01-runtime-concurrency/S-CONC-11-waitgroup-once-cond.md) |
| 12 | `S-CONC-12` | [Context 树、取消传播与泄漏](./docs/topics/01-runtime-concurrency/S-CONC-12-context.md) |
| 13 | `S-CONC-13` | [Goroutine 泄漏成因与 pprof 排查](./docs/topics/01-runtime-concurrency/S-CONC-13-goroutine-leak.md) |
| 14 | `S-CONC-14` | [Go 内存模型与 happens-before](./docs/topics/01-runtime-concurrency/S-CONC-14-memory-model.md) |
| 15 | `S-CONC-15` | [Race Detector 原理与工程实践](./docs/topics/01-runtime-concurrency/S-CONC-15-race-detector.md) |
| 16 | `S-CONC-16` | [Worker Pool 设计模式](./docs/topics/01-runtime-concurrency/S-CONC-16-worker-pool.md) |
| 17 | `S-CONC-17` | [Fan-out/Fan-in 与 Pipeline 模式](./docs/topics/01-runtime-concurrency/S-CONC-17-pipeline.md) |
| 18 | `S-CONC-18` | [Goroutine 泛滥治理与并发预算](./docs/topics/01-runtime-concurrency/S-CONC-18-goroutine-governance.md) |
| 19 | `S-CONC-19` | [Netpoller 与阻塞 Syscall 行为](./docs/topics/01-runtime-concurrency/S-CONC-19-netpoller.md) |
| 20 | `S-CONC-20` | [Go 1.22 循环变量与 Go 1.26 泛型演进](./docs/topics/01-runtime-concurrency/S-CONC-20-go122-generics.md) |
| 21 | `S-MEM-01` | [三色标记与混合写屏障](./docs/topics/02-memory-gc/S-MEM-01-tri-color-gc.md) |
| 22 | `S-MEM-02` | [STW 阶段与 Go 1.5+ GC 演进](./docs/topics/02-memory-gc/S-MEM-02-stw-evolution.md) |
| 23 | `S-MEM-03` | [GC 触发条件与 GOGC 调优](./docs/topics/02-memory-gc/S-MEM-03-gogc-tuning.md) |
| 24 | `S-MEM-04` | [逃逸分析与 -gcflags=-m](./docs/topics/02-memory-gc/S-MEM-04-escape-analysis.md) |
| 25 | `S-MEM-05` | [slice 底层、扩容与内存泄漏场景](./docs/topics/02-memory-gc/S-MEM-05-slice-internals.md) |
| 26 | `S-MEM-06` | [map 并发安全、扩容与 sync.Map 选型](./docs/topics/02-memory-gc/S-MEM-06-map-internals.md) |
| 27 | `S-MEM-07` | [interface 底层 eface/iface 与断言成本](./docs/topics/02-memory-gc/S-MEM-07-interface.md) |
| 28 | `S-MEM-08` | [内存对齐与 unsafe 边界](./docs/topics/02-memory-gc/S-MEM-08-unsafe-alignment.md) |
| 29 | `S-MEM-09` | [大对象、堆外与 OOM 排查](./docs/topics/02-memory-gc/S-MEM-09-oom-debug.md) |
| 30 | `S-MEM-10` | [pprof heap/allocs 实战解读](./docs/topics/02-memory-gc/S-MEM-10-pprof-heap.md) |
| 31 | `S-MEM-11` | [CPU profile vs execution trace 选型](./docs/topics/02-memory-gc/S-MEM-11-pprof-vs-trace.md) |
| 32 | `S-MEM-12` | [减少 GC 压力的系统级手段](./docs/topics/02-memory-gc/S-MEM-12-reduce-gc-pressure.md) |
| 33 | `S-MEM-13` | [延迟敏感服务的 GC 抖动治理](./docs/topics/02-memory-gc/S-MEM-13-gc-jitter.md) |
| 34 | `S-MEM-14` | [new 与 make：分配语义、逃逸与选型](./docs/topics/02-memory-gc/S-MEM-14-new-make.md) |
| 35 | `S-MEM-15` | [defer 链、开销与错误处理](./docs/topics/02-memory-gc/S-MEM-15-defer.md) |
| 36 | `S-GOENG-01` | [错误契约、Wrapping 与 Panic 边界](./docs/topics/16-go-production-engineering/S-GOENG-01-errors-contract-panic-boundary.md) |
| 37 | `S-GOENG-02` | [包边界、接口设计与依赖注入](./docs/topics/16-go-production-engineering/S-GOENG-02-package-interface-di.md) |
| 38 | `S-GOENG-03` | [Go 单元测试：表驱动、子测试与 Test Double](./docs/topics/16-go-production-engineering/S-GOENG-03-testing-table-fake.md) |
| 39 | `S-GOENG-04` | [Fuzz、Benchmark、Race 与回归门禁](./docs/topics/16-go-production-engineering/S-GOENG-04-fuzz-benchmark-race.md) |
| 40 | `S-GOENG-05` | [Go Modules、Workspace、Toolchain 与可复现构建](./docs/topics/16-go-production-engineering/S-GOENG-05-modules-toolchain-reproducible.md) |
| 41 | `S-GOENG-06` | [静态分析、govulncheck 与依赖供应链](./docs/topics/16-go-production-engineering/S-GOENG-06-static-analysis-supply-chain.md) |
| 42 | `S-CODE-06` | [Singleflight 缓存击穿抑制](./docs/topics/08-coding-senior/S-CODE-06-singleflight-cache.md) |
| 43 | `S-CODE-07` | [有界批处理执行器：取消、顺序与背压](./docs/topics/08-coding-senior/S-CODE-07-bounded-batch-executor.md) |
| 44 | `S-CODE-01` | [并发安全 LRU 缓存](./docs/topics/08-coding-senior/S-CODE-01-concurrent-lru.md) |
| 45 | `S-CODE-02` | [令牌桶限流器](./docs/topics/08-coding-senior/S-CODE-02-token-bucket.md) |
| 46 | `S-CODE-03` | [HTTP 服务优雅关闭](./docs/topics/08-coding-senior/S-CODE-03-graceful-shutdown.md) |
| 47 | `S-CODE-04` | [errgroup 语义实现](./docs/topics/08-coding-senior/S-CODE-04-errgroup.md) |
| 48 | `S-CODE-05` | [连接池实现要点](./docs/topics/08-coding-senior/S-CODE-05-connection-pool.md) |

### 进阶 · 网络与中间件

| 序号 | 文档 ID | 标题 |
|------|------|------|
| 49 | `S-NET-06` | [Linux 进程、文件描述符、epoll 与 Go netpoll](./docs/topics/06-network-governance/S-NET-06-linux-fd-epoll-netpoll.md) |
| 50 | `S-NET-07` | [TCP 建连、队列、TIME_WAIT 与故障排查](./docs/topics/06-network-governance/S-NET-07-tcp-lifecycle-queues-timewait.md) |
| 51 | `S-NET-01` | [gRPC vs HTTP/REST 选型](./docs/topics/06-network-governance/S-NET-01-grpc-vs-rest.md) |
| 52 | `S-NET-02` | [HTTP 连接池与 Keep-Alive](./docs/topics/06-network-governance/S-NET-02-http-connection-pool.md) |
| 53 | `S-NET-03` | [Gin 中间件链与请求生命周期](./docs/topics/06-network-governance/S-NET-03-gin-middleware.md) |
| 54 | `S-NET-04` | [JWT 认证与安全边界](./docs/topics/06-network-governance/S-NET-04-jwt-auth.md) |
| 55 | `S-NET-05` | [WebSocket 网关设计](./docs/topics/06-network-governance/S-NET-05-websocket-gateway.md) |
| 56 | `S-DB-06` | [复杂 SQL：JOIN、CTE、窗口函数与执行计划](./docs/topics/middleware/mysql/S-DB-06-advanced-sql.md) |
| 57 | `S-DB-07` | [资金类表设计：DECIMAL、约束、锁与死锁](./docs/topics/middleware/mysql/S-DB-07-financial-schema-locking.md) |
| 58 | `S-DB-01` | [MySQL 索引原理与最左前缀](./docs/topics/middleware/mysql/S-DB-01-mysql-index.md) |
| 59 | `S-DB-02` | [事务隔离级别与 MVCC](./docs/topics/middleware/mysql/S-DB-02-transaction-mvcc.md) |
| 60 | `S-DB-03` | [慢查询排查与 EXPLAIN](./docs/topics/middleware/mysql/S-DB-03-slow-query.md) |
| 61 | `S-DB-04` | [分库分表策略与跨库查询](./docs/topics/middleware/mysql/S-DB-04-sharding.md) |
| 62 | `S-DB-05` | [GORM 预加载 N+1 与事务陷阱](./docs/topics/middleware/mysql/S-DB-05-gorm-pitfalls.md) |
| 63 | `S-PG-01` | [PostgreSQL MVCC、VACUUM、可见性与索引设计](./docs/topics/middleware/postgresql/S-PG-01-mvcc-vacuum-indexes.md) |
| 64 | `S-PG-02` | [PostgreSQL 隔离级别、锁与资金写入](./docs/topics/middleware/postgresql/S-PG-02-isolation-locking-ledger.md) |
| 65 | `S-PG-03` | [PostgreSQL WAL、复制、故障切换与 pgx 连接治理](./docs/topics/middleware/postgresql/S-PG-03-wal-replication-pgx-ha.md) |
| 66 | `S-DIST-01` | [Redis 集群模式与选型](./docs/topics/middleware/redis/S-DIST-01-redis-cluster.md) |
| 67 | `S-DIST-02` | [分布式锁与 Redlock 争议](./docs/topics/middleware/redis/S-DIST-02-distributed-lock.md) |
| 68 | `S-DIST-03` | [热点 Key 发现与治理](./docs/topics/middleware/redis/S-DIST-03-hot-key.md) |
| 69 | `S-KAFKA-01` | [Kafka 架构与存储：Topic、Partition、ISR](./docs/topics/middleware/kafka/S-KAFKA-01-architecture-storage.md) |
| 70 | `S-KAFKA-02` | [Kafka Producer 可靠性：acks、幂等与分区键](./docs/topics/middleware/kafka/S-KAFKA-02-producer-reliability.md) |
| 71 | `S-KAFKA-03` | [Kafka 交易事件总线：成交广播与 lag 治理](./docs/topics/middleware/kafka/S-KAFKA-03-trade-event-bus.md) |
| 72 | `S-DIST-04` | [Kafka 消费语义与 Rebalance](./docs/topics/middleware/kafka/S-DIST-04-kafka-semantics.md) |
| 73 | `S-RMQ-01` | [RocketMQ 架构与核心概念](./docs/topics/middleware/rocketmq/S-RMQ-01-architecture.md) |
| 74 | `S-RMQ-02` | [RocketMQ 顺序消息、事务消息与延迟消息](./docs/topics/middleware/rocketmq/S-RMQ-02-order-transaction-delay.md) |
| 75 | `S-RMQ-03` | [RocketMQ 与 Kafka 选型对比](./docs/topics/middleware/rocketmq/S-RMQ-03-vs-kafka.md) |
| 76 | `S-RMQ-04` | [RocketMQ 运维排障：堆积、重试与死信](./docs/topics/middleware/rocketmq/S-RMQ-04-ops-troubleshooting.md) |
| 77 | `S-RAB-01` | [RabbitMQ 拆分链上监听与业务写入](./docs/topics/middleware/rabbitmq/S-RAB-01-exchange-async-pipeline.md) |
| 78 | `S-ES-01` | [Elasticsearch 倒排索引与基本原理](./docs/topics/middleware/elasticsearch/S-ES-01-inverted-index.md) |
| 79 | `S-ES-02` | [Elasticsearch Mapping、查询与聚合](./docs/topics/middleware/elasticsearch/S-ES-02-mapping-query-agg.md) |
| 80 | `S-ES-03` | [Elasticsearch 数据同步与运维](./docs/topics/middleware/elasticsearch/S-ES-03-sync-ops.md) |
| 81 | `S-DIST-05` | [分布式事务 TCC vs Saga](./docs/topics/middleware/distributed/S-DIST-05-distributed-transaction.md) |

### 高阶 · 系统设计与架构

| 序号 | 文档 ID | 标题 |
|------|------|------|
| 82 | `S-ARCH-01` | [设计支撑 10 万 QPS 的读多写少 API](./docs/topics/03-system-design/S-ARCH-01-read-heavy-api.md) |
| 83 | `S-ARCH-02` | [秒杀：库存、超卖、热点 Key](./docs/topics/03-system-design/S-ARCH-02-seckill.md) |
| 84 | `S-ARCH-03` | [分布式 ID：雪花、号段、UUID 取舍](./docs/topics/03-system-design/S-ARCH-03-distributed-id.md) |
| 85 | `S-ARCH-04` | [幂等设计：接口、消息、数据库层](./docs/topics/03-system-design/S-ARCH-04-idempotency.md) |
| 86 | `S-ARCH-05` | [最终一致 vs 强一致：业务怎么选](./docs/topics/03-system-design/S-ARCH-05-consistency-tradeoff.md) |
| 87 | `S-ARCH-06` | [缓存穿透、击穿、雪崩治理体系](./docs/topics/03-system-design/S-ARCH-06-cache-failure-modes.md) |
| 88 | `S-ARCH-07` | [多级缓存与本地缓存一致性](./docs/topics/03-system-design/S-ARCH-07-multi-level-cache.md) |
| 89 | `S-ARCH-08` | [限流：令牌桶、漏桶、分布式限流](./docs/topics/03-system-design/S-ARCH-08-rate-limiting.md) |
| 90 | `S-ARCH-09` | [熔断、降级、舱壁](./docs/topics/03-system-design/S-ARCH-09-circuit-breaker.md) |
| 91 | `S-ARCH-10` | [消息队列：至少一次、恰好一次、顺序性](./docs/topics/03-system-design/S-ARCH-10-mq-semantics.md) |
| 92 | `S-ARCH-11` | [延迟任务与定时任务架构](./docs/topics/03-system-design/S-ARCH-11-delayed-jobs.md) |
| 93 | `S-ARCH-12` | [支付/订单状态机设计](./docs/topics/03-system-design/S-ARCH-12-order-state-machine.md) |
| 94 | `S-ARCH-13` | [多活、异地容灾与 RPO/RTO](./docs/topics/03-system-design/S-ARCH-13-multi-active-dr.md) |
| 95 | `S-ARCH-14` | [微服务拆分边界与何时不该拆](./docs/topics/03-system-design/S-ARCH-14-microservice-boundary.md) |
| 96 | `S-ARCH-15` | [API 版本、灰度发布与特性开关](./docs/topics/03-system-design/S-ARCH-15-release-strategy.md) |
| 97 | `S-ARCH-16` | [可观测性：日志、指标、链路](./docs/topics/03-system-design/S-ARCH-16-observability.md) |
| 98 | `S-ARCH-17` | [SLO/SLI 与错误预算](./docs/topics/03-system-design/S-ARCH-17-slo-error-budget.md) |
| 99 | `S-ARCH-18` | [容量评估与压测方法论](./docs/topics/03-system-design/S-ARCH-18-capacity-planning.md) |
| 100 | `S-ARCH-19` | [从单体到微服务的演进与回退](./docs/topics/03-system-design/S-ARCH-19-monolith-to-microservices.md) |
| 101 | `S-ARCH-20` | [技术选型文档怎么写（Lead 面）](./docs/topics/03-system-design/S-ARCH-20-tech-decision-doc.md) |
| 102 | `S-ARCH-21` | [实时风控数据平台：CDC、Flink、ES 与可重放链路](./docs/topics/03-system-design/S-ARCH-21-realtime-risk-cdc-flink.md) |
| 103 | `S-CLOUD-01` | [Kubernetes 调度与 Go 服务资源 limit](./docs/topics/09-cloud-native/S-CLOUD-01-k8s-scheduling.md) |
| 104 | `S-CLOUD-02` | [Docker 多阶段构建与 Go 镜像最佳实践](./docs/topics/09-cloud-native/S-CLOUD-02-docker-multistage.md) |
| 105 | `S-CLOUD-03` | [OpenTelemetry 与 Go 可观测性接入](./docs/topics/09-cloud-native/S-CLOUD-03-opentelemetry.md) |
| 106 | `S-CLOUD-04` | [滚动发布、探针与 PodDisruptionBudget](./docs/topics/09-cloud-native/S-CLOUD-04-rolling-update-probes-pdb.md) |
| 107 | `S-CLOUD-05` | [HPA 与 Go 服务自定义指标扩缩容](./docs/topics/09-cloud-native/S-CLOUD-05-hpa-autoscaling.md) |
| 108 | `S-CLOUD-06` | [Ingress、Gateway API 与南北向流量](./docs/topics/09-cloud-native/S-CLOUD-06-ingress-gateway.md) |
| 109 | `S-CLOUD-07` | [K8s 故障排查：OOMKilled、CrashLoop 与 Evicted](./docs/topics/09-cloud-native/S-CLOUD-07-k8s-troubleshooting.md) |
| 110 | `S-CLOUD-08` | [ConfigMap、Secret 与 Go 配置热更新](./docs/topics/09-cloud-native/S-CLOUD-08-configmap-secret.md) |
| 111 | `S-CLOUD-09` | [Terraform State、模块、Drift 与安全变更](./docs/topics/09-cloud-native/S-CLOUD-09-terraform-state-drift-safe-change.md) |
| 112 | `S-CLOUD-10` | [Helm 与 GitOps：持续收敛、发布顺序和回滚](./docs/topics/09-cloud-native/S-CLOUD-10-helm-gitops-rollout-rollback.md) |
| 113 | `S-SOL-01` | [限界上下文与 DDD 战略设计](./docs/topics/11-solution-architecture/S-SOL-01-bounded-context-ddd.md) |
| 114 | `S-SOL-02` | [绞杀者模式与遗留系统迁移](./docs/topics/11-solution-architecture/S-SOL-02-strangler-fig-migration.md) |
| 115 | `S-SOL-03` | [事件驱动、CQRS 与一致性边界](./docs/topics/11-solution-architecture/S-SOL-03-event-driven-cqrs.md) |
| 116 | `S-SOL-04` | [BFF、API 网关与服务网格职责划分](./docs/topics/11-solution-architecture/S-SOL-04-bff-gateway-mesh.md) |
| 117 | `S-SOL-05` | [多租户 SaaS 隔离与权限架构](./docs/topics/11-solution-architecture/S-SOL-05-multi-tenant-saas.md) |
| 118 | `S-SOL-06` | [架构评审：流程、产出与博弈](./docs/topics/11-solution-architecture/S-SOL-06-architecture-review.md) |
| 119 | `S-SOL-07` | [安全与审计的全局架构](./docs/topics/11-solution-architecture/S-SOL-07-security-audit-architecture.md) |
| 120 | `S-SOL-08` | [45 分钟架构演进白板模板](./docs/topics/11-solution-architecture/S-SOL-08-evolution-whiteboard.md) |
| 121 | `S-MSVC-01` | [交易所微服务全链路架构白板（CEX + DEX）](./docs/topics/15-microservices-exchange/S-MSVC-01-exchange-microservices-whiteboard.md) |
| 122 | `S-MSVC-02` | [交易域服务拆分与限界上下文](./docs/topics/15-microservices-exchange/S-MSVC-02-domain-decomposition.md) |
| 123 | `S-MSVC-03` | [服务发现与 gRPC 服务间通信治理](./docs/topics/15-microservices-exchange/S-MSVC-03-discovery-grpc-governance.md) |
| 124 | `S-MSVC-04` | [Database per Service 与跨服务数据一致性](./docs/topics/15-microservices-exchange/S-MSVC-04-database-per-service.md) |
| 125 | `S-MSVC-05` | [API 网关、BFF 与交易流量治理](./docs/topics/15-microservices-exchange/S-MSVC-05-gateway-bff-traffic.md) |
| 126 | `S-MSVC-06` | [事件总线与异步服务边界（交易所）](./docs/topics/15-microservices-exchange/S-MSVC-06-event-bus-async-boundary.md) |

### 专题 · Web3 核心基础设施

| 序号 | 文档 ID | 标题 |
|------|------|------|
| 127 | `S-BC-01` | [区块链基础与 EVM 账户模型](./docs/topics/12-blockchain-web3/S-BC-01-blockchain-evm-basics.md) |
| 128 | `S-BC-02` | [Go 连接节点：JSON-RPC 与 ethclient](./docs/topics/12-blockchain-web3/S-BC-02-go-ethereum-rpc.md) |
| 129 | `S-BC-03` | [交易签名与密钥管理](./docs/topics/12-blockchain-web3/S-BC-03-tx-signing-key-mgmt.md) |
| 130 | `S-BC-04` | [智能合约交互：ABI 与事件监听](./docs/topics/12-blockchain-web3/S-BC-04-contract-abi-events.md) |
| 131 | `S-BC-05` | [链上索引器：扫块、重组与幂等](./docs/topics/12-blockchain-web3/S-BC-05-indexer-reorg.md) |
| 132 | `S-BC-06` | [DeFi / NFT 后端架构模式](./docs/topics/12-blockchain-web3/S-BC-06-defi-backend-patterns.md) |
| 133 | `S-BC-07` | [L2 扩容与跨链桥架构](./docs/topics/12-blockchain-web3/S-BC-07-l2-cross-chain-bridge.md) |
| 134 | `S-BC-08` | [Account Abstraction ERC-4337 与 Go 后端](./docs/topics/12-blockchain-web3/S-BC-08-erc4337-account-abstraction.md) |
| 135 | `S-BC-09` | [go-ethereum abigen 完整合约调用实战](./docs/topics/12-blockchain-web3/S-BC-09-abigen-contract-bindings.md) |
| 136 | `S-BC-10` | [MPC/TSS 与 CEX 托管签名架构](./docs/topics/12-blockchain-web3/S-BC-10-mpc-tss-custody.md) |
| 137 | `S-BC-11` | [Rollup 安全边界：Finality、数据可用性、证明与强制退出](./docs/topics/12-blockchain-web3/S-BC-11-rollup-finality-da-proof-security.md) |
| 138 | `S-BC-12` | [跨链消息与桥安全：认证、重放、限额与故障恢复](./docs/topics/12-blockchain-web3/S-BC-12-cross-chain-message-bridge-security.md) |
| 139 | `S-BC-13` | [Gas / Fee 计费与多链费用差异](./docs/topics/12-blockchain-web3/S-BC-13-gas-fee-multichain.md) |
| 140 | `S-BC-14` | [EVM 公链全景速览：L1、侧链、Rollup 与接入差异](./docs/topics/12-blockchain-web3/S-BC-14-evm-chains-landscape-integration.md) |
| 141 | `S-WALLET-01` | [多链钱包 Chain Adapter 与能力矩阵](./docs/topics/17-multichain-wallet/S-WALLET-01-chain-adapter-capability-matrix.md) |
| 142 | `S-WALLET-02` | [Bitcoin UTXO、Coin Selection、PSBT 与手续费替换](./docs/topics/17-multichain-wallet/S-WALLET-02-bitcoin-utxo-psbt-fee-bump.md) |
| 143 | `S-WALLET-03` | [Solana 账户模型、PDA 与交易生命周期](./docs/topics/17-multichain-wallet/S-WALLET-03-solana-account-pda-transaction.md) |
| 144 | `S-WALLET-04` | [Cosmos SDK、CometBFT、IBC 与账户 Sequence](./docs/topics/17-multichain-wallet/S-WALLET-04-cosmos-cometbft-ibc-sequence.md) |
| 145 | `S-WALLET-05` | [Sui Object 与 Aptos Resource 模型对比](./docs/topics/17-multichain-wallet/S-WALLET-05-sui-aptos-state-model.md) |
| 146 | `S-WALLET-06` | [充值地址、归集、Nonce/UTXO 预占与恢复](./docs/topics/17-multichain-wallet/S-WALLET-06-deposit-sweep-reservation-recovery.md) |
| 147 | `S-WALLET-07` | [MPC/TSS 的 DKG、Reshare 与故障恢复](./docs/topics/17-multichain-wallet/S-WALLET-07-mpc-dkg-reshare-recovery.md) |
| 148 | `S-WALLET-08` | [Solana Go SDK 实战：离线构建、签名与确认状态](./docs/topics/17-multichain-wallet/S-WALLET-08-solana-go-sdk-transaction.md) |
| 149 | `S-WALLET-09` | [Cosmos SDK Go 实战：TxBuilder、SIGN_MODE_DIRECT 与 Sequence](./docs/topics/17-multichain-wallet/S-WALLET-09-cosmos-go-sdk-sign-mode-direct.md) |
| 150 | `S-WALLET-10` | [Aptos Go SDK 实战：BCS 交易、域分离签名与执行跟踪](./docs/topics/17-multichain-wallet/S-WALLET-10-aptos-go-sdk-bcs-transaction.md) |
| 151 | `S-WALLET-11` | [Sui Go 集成实战：Object、Address Balance 与能力演进](./docs/topics/17-multichain-wallet/S-WALLET-11-sui-go-capability-adapter.md) |
| 152 | `S-WALLET-12` | [TRON / TRC20 钱包：资源、权限与交易生命周期](./docs/topics/17-multichain-wallet/S-WALLET-12-tron-trc20-resource-transaction.md) |
| 153 | `S-PAY-01` | [Web3 支付状态机、幂等、Webhook 与冲正](./docs/topics/18-web3-payments-stablecoin/S-PAY-01-payment-state-idempotency-reversal.md) |
| 154 | `S-PAY-02` | [稳定币发行人控制、跨链转移与结算风险](./docs/topics/18-web3-payments-stablecoin/S-PAY-02-stablecoin-issuer-crosschain-risk.md) |
| 155 | `S-PAY-03` | [Treasury、流动性与多链资金再平衡](./docs/topics/18-web3-payments-stablecoin/S-PAY-03-treasury-liquidity-rebalancing.md) |
| 156 | `S-PAY-04` | [支付账本、清结算与三方对账](./docs/topics/18-web3-payments-stablecoin/S-PAY-04-ledger-clearing-settlement-reconciliation.md) |
| 157 | `S-PAY-05` | [KYC/KYB、Travel Rule 与制裁筛查架构](./docs/topics/18-web3-payments-stablecoin/S-PAY-05-compliance-travel-rule-sanctions.md) |
| 158 | `S-PAY-06` | [机构托管、DvP 清算、RWA 生命周期与 ISO 20022](./docs/topics/18-web3-payments-stablecoin/S-PAY-06-institutional-custody-rwa-iso20022.md) |
| 159 | `S-NODE-01` | [Ethereum EL/CL、Full/Archive Node 与同步模式](./docs/topics/19-node-rpc-staking/S-NODE-01-ethereum-node-architecture-sync.md) |
| 160 | `S-NODE-02` | [RPC 高可用：多 Provider、Quorum、Hedging 与缓存](./docs/topics/19-node-rpc-staking/S-NODE-02-rpc-ha-quorum-hedging-cache.md) |
| 161 | `S-NODE-03` | [Validator、Staking、Slashing 与密钥生命周期](./docs/topics/19-node-rpc-staking/S-NODE-03-validator-staking-slashing-keys.md) |
| 162 | `S-NODE-04` | [链上数据平台：Backfill、实时流、Trace 与 Schema](./docs/topics/19-node-rpc-staking/S-NODE-04-chain-data-platform.md) |
| 163 | `S-NODE-05` | [Relayer 与交易管理器：Nonce、Fee、Replacement、Finality](./docs/topics/19-node-rpc-staking/S-NODE-05-relayer-transaction-manager.md) |
| 164 | `S-NODE-06` | [节点运维：升级、快照、Pruning、监控与 Runbook](./docs/topics/19-node-rpc-staking/S-NODE-06-node-operations-runbook.md) |
| 165 | `S-NODE-07` | [Canonical Backfill + Realtime Merge 与 Reorg 提交协议](./docs/topics/19-node-rpc-staking/S-NODE-07-canonical-backfill-realtime-merge.md) |
| 166 | `S-NODE-08` | [Trace、State Diff、版本化 Decoder 与链数据质量](./docs/topics/19-node-rpc-staking/S-NODE-08-trace-state-diff-versioned-decoder-quality.md) |
| 167 | `S-NODE-09` | [非 EVM 在线 SDK：提交、确认、故障注入与升级兼容](./docs/topics/19-node-rpc-staking/S-NODE-09-non-evm-online-sdk-fault-injection.md) |
| 168 | `S-NODE-10` | [链数据列存：ClickHouse 建模、Reorg 与 Lakehouse 分层](./docs/topics/19-node-rpc-staking/S-NODE-10-chain-data-clickhouse-lakehouse.md) |
| 169 | `S-PROTO-01` | [Ethereum PoS、Fork Choice、Finality 与弱主观性](./docs/topics/20-protocol-consensus-security/S-PROTO-01-ethereum-pos-fork-choice-finality.md) |
| 170 | `S-PROTO-02` | [BFT / CometBFT：轮次、锁、安全性与活性](./docs/topics/20-protocol-consensus-security/S-PROTO-02-bft-cometbft-round-lock-safety-liveness.md) |
| 171 | `S-PROTO-03` | [Blob、DA 与 PeerDAS：从 EIP-4844 到 Fusaka](./docs/topics/20-protocol-consensus-security/S-PROTO-03-blob-da-peerdas-security.md) |
| 172 | `S-PROTO-04` | [协议升级、状态迁移与不可回滚边界](./docs/topics/20-protocol-consensus-security/S-PROTO-04-protocol-upgrade-state-migration.md) |
| 173 | `S-PROTO-05` | [经典共识 vs 链上共识：Paxos/Raft/PBFT 与 PoW/PoS/DPoS/BFT](./docs/topics/20-protocol-consensus-security/S-PROTO-05-classic-vs-onchain-consensus.md) |
| 174 | `S-SEC-01` | [Web3 威胁建模、IAM 与信任边界](./docs/topics/21-security-engineering/S-SEC-01-web3-threat-model-iam-trust-boundaries.md) |
| 175 | `S-SEC-02` | [Key Ceremony、远程签名机 Fencing 与恢复](./docs/topics/21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) |
| 176 | `S-SEC-03` | [SBOM、SLSA Provenance 与发布准入](./docs/topics/21-security-engineering/S-SEC-03-sbom-provenance-release-admission.md) |
| 177 | `S-SEC-04` | [Fuzz、Property、Differential Test 与安全事件响应](./docs/topics/21-security-engineering/S-SEC-04-security-testing-incident-response.md) |
| 178 | `S-SOLID-01` | [Solidity 语言基础与 storage 布局](./docs/topics/13-solidity-contracts/S-SOLID-01-language-storage.md) |
| 179 | `S-SOLID-02` | [合约安全：重入、权限与 OWASP](./docs/topics/13-solidity-contracts/S-SOLID-02-security-reentrancy.md) |
| 180 | `S-SOLID-03` | [ERC-20 / 721 / 1155 标准与实现](./docs/topics/13-solidity-contracts/S-SOLID-03-erc-standards.md) |
| 181 | `S-SOLID-04` | [可升级合约：Proxy / UUPS / 存储槽](./docs/topics/13-solidity-contracts/S-SOLID-04-upgradeable-proxy.md) |
| 182 | `S-SOLID-05` | [Gas 优化与设计模式](./docs/topics/13-solidity-contracts/S-SOLID-05-gas-optimization.md) |
| 183 | `S-SOLID-06` | [Foundry 测试与审计清单](./docs/topics/13-solidity-contracts/S-SOLID-06-testing-audit.md) |
| 184 | `S-SOLID-07` | [DeFi 合约模式：AMM / Oracle / 闪电贷](./docs/topics/13-solidity-contracts/S-SOLID-07-defi-patterns.md) |
| 185 | `S-SOLID-08` | [合约与 Go 后端架构边界](./docs/topics/13-solidity-contracts/S-SOLID-08-contract-go-boundary.md) |
| 186 | `S-EXCH-01` | [CEX 撮合引擎与订单簿架构](./docs/topics/14-dex-cex-engineering/S-EXCH-01-cex-matching-engine.md) |
| 187 | `S-EXCH-02` | [充值、提现与链上钱包体系](./docs/topics/14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md) |
| 188 | `S-EXCH-03` | [账户体系与资金账务（复式记账）](./docs/topics/14-dex-cex-engineering/S-EXCH-03-account-ledger.md) |
| 189 | `S-EXCH-04` | [合约交易：保证金、强平、资金费率](./docs/topics/14-dex-cex-engineering/S-EXCH-04-futures-margin-liquidation.md) |
| 190 | `S-EXCH-05` | [风控、反洗钱与对账体系](./docs/topics/14-dex-cex-engineering/S-EXCH-05-risk-reconciliation.md) |
| 191 | `S-EXCH-06` | [DEX AMM、流动性池与 LP 收益](./docs/topics/14-dex-cex-engineering/S-EXCH-06-dex-amm-liquidity.md) |
| 192 | `S-EXCH-07` | [DEX 聚合路由、滑点与 Gas 优化](./docs/topics/14-dex-cex-engineering/S-EXCH-07-aggregator-slippage.md) |
| 193 | `S-EXCH-08` | [MEV、抢跑与三明治攻击防护](./docs/topics/14-dex-cex-engineering/S-EXCH-08-mev-sandwich.md) |
| 194 | `S-EXCH-09` | [CEX 与 DEX 混合架构（CeDeFi）](./docs/topics/14-dex-cex-engineering/S-EXCH-09-hybrid-cex-dex.md) |
| 195 | `S-EXCH-10` | [链上成交事件驱动 K 线与行情聚合](./docs/topics/14-dex-cex-engineering/S-EXCH-10-kline-event-aggregation.md) |
| 196 | `S-EXCH-11` | [WebSocket 行情 Hub 与连接治理](./docs/topics/14-dex-cex-engineering/S-EXCH-11-websocket-market-hub.md) |
| 197 | `S-EXCH-12` | [Token 发行平台：毕业、分账与返佣提现](./docs/topics/14-dex-cex-engineering/S-EXCH-12-token-launch-rebate.md) |
| 198 | `S-EXCH-13` | [CEX 端到端交易系统架构（45 分钟白板）](./docs/topics/14-dex-cex-engineering/S-EXCH-13-cex-end-to-end-architecture.md) |
| 199 | `S-EXCH-14` | [Web3 交易所全栈架构（链上 DEX + 链下 Go）](./docs/topics/14-dex-cex-engineering/S-EXCH-14-web3-exchange-fullstack-architecture.md) |
| 200 | `S-EXCH-15` | [交易所清结算、对账与高可用架构](./docs/topics/14-dex-cex-engineering/S-EXCH-15-settlement-ha-disaster-recovery.md) |
| 201 | `S-EXCH-16` | [永续合约撮合与仓位引擎架构](./docs/topics/14-dex-cex-engineering/S-EXCH-16-perpetual-matching-position.md) |
| 202 | `S-EXCH-17` | [Go 可运行确定性撮合引擎：价格时间优先与订单语义](./docs/topics/14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md) |
| 203 | `S-EXCH-18` | [撮合 WAL、快照与确定性回放：崩溃一致性实战](./docs/topics/14-dex-cex-engineering/S-EXCH-18-wal-snapshot-replay.md) |
| 204 | `S-EXCH-19` | [行情序号、快照桥接与 Gap Recovery](./docs/topics/14-dex-cex-engineering/S-EXCH-19-market-data-sequence-gap-recovery.md) |
| 205 | `S-EXCH-20` | [FIX Session：序号、Resend、Gap Fill 与断线恢复](./docs/topics/14-dex-cex-engineering/S-EXCH-20-fix-session-sequence-recovery.md) |
| 206 | `S-EXCH-21` | [STP 自成交防护：撮合语义、账户边界与监控合规](./docs/topics/14-dex-cex-engineering/S-EXCH-21-self-trade-prevention-surveillance.md) |
| 207 | `S-EXCH-22` | [集合竞价与撮合性能验证：清算价、分配和 Benchmark](./docs/topics/14-dex-cex-engineering/S-EXCH-22-call-auction-performance-validation.md) |
| 208 | `S-EXCH-23` | [预测市场 CTF、Outcome Token 与市场生命周期](./docs/topics/14-dex-cex-engineering/S-EXCH-23-prediction-market-ctf-lifecycle.md) |
| 209 | `S-EXCH-24` | [CLOB-first 预测市场：EIP-712 订单与链上结算](./docs/topics/14-dex-cex-engineering/S-EXCH-24-prediction-market-clob-eip712-settlement.md) |
| 210 | `S-EXCH-25` | [预测市场预言机、事件数据源与争议仲裁](./docs/topics/14-dex-cex-engineering/S-EXCH-25-prediction-market-oracle-dispute-resolution.md) |
| 211 | `S-EXCH-26` | [预测市场安全不变量、测试矩阵与主网上线](./docs/topics/14-dex-cex-engineering/S-EXCH-26-prediction-market-security-testing-mainnet.md) |
| 212 | `S-EXCH-27` | [PancakeSwap V2 与 V3：AMM、流动性与后端集成差异](./docs/topics/14-dex-cex-engineering/S-EXCH-27-pancakeswap-v2-v3-differences.md) |
| 213 | `S-EXCH-28` | [CEX/DEX 多级代理：极差费率、计佣账本与后台隔离](./docs/topics/14-dex-cex-engineering/S-EXCH-28-affiliate-tiered-rate-rebate.md) |
| 214 | `S-EXCH-29` | [DeFi Staking、流动性挖矿与 Yield Farming](./docs/topics/14-dex-cex-engineering/S-EXCH-29-defi-staking-liquidity-mining-yield.md) |
| 215 | `S-EXCH-30` | [Uniswap V2 与 V3 协议机制深挖](./docs/topics/14-dex-cex-engineering/S-EXCH-30-uniswap-v2-v3-protocol.md) |
| 216 | `S-EXCH-31` | [DEX Tech Lead 45 分钟架构白板](./docs/topics/14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md) |

### 综合 · 领导力与 AI

| 序号 | 文档 ID | 标题 |
|------|------|------|
| 217 | `S-LEAD-01` | [线上事故复盘结构](./docs/topics/07-engineering-leadership/S-LEAD-01-incident-postmortem.md) |
| 218 | `S-LEAD-02` | [技术债识别与偿还策略](./docs/topics/07-engineering-leadership/S-LEAD-02-tech-debt.md) |
| 219 | `S-LEAD-03` | [带团队与 Code Review 文化](./docs/topics/07-engineering-leadership/S-LEAD-03-code-review-culture.md) |
| 220 | `S-LEAD-04` | [Staff 技术战略、无职权影响力与案例表达](./docs/topics/07-engineering-leadership/S-LEAD-04-staff-strategy-influence-case.md) |
| 221 | `S-LEAD-05` | [跨团队高风险迁移、灰度切换与回滚边界](./docs/topics/07-engineering-leadership/S-LEAD-05-cross-team-migration-case.md) |
| 222 | `S-AI-01` | [Go 接入大模型 API：流式、重试、超时](./docs/topics/10-ai-engineering/S-AI-01-llm-api-integration.md) |
| 223 | `S-AI-02` | [RAG 架构：分块、向量检索与 Go 落地](./docs/topics/10-ai-engineering/S-AI-02-rag-architecture.md) |
| 224 | `S-AI-03` | [AI Agent 与 Function Calling](./docs/topics/10-ai-engineering/S-AI-03-agent-tool-calling.md) |
| 225 | `S-AI-04` | [Prompt 工程与 Context 窗口管理](./docs/topics/10-ai-engineering/S-AI-04-prompt-context.md) |
| 226 | `S-AI-05` | [LLM 应用安全：注入、PII、护栏](./docs/topics/10-ai-engineering/S-AI-05-llm-security.md) |
| 227 | `S-AI-06` | [LLM 可观测性、成本与延迟优化](./docs/topics/10-ai-engineering/S-AI-06-llm-observability-cost.md) |
| 228 | `S-AI-07` | [Go 实现 MCP Server：工具暴露与 stdio/HTTP 部署](./docs/topics/10-ai-engineering/S-AI-07-mcp-server-go.md) |
| 229 | `S-AI-08` | [多模态与语音接入：图像、音频在 Go 服务中的工程实践](./docs/topics/10-ai-engineering/S-AI-08-multimodal-voice.md) |
| 230 | `S-AI-09` | [Agent 工作流、Human-in-the-loop 与可靠发布控制面](./docs/topics/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) |
| 231 | `S-AI-10` | [Persona、分层 Memory 与反馈学习治理](./docs/topics/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md) |
| 232 | `S-AI-11` | [MCP 与 A2A：Tool、Agent、任务生命周期与跨框架互操作](./docs/topics/10-ai-engineering/S-AI-11-mcp-a2a-vendor-neutral-interoperability.md) |
| 233 | `S-AI-12` | [ERC-8004：Agent 身份、信誉、验证与钱包绑定](./docs/topics/10-ai-engineering/S-AI-12-erc8004-agent-identity-reputation-validation.md) |
| 234 | `S-AI-13` | [x402、x402b 与 ERC-8183：Agent 支付、托管、争议和对账](./docs/topics/10-ai-engineering/S-AI-13-x402-erc8183-agent-commerce.md) |
| 235 | `S-AI-14` | [Crypto Agent SDK、开放平台、Marketplace 与 Launchpad 架构](./docs/topics/10-ai-engineering/S-AI-14-crypto-agent-open-platform-marketplace-launchpad.md) |

<!-- QUESTION_TABLE_END -->
## 可运行代码

| 目录 | 说明 |
|------|------|
| [basis/](./basis/) | goroutine、channel、sync、struct |
| [gin-example/](./gin-example/) | Gin Web 示例 |
| [gorm/](./gorm/) | GORM、sqlx、事务 |
| [algorithm/](./algorithm/) | LeetCode 参考实现 |
| [examples/senior/](./examples/senior/) | LRU、限流、撮合/WAL、跨链 Guard、RAG、RPC 等 |
| [examples/signer-project/](./examples/signer-project/) | bbolt/etcd signer fence、PKCS#11 HSM 验收、跨进程 2-of-3 FROST |
| [examples/non-evm-sdk/](./examples/non-evm-sdk/) | 四链 Go adapter、N/N-1 fixture、localnet 故障与升级门禁 |
| [examples/solidity/](./examples/solidity/) | 合约示例（重入防护等） |

```bash
# 进入对应示例目录运行
cd basis/goroutine && go run .
```

## 本地预览文档

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements-docs.txt

# 生成专题自测数据、侧栏导航与 README 专题表（新增专题后建议执行）
python3 scripts/generate_topic_quiz_data.py
python3 scripts/generate_nav_pages.py
python3 scripts/generate_readme_question_table.py
python3 scripts/verify_knowledge_metadata.py

mkdocs serve   # http://127.0.0.1:8000
```

## 维护脚本

| 脚本 | 作用 |
|------|------|
| [scripts/generate_topic_quiz_data.py](./scripts/generate_topic_quiz_data.py) | 从 `topics.yaml` 生成 `docs/data/topics.json`（专题自测页） |
| [scripts/generate_redirect_maps.py](./scripts/generate_redirect_maps.py) | 生成 `interview/*` → `topics/*` 旧路径重定向 |
| [scripts/generate_nav_pages.py](./scripts/generate_nav_pages.py) | 生成 `topics/.pages` 与各模块 `.pages`（三级侧栏专题标题） |
| [scripts/generate_readme_question_table.py](./scripts/generate_readme_question_table.py) | 从 `topics.yaml` 更新 README 专题全表（序号 + 文档 ID + 标题） |
| [scripts/verify_knowledge_metadata.py](./scripts/verify_knowledge_metadata.py) | 校验 235 篇正文、角色优先级与证据标签一致性 |

## 引用来源

见 [docs/sources.md](./docs/sources.md)。
