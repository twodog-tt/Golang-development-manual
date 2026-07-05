# Go 后端与区块链架构师面试手册

面向 **5 年+ Go 后端 + 区块链/Web3 架构师** 的面试知识库（**153 篇正文**）。

**在线阅读**：https://twodog-tt.github.io/Golang-development-manual/

![gopher](./gopher.png)

> **定位**：Go 运行时与系统设计 + 链上工程（Solidity）+ 链下工程（Go RPC/索引）+ 解决方案架构 + 微服务治理 + AI 工程。  
> **⭐ Web3 交易所 / 钱包方向**：[docs/resume-focus-web3.md](./docs/resume-focus-web3.md)

## 快速开始

| 步骤 | 链接 |
|------|------|
| 1. 学习路线（4 周 / 8 周 / 架构师 / Web3） | [docs/learning-path-senior.md](./docs/learning-path-senior.md) |
| 2. **模拟面试**（随机抽题 + 熟练度） | [docs/mock-interview.md](./docs/mock-interview.md) |
| 3. 面试题总索引 | [docs/interview-catalog.md](./docs/interview-catalog.md) |
| 4. Web3 交易所重点题单 | [docs/resume-focus-web3.md](./docs/resume-focus-web3.md) |
| 5. 题单与元数据 | [docs/interview/_meta/questions.yaml](./docs/interview/_meta/questions.yaml) |
| 6. 代码 ↔ 题目映射 | [docs/interview/_meta/mapping.md](./docs/interview/_meta/mapping.md) |
| 7. 题源与引用规范 | [docs/sources.md](./docs/sources.md) |

## 导航结构（由易到难 · 三级菜单）

站点左侧导航为 **分组 → 模块 → 题目标题** 三级结构：

| 层级 | 模块 | 题数 |
|------|------|------|
| **基础 · Go 语言与编码** | 01 并发 → 02 内存 → 08 手写题 | 40 |
| **进阶 · 网络与中间件** | 06 网络 + MySQL/Redis/Kafka/RocketMQ/RabbitMQ/ES/分布式事务 | 26 |
| **高阶 · 系统设计与架构** | 03 系统设计 → 09 云原生 → 11 解决方案架构 → **15 微服务（交易所）** | 42 |
| **专题 · Web3 与交易所** | 12 区块链 Web3 → 13 Solidity → 14 DEX/CEX | 34 |
| **综合 · 领导力与 AI** | 07 工程领导力 → 10 AI 工程 | 11 |

## 模块入口

### 基础 · Go 语言与编码

| 模块 | 题数 | 入口 |
|------|------|------|
| [01 并发与运行时](./docs/interview/01-runtime-concurrency/index.md) | 20 | GMP、Channel、Context、泄漏 |
| [02 内存与 GC](./docs/interview/02-memory-gc/index.md) | 15 | 三色标记、逃逸、pprof |
| [08 手写题](./docs/interview/08-coding-senior/index.md) | 5 | LRU、令牌桶、优雅关闭、errgroup |

### 进阶 · 网络与中间件

| 模块 | 题数 | 入口 |
|------|------|------|
| [06 网络与服务治理](./docs/interview/06-network-governance/index.md) | 5 | gRPC、Gin、JWT、WebSocket |
| [中间件与数据库](./docs/interview/middleware/index.md) | 21 | MySQL(5)、Redis(3)、Kafka(4)、RocketMQ(4)、RabbitMQ(1)、ES(3)、分布式事务(1) |

### 高阶 · 系统设计与架构

| 模块 | 题数 | 入口 |
|------|------|------|
| [03 系统设计](./docs/interview/03-system-design/index.md) | 20 | 秒杀、幂等、缓存、MQ、多活 |
| [09 云原生](./docs/interview/09-cloud-native/index.md) | 8 | K8s、Docker、HPA、Ingress、OTel |
| [11 解决方案架构](./docs/interview/11-solution-architecture/index.md) | 8 | DDD、演进、评审、45min 白板 |
| [15 微服务（交易所场景）](./docs/interview/15-microservices-exchange/index.md) | 6 | 服务拆分、gRPC、WAL、网关、事件总线 |

### 专题 · Web3 与交易所

| 模块 | 题数 | 入口 |
|------|------|------|
| [12 区块链与 Web3（Go）](./docs/interview/12-blockchain-web3/index.md) | 10 | RPC、索引、L2、4337、abigen、MPC |
| [13 Solidity 与合约工程](./docs/interview/13-solidity-contracts/index.md) | 8 | 安全、ERC、Proxy、DeFi |
| [14 DEX / CEX 交易所工程](./docs/interview/14-dex-cex-engineering/index.md) | 16 | 撮合、账务、AMM、K 线、架构白板、永续 |

### 综合 · 领导力与 AI

| 模块 | 题数 | 入口 |
|------|------|------|
| [07 工程与领导力](./docs/interview/07-engineering-leadership/index.md) | 3 | 复盘、技术债、Code Review |
| [10 AI 工程与编程](./docs/interview/10-ai-engineering/index.md) | 8 | LLM、RAG、Agent、MCP |

### Web3 架构师速查

| 链上（13） | 链下（12） | 交易所（14 + 15） |
|------------|------------|-------------------|
| Solidity、ERC、升级、审计 | RPC、索引、签名、abigen | CEX 撮合/账务、DEX AMM、微服务治理 |
| [13-solidity-contracts](./docs/interview/13-solidity-contracts/index.md) | [12-blockchain-web3](./docs/interview/12-blockchain-web3/index.md) | [14-dex-cex-engineering](./docs/interview/14-dex-cex-engineering/index.md) · [15-microservices-exchange](./docs/interview/15-microservices-exchange/index.md) |

<!-- QUESTION_TABLE_START -->
## 面试题全表（153 题）

> 序号按 **基础 → 进阶 → 高阶 → 专题 → 综合** 排列；题号即文档 ID（如 `S-CONC-01`）。点击题目可跳转至 Markdown 正文。

### 基础 · Go 语言与编码

| 序号 | 题号 | 题目 |
|------|------|------|
| 1 | `S-CONC-01` | [GMP 模型全貌与 1.14+ 抢占式调度演进](./docs/interview/01-runtime-concurrency/S-CONC-01-gmp-overview.md) |
| 2 | `S-CONC-02` | [G、M、P 各自职责；去掉 P 会怎样](./docs/interview/01-runtime-concurrency/S-CONC-02-gmp-roles.md) |
| 3 | `S-CONC-03` | [goroutine 栈增长与分裂；和 OS 线程对比](./docs/interview/01-runtime-concurrency/S-CONC-03-goroutine-stack.md) |
| 4 | `S-CONC-04` | [GOMAXPROCS 设置依据；容器 CPU limit 下的误区](./docs/interview/01-runtime-concurrency/S-CONC-04-gomaxprocs.md) |
| 5 | `S-CONC-05` | [Channel 底层结构与有缓冲/无缓冲选型](./docs/interview/01-runtime-concurrency/S-CONC-05-channel.md) |
| 6 | `S-CONC-06` | [Channel 死锁典型场景与排查](./docs/interview/01-runtime-concurrency/S-CONC-06-channel-deadlock.md) |
| 7 | `S-CONC-07` | [select 语义、公平性与 default 的坑](./docs/interview/01-runtime-concurrency/S-CONC-07-select.md) |
| 8 | `S-CONC-08` | [Mutex vs RWMutex vs atomic 选型](./docs/interview/01-runtime-concurrency/S-CONC-08-sync-primitives.md) |
| 9 | `S-CONC-09` | [sync.Map 适用场景与误用](./docs/interview/01-runtime-concurrency/S-CONC-09-sync-map.md) |
| 10 | `S-CONC-10` | [sync.Pool 原理、GC 交互与线上误用](./docs/interview/01-runtime-concurrency/S-CONC-10-sync-pool.md) |
| 11 | `S-CONC-11` | [WaitGroup、Once、Cond 生产用法](./docs/interview/01-runtime-concurrency/S-CONC-11-waitgroup-once-cond.md) |
| 12 | `S-CONC-12` | [Context 树、取消传播、超时与泄漏](./docs/interview/01-runtime-concurrency/S-CONC-12-context.md) |
| 13 | `S-CONC-13` | [goroutine 泄漏成因与 pprof 排查](./docs/interview/01-runtime-concurrency/S-CONC-13-goroutine-leak.md) |
| 14 | `S-CONC-14` | [Go 内存模型与 happens-before](./docs/interview/01-runtime-concurrency/S-CONC-14-memory-model.md) |
| 15 | `S-CONC-15` | [race detector 原理与 data race 面试表达](./docs/interview/01-runtime-concurrency/S-CONC-15-race-detector.md) |
| 16 | `S-CONC-16` | [Worker Pool 设计：队列、背压、优雅退出](./docs/interview/01-runtime-concurrency/S-CONC-16-worker-pool.md) |
| 17 | `S-CONC-17` | [Fan-out/Fan-in 与 Pipeline 瓶颈分析](./docs/interview/01-runtime-concurrency/S-CONC-17-pipeline.md) |
| 18 | `S-CONC-18` | [高并发下 goroutine 泛滥的架构治理](./docs/interview/01-runtime-concurrency/S-CONC-18-goroutine-governance.md) |
| 19 | `S-CONC-19` | [netpoller 与阻塞 syscall 的处理](./docs/interview/01-runtime-concurrency/S-CONC-19-netpoller.md) |
| 20 | `S-CONC-20` | [Go 1.22+ loop 变量与泛型对并发代码的影响](./docs/interview/01-runtime-concurrency/S-CONC-20-go122-generics.md) |
| 21 | `S-MEM-01` | [三色标记与混合写屏障](./docs/interview/02-memory-gc/S-MEM-01-tri-color-gc.md) |
| 22 | `S-MEM-02` | [STW 阶段与 Go 1.5+ GC 演进](./docs/interview/02-memory-gc/S-MEM-02-stw-evolution.md) |
| 23 | `S-MEM-03` | [GC 触发条件与 GOGC 调优](./docs/interview/02-memory-gc/S-MEM-03-gogc-tuning.md) |
| 24 | `S-MEM-04` | [逃逸分析与 -gcflags=-m](./docs/interview/02-memory-gc/S-MEM-04-escape-analysis.md) |
| 25 | `S-MEM-05` | [slice 底层、扩容与内存泄漏场景](./docs/interview/02-memory-gc/S-MEM-05-slice-internals.md) |
| 26 | `S-MEM-06` | [map 并发安全、扩容与 sync.Map 选型](./docs/interview/02-memory-gc/S-MEM-06-map-internals.md) |
| 27 | `S-MEM-07` | [interface 底层 eface/iface 与断言成本](./docs/interview/02-memory-gc/S-MEM-07-interface.md) |
| 28 | `S-MEM-08` | [内存对齐与 unsafe 边界](./docs/interview/02-memory-gc/S-MEM-08-unsafe-alignment.md) |
| 29 | `S-MEM-09` | [大对象、堆外与 OOM 排查](./docs/interview/02-memory-gc/S-MEM-09-oom-debug.md) |
| 30 | `S-MEM-10` | [pprof heap/allocs 实战解读](./docs/interview/02-memory-gc/S-MEM-10-pprof-heap.md) |
| 31 | `S-MEM-11` | [CPU profile vs execution trace 选型](./docs/interview/02-memory-gc/S-MEM-11-pprof-vs-trace.md) |
| 32 | `S-MEM-12` | [减少 GC 压力的系统级手段](./docs/interview/02-memory-gc/S-MEM-12-reduce-gc-pressure.md) |
| 33 | `S-MEM-13` | [延迟敏感服务的 GC 抖动治理](./docs/interview/02-memory-gc/S-MEM-13-gc-jitter.md) |
| 34 | `S-MEM-14` | [new/make 在资深面试中的升维回答](./docs/interview/02-memory-gc/S-MEM-14-new-make.md) |
| 35 | `S-MEM-15` | [defer 链、开销与错误处理](./docs/interview/02-memory-gc/S-MEM-15-defer.md) |
| 36 | `S-CODE-01` | [并发安全 LRU](./docs/interview/08-coding-senior/S-CODE-01-concurrent-lru.md) |
| 37 | `S-CODE-02` | [令牌桶限流器实现](./docs/interview/08-coding-senior/S-CODE-02-token-bucket.md) |
| 38 | `S-CODE-03` | [优雅关闭 HTTP 服务](./docs/interview/08-coding-senior/S-CODE-03-graceful-shutdown.md) |
| 39 | `S-CODE-04` | [errgroup 语义实现](./docs/interview/08-coding-senior/S-CODE-04-errgroup.md) |
| 40 | `S-CODE-05` | [连接池实现要点](./docs/interview/08-coding-senior/S-CODE-05-connection-pool.md) |

### 进阶 · 网络与中间件

| 序号 | 题号 | 题目 |
|------|------|------|
| 41 | `S-NET-01` | [gRPC vs HTTP/REST 选型](./docs/interview/06-network-governance/S-NET-01-grpc-vs-rest.md) |
| 42 | `S-NET-02` | [HTTP 连接池与 Keep-Alive](./docs/interview/06-network-governance/S-NET-02-http-connection-pool.md) |
| 43 | `S-NET-03` | [Gin 中间件链与请求生命周期](./docs/interview/06-network-governance/S-NET-03-gin-middleware.md) |
| 44 | `S-NET-04` | [JWT 认证与安全边界](./docs/interview/06-network-governance/S-NET-04-jwt-auth.md) |
| 45 | `S-NET-05` | [长连接与 WebSocket 网关设计](./docs/interview/06-network-governance/S-NET-05-websocket-gateway.md) |
| 46 | `S-DB-01` | [MySQL 索引原理与最左前缀](./docs/interview/middleware/mysql/S-DB-01-mysql-index.md) |
| 47 | `S-DB-02` | [事务隔离级别与 MVCC](./docs/interview/middleware/mysql/S-DB-02-transaction-mvcc.md) |
| 48 | `S-DB-03` | [慢查询排查与 EXPLAIN](./docs/interview/middleware/mysql/S-DB-03-slow-query.md) |
| 49 | `S-DB-04` | [分库分表策略与跨库查询](./docs/interview/middleware/mysql/S-DB-04-sharding.md) |
| 50 | `S-DB-05` | [GORM 预加载 N+1 与事务陷阱](./docs/interview/middleware/mysql/S-DB-05-gorm-pitfalls.md) |
| 51 | `S-DIST-01` | [Redis 集群模式与选型](./docs/interview/middleware/redis/S-DIST-01-redis-cluster.md) |
| 52 | `S-DIST-02` | [分布式锁 Redlock 争议](./docs/interview/middleware/redis/S-DIST-02-distributed-lock.md) |
| 53 | `S-DIST-03` | [热点 Key 发现与治理](./docs/interview/middleware/redis/S-DIST-03-hot-key.md) |
| 54 | `S-KAFKA-01` | [Kafka 架构与存储：Topic、Partition、ISR](./docs/interview/middleware/kafka/S-KAFKA-01-architecture-storage.md) |
| 55 | `S-KAFKA-02` | [Kafka Producer 可靠性：acks、幂等与分区键](./docs/interview/middleware/kafka/S-KAFKA-02-producer-reliability.md) |
| 56 | `S-KAFKA-03` | [Kafka 交易事件总线：成交广播与 lag 治理](./docs/interview/middleware/kafka/S-KAFKA-03-trade-event-bus.md) |
| 57 | `S-DIST-04` | [Kafka 消费语义与 rebalance](./docs/interview/middleware/kafka/S-DIST-04-kafka-semantics.md) |
| 58 | `S-RMQ-01` | [RocketMQ 架构与核心概念](./docs/interview/middleware/rocketmq/S-RMQ-01-architecture.md) |
| 59 | `S-RMQ-02` | [RocketMQ 顺序/事务/延迟消息](./docs/interview/middleware/rocketmq/S-RMQ-02-order-transaction-delay.md) |
| 60 | `S-RMQ-03` | [RocketMQ 与 Kafka 选型对比](./docs/interview/middleware/rocketmq/S-RMQ-03-vs-kafka.md) |
| 61 | `S-RMQ-04` | [RocketMQ 运维排障：堆积、重试与死信](./docs/interview/middleware/rocketmq/S-RMQ-04-ops-troubleshooting.md) |
| 62 | `S-RAB-01` | [RabbitMQ 拆分链上监听与业务写入](./docs/interview/middleware/rabbitmq/S-RAB-01-exchange-async-pipeline.md) |
| 63 | `S-ES-01` | [Elasticsearch 倒排索引与基本原理](./docs/interview/middleware/elasticsearch/S-ES-01-inverted-index.md) |
| 64 | `S-ES-02` | [Elasticsearch Mapping 查询与聚合](./docs/interview/middleware/elasticsearch/S-ES-02-mapping-query-agg.md) |
| 65 | `S-ES-03` | [Elasticsearch 数据同步与运维](./docs/interview/middleware/elasticsearch/S-ES-03-sync-ops.md) |
| 66 | `S-DIST-05` | [分布式事务 TCC/Saga 对比](./docs/interview/middleware/distributed/S-DIST-05-distributed-transaction.md) |

### 高阶 · 系统设计与架构

| 序号 | 题号 | 题目 |
|------|------|------|
| 67 | `S-ARCH-01` | [设计支撑 10 万 QPS 的读多写少 API](./docs/interview/03-system-design/S-ARCH-01-read-heavy-api.md) |
| 68 | `S-ARCH-02` | [秒杀：库存、超卖、热点 Key](./docs/interview/03-system-design/S-ARCH-02-seckill.md) |
| 69 | `S-ARCH-03` | [分布式 ID：雪花、号段、UUID 取舍](./docs/interview/03-system-design/S-ARCH-03-distributed-id.md) |
| 70 | `S-ARCH-04` | [幂等设计：接口、消息、数据库层](./docs/interview/03-system-design/S-ARCH-04-idempotency.md) |
| 71 | `S-ARCH-05` | [最终一致 vs 强一致：业务怎么选](./docs/interview/03-system-design/S-ARCH-05-consistency-tradeoff.md) |
| 72 | `S-ARCH-06` | [缓存穿透、击穿、雪崩治理体系](./docs/interview/03-system-design/S-ARCH-06-cache-failure-modes.md) |
| 73 | `S-ARCH-07` | [多级缓存与本地缓存一致性](./docs/interview/03-system-design/S-ARCH-07-multi-level-cache.md) |
| 74 | `S-ARCH-08` | [限流：令牌桶、漏桶、分布式限流](./docs/interview/03-system-design/S-ARCH-08-rate-limiting.md) |
| 75 | `S-ARCH-09` | [熔断、降级、舱壁](./docs/interview/03-system-design/S-ARCH-09-circuit-breaker.md) |
| 76 | `S-ARCH-10` | [消息队列：至少一次、恰好一次、顺序性](./docs/interview/03-system-design/S-ARCH-10-mq-semantics.md) |
| 77 | `S-ARCH-11` | [延迟任务与定时任务架构](./docs/interview/03-system-design/S-ARCH-11-delayed-jobs.md) |
| 78 | `S-ARCH-12` | [支付/订单状态机设计](./docs/interview/03-system-design/S-ARCH-12-order-state-machine.md) |
| 79 | `S-ARCH-13` | [多活、异地容灾与 RPO/RTO](./docs/interview/03-system-design/S-ARCH-13-multi-active-dr.md) |
| 80 | `S-ARCH-14` | [微服务拆分边界与何时不该拆](./docs/interview/03-system-design/S-ARCH-14-microservice-boundary.md) |
| 81 | `S-ARCH-15` | [API 版本、灰度发布与特性开关](./docs/interview/03-system-design/S-ARCH-15-release-strategy.md) |
| 82 | `S-ARCH-16` | [可观测性：日志、指标、链路](./docs/interview/03-system-design/S-ARCH-16-observability.md) |
| 83 | `S-ARCH-17` | [SLO/SLI 与错误预算](./docs/interview/03-system-design/S-ARCH-17-slo-error-budget.md) |
| 84 | `S-ARCH-18` | [容量评估与压测方法论](./docs/interview/03-system-design/S-ARCH-18-capacity-planning.md) |
| 85 | `S-ARCH-19` | [从单体到微服务的演进与回退](./docs/interview/03-system-design/S-ARCH-19-monolith-to-microservices.md) |
| 86 | `S-ARCH-20` | [技术选型文档怎么写（Lead 面）](./docs/interview/03-system-design/S-ARCH-20-tech-decision-doc.md) |
| 87 | `S-CLOUD-01` | [Kubernetes 调度与 Go 服务资源 limit](./docs/interview/09-cloud-native/S-CLOUD-01-k8s-scheduling.md) |
| 88 | `S-CLOUD-02` | [Docker 多阶段构建与 Go 镜像最佳实践](./docs/interview/09-cloud-native/S-CLOUD-02-docker-multistage.md) |
| 89 | `S-CLOUD-03` | [OpenTelemetry 与 Go 可观测性接入](./docs/interview/09-cloud-native/S-CLOUD-03-opentelemetry.md) |
| 90 | `S-CLOUD-04` | [滚动发布、探针与 PodDisruptionBudget](./docs/interview/09-cloud-native/S-CLOUD-04-rolling-update-probes-pdb.md) |
| 91 | `S-CLOUD-05` | [HPA 与 Go 服务自定义指标扩缩容](./docs/interview/09-cloud-native/S-CLOUD-05-hpa-autoscaling.md) |
| 92 | `S-CLOUD-06` | [Ingress、Gateway API 与南北向流量](./docs/interview/09-cloud-native/S-CLOUD-06-ingress-gateway.md) |
| 93 | `S-CLOUD-07` | [K8s 故障排查：OOMKilled、CrashLoop 与 Evicted](./docs/interview/09-cloud-native/S-CLOUD-07-k8s-troubleshooting.md) |
| 94 | `S-CLOUD-08` | [ConfigMap、Secret 与 Go 配置热更新](./docs/interview/09-cloud-native/S-CLOUD-08-configmap-secret.md) |
| 95 | `S-SOL-01` | [限界上下文与 DDD 战略设计](./docs/interview/11-solution-architecture/S-SOL-01-bounded-context-ddd.md) |
| 96 | `S-SOL-02` | [绞杀者模式与遗留系统迁移](./docs/interview/11-solution-architecture/S-SOL-02-strangler-fig-migration.md) |
| 97 | `S-SOL-03` | [事件驱动、CQRS 与一致性边界](./docs/interview/11-solution-architecture/S-SOL-03-event-driven-cqrs.md) |
| 98 | `S-SOL-04` | [BFF、API 网关与服务网格职责划分](./docs/interview/11-solution-architecture/S-SOL-04-bff-gateway-mesh.md) |
| 99 | `S-SOL-05` | [多租户 SaaS 隔离与权限架构](./docs/interview/11-solution-architecture/S-SOL-05-multi-tenant-saas.md) |
| 100 | `S-SOL-06` | [架构评审：流程、产出与博弈](./docs/interview/11-solution-architecture/S-SOL-06-architecture-review.md) |
| 101 | `S-SOL-07` | [安全与审计的全局架构](./docs/interview/11-solution-architecture/S-SOL-07-security-audit-architecture.md) |
| 102 | `S-SOL-08` | [45 分钟架构演进白板模板](./docs/interview/11-solution-architecture/S-SOL-08-evolution-whiteboard.md) |
| 103 | `S-MSVC-01` | [交易所微服务全链路白板（CEX+DEX）](./docs/interview/15-microservices-exchange/S-MSVC-01-exchange-microservices-whiteboard.md) |
| 104 | `S-MSVC-02` | [交易域服务拆分与限界上下文](./docs/interview/15-microservices-exchange/S-MSVC-02-domain-decomposition.md) |
| 105 | `S-MSVC-03` | [服务发现与 gRPC 通信治理](./docs/interview/15-microservices-exchange/S-MSVC-03-discovery-grpc-governance.md) |
| 106 | `S-MSVC-04` | [Database per Service 与跨服务一致性](./docs/interview/15-microservices-exchange/S-MSVC-04-database-per-service.md) |
| 107 | `S-MSVC-05` | [API 网关、BFF 与交易流量治理](./docs/interview/15-microservices-exchange/S-MSVC-05-gateway-bff-traffic.md) |
| 108 | `S-MSVC-06` | [事件总线与异步服务边界（交易所）](./docs/interview/15-microservices-exchange/S-MSVC-06-event-bus-async-boundary.md) |

### 专题 · Web3 与交易所

| 序号 | 题号 | 题目 |
|------|------|------|
| 109 | `S-BC-01` | [区块链基础与 EVM 账户模型](./docs/interview/12-blockchain-web3/S-BC-01-blockchain-evm-basics.md) |
| 110 | `S-BC-02` | [Go 连接节点：JSON-RPC 与 ethclient](./docs/interview/12-blockchain-web3/S-BC-02-go-ethereum-rpc.md) |
| 111 | `S-BC-03` | [交易签名与密钥管理](./docs/interview/12-blockchain-web3/S-BC-03-tx-signing-key-mgmt.md) |
| 112 | `S-BC-04` | [智能合约交互：ABI 与事件监听](./docs/interview/12-blockchain-web3/S-BC-04-contract-abi-events.md) |
| 113 | `S-BC-05` | [链上索引器：扫块、重组与幂等](./docs/interview/12-blockchain-web3/S-BC-05-indexer-reorg.md) |
| 114 | `S-BC-06` | [DeFi / NFT 后端架构模式](./docs/interview/12-blockchain-web3/S-BC-06-defi-backend-patterns.md) |
| 115 | `S-BC-07` | [L2 扩容与跨链桥架构](./docs/interview/12-blockchain-web3/S-BC-07-l2-cross-chain-bridge.md) |
| 116 | `S-BC-08` | [Account Abstraction ERC-4337 与 Go 后端](./docs/interview/12-blockchain-web3/S-BC-08-erc4337-account-abstraction.md) |
| 117 | `S-BC-09` | [go-ethereum abigen 完整合约调用实战](./docs/interview/12-blockchain-web3/S-BC-09-abigen-contract-bindings.md) |
| 118 | `S-BC-10` | [MPC/TSS 与 CEX 托管签名架构](./docs/interview/12-blockchain-web3/S-BC-10-mpc-tss-custody.md) |
| 119 | `S-SOLID-01` | [Solidity 语言基础与 storage 布局](./docs/interview/13-solidity-contracts/S-SOLID-01-language-storage.md) |
| 120 | `S-SOLID-02` | [合约安全：重入、权限与 OWASP](./docs/interview/13-solidity-contracts/S-SOLID-02-security-reentrancy.md) |
| 121 | `S-SOLID-03` | [ERC-20 / 721 / 1155 标准与实现](./docs/interview/13-solidity-contracts/S-SOLID-03-erc-standards.md) |
| 122 | `S-SOLID-04` | [可升级合约：Proxy / UUPS / 存储槽](./docs/interview/13-solidity-contracts/S-SOLID-04-upgradeable-proxy.md) |
| 123 | `S-SOLID-05` | [Gas 优化与设计模式](./docs/interview/13-solidity-contracts/S-SOLID-05-gas-optimization.md) |
| 124 | `S-SOLID-06` | [Foundry 测试与审计清单](./docs/interview/13-solidity-contracts/S-SOLID-06-testing-audit.md) |
| 125 | `S-SOLID-07` | [DeFi 合约模式：AMM / Oracle / 闪电贷](./docs/interview/13-solidity-contracts/S-SOLID-07-defi-patterns.md) |
| 126 | `S-SOLID-08` | [合约与 Go 后端架构边界](./docs/interview/13-solidity-contracts/S-SOLID-08-contract-go-boundary.md) |
| 127 | `S-EXCH-01` | [CEX 撮合引擎与订单簿架构](./docs/interview/14-dex-cex-engineering/S-EXCH-01-cex-matching-engine.md) |
| 128 | `S-EXCH-02` | [充值、提现与链上钱包体系](./docs/interview/14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md) |
| 129 | `S-EXCH-03` | [账户体系与资金账务（复式记账）](./docs/interview/14-dex-cex-engineering/S-EXCH-03-account-ledger.md) |
| 130 | `S-EXCH-04` | [合约交易：保证金、强平、资金费率](./docs/interview/14-dex-cex-engineering/S-EXCH-04-futures-margin-liquidation.md) |
| 131 | `S-EXCH-05` | [风控、反洗钱与对账体系](./docs/interview/14-dex-cex-engineering/S-EXCH-05-risk-reconciliation.md) |
| 132 | `S-EXCH-06` | [DEX AMM、流动性池与 LP 收益](./docs/interview/14-dex-cex-engineering/S-EXCH-06-dex-amm-liquidity.md) |
| 133 | `S-EXCH-07` | [DEX 聚合路由、滑点与 Gas 优化](./docs/interview/14-dex-cex-engineering/S-EXCH-07-aggregator-slippage.md) |
| 134 | `S-EXCH-08` | [MEV、抢跑与三明治攻击防护](./docs/interview/14-dex-cex-engineering/S-EXCH-08-mev-sandwich.md) |
| 135 | `S-EXCH-09` | [CEX 与 DEX 混合架构（CeDeFi）](./docs/interview/14-dex-cex-engineering/S-EXCH-09-hybrid-cex-dex.md) |
| 136 | `S-EXCH-10` | [链上成交事件驱动 K 线与行情聚合](./docs/interview/14-dex-cex-engineering/S-EXCH-10-kline-event-aggregation.md) |
| 137 | `S-EXCH-11` | [WebSocket 行情 Hub 与连接治理](./docs/interview/14-dex-cex-engineering/S-EXCH-11-websocket-market-hub.md) |
| 138 | `S-EXCH-12` | [Token 发行平台：毕业、分账与返佣提现](./docs/interview/14-dex-cex-engineering/S-EXCH-12-token-launch-rebate.md) |
| 139 | `S-EXCH-13` | [CEX 端到端交易系统架构（45 分钟白板）](./docs/interview/14-dex-cex-engineering/S-EXCH-13-cex-end-to-end-architecture.md) |
| 140 | `S-EXCH-14` | [Web3 交易所全栈架构（链上 DEX + 链下 Go）](./docs/interview/14-dex-cex-engineering/S-EXCH-14-web3-exchange-fullstack-architecture.md) |
| 141 | `S-EXCH-15` | [交易所清结算、对账与高可用架构](./docs/interview/14-dex-cex-engineering/S-EXCH-15-settlement-ha-disaster-recovery.md) |
| 142 | `S-EXCH-16` | [永续合约撮合与仓位引擎架构](./docs/interview/14-dex-cex-engineering/S-EXCH-16-perpetual-matching-position.md) |

### 综合 · 领导力与 AI

| 序号 | 题号 | 题目 |
|------|------|------|
| 143 | `S-LEAD-01` | [线上事故复盘结构](./docs/interview/07-engineering-leadership/S-LEAD-01-incident-postmortem.md) |
| 144 | `S-LEAD-02` | [技术债识别与偿还策略](./docs/interview/07-engineering-leadership/S-LEAD-02-tech-debt.md) |
| 145 | `S-LEAD-03` | [带团队与 Code Review 文化](./docs/interview/07-engineering-leadership/S-LEAD-03-code-review-culture.md) |
| 146 | `S-AI-01` | [Go 接入大模型 API：流式、重试、超时](./docs/interview/10-ai-engineering/S-AI-01-llm-api-integration.md) |
| 147 | `S-AI-02` | [RAG 架构：分块、向量检索与 Go 落地](./docs/interview/10-ai-engineering/S-AI-02-rag-architecture.md) |
| 148 | `S-AI-03` | [AI Agent 与 Function Calling](./docs/interview/10-ai-engineering/S-AI-03-agent-tool-calling.md) |
| 149 | `S-AI-04` | [Prompt 工程与 Context 窗口管理](./docs/interview/10-ai-engineering/S-AI-04-prompt-context.md) |
| 150 | `S-AI-05` | [LLM 应用安全：注入、PII、护栏](./docs/interview/10-ai-engineering/S-AI-05-llm-security.md) |
| 151 | `S-AI-06` | [LLM 可观测性、成本与延迟优化](./docs/interview/10-ai-engineering/S-AI-06-llm-observability-cost.md) |
| 152 | `S-AI-07` | [Go 实现 MCP Server：工具暴露与 stdio/HTTP 部署](./docs/interview/10-ai-engineering/S-AI-07-mcp-server-go.md) |
| 153 | `S-AI-08` | [多模态与语音接入：图像、音频在 Go 服务中的工程实践](./docs/interview/10-ai-engineering/S-AI-08-multimodal-voice.md) |

<!-- QUESTION_TABLE_END -->

## 可运行代码

| 目录 | 说明 |
|------|------|
| [basis/](./basis/) | goroutine、channel、sync、struct |
| [gin-example/](./gin-example/) | Gin Web 示例 |
| [gorm/](./gorm/) | GORM、sqlx、事务 |
| [algorithm/](./algorithm/) | LeetCode 参考实现 |
| [examples/senior/](./examples/senior/) | LRU、限流、RAG、MCP、ethrpc 等 |
| [examples/solidity/](./examples/solidity/) | 合约示例（重入防护等） |

```bash
# 进入对应示例目录运行
cd basis/goroutine && go run .
```

## 本地预览文档

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements-docs.txt

# 生成模拟面试数据、侧栏导航与 README 题表（新增题目后建议执行）
python3 scripts/generate_mock_interview_data.py
python3 scripts/generate_nav_pages.py
python3 scripts/generate_readme_question_table.py

mkdocs serve   # http://127.0.0.1:8000
```

## 维护脚本

| 脚本 | 作用 |
|------|------|
| [scripts/generate_mock_interview_data.py](./scripts/generate_mock_interview_data.py) | 从 `questions.yaml` 生成 `docs/data/questions.json`（模拟面试页） |
| [scripts/generate_nav_pages.py](./scripts/generate_nav_pages.py) | 生成 `interview/.pages` 与各模块 `.pages`（三级侧栏题目标题） |
| [scripts/generate_readme_question_table.py](./scripts/generate_readme_question_table.py) | 从 `questions.yaml` 更新 README 面试题全表（序号 + 题号 + 题目） |

## 引用来源

见 [docs/sources.md](./docs/sources.md)。
