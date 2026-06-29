# Web3 交易所重点准备题单

**图例**：⭐ **P0 必背**（岗位高频） · 🔶 **P1 强化**（二面 / 架构延伸） · ○ **P2 基础**（Go 核心保底）

---

## 按技术场景速查

| 面试常问 | 对应题目 |
|----------|----------|
| 链上索引、事件幂等、K 线、WebSocket 行情、RabbitMQ、UUPS 合约、返佣提现 | [P0 区块 Web3 + DEX 专题](#p0-25)（[12 Web3](interview/12-blockchain-web3/index.md) · [14 DEX](interview/14-dex-cex-engineering/index.md)） |
| 多链充提、reorg、MPC/TSS、热冷钱包、提现风控 | [S-EXCH-02](interview/14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md)、[S-BC-10](interview/12-blockchain-web3/S-BC-10-mpc-tss-custody.md) |
| 实时风控、ES、MQ、限流熔断 | [S-ARCH-08](interview/03-system-design/S-ARCH-08-rate-limiting.md)、[S-ES 系列](interview/middleware/elasticsearch/index.md) |

---

## P0 必背（25 题）

优先按 **DEX 链上交易 → CEX 钱包托管 → 通用工程** 口述，每题准备 **1 个生产案例**。

### 链上索引与 Web3 Go

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-BC-05](interview/12-blockchain-web3/S-BC-05-indexer-reorg.md) | 链上索引器：扫块、重组与幂等 | 区块游标、tx_hash+log_index |
| ⭐ [S-BC-10](interview/12-blockchain-web3/S-BC-10-mpc-tss-custody.md) | MPC/TSS 与 CEX 托管签名 | 门限签名、提现链路 |
| ⭐ [S-BC-04](interview/12-blockchain-web3/S-BC-04-contract-abi-events.md) | ABI 与事件监听 | Swap/TokenCreated 等 |
| ⭐ [S-BC-02](interview/12-blockchain-web3/S-BC-02-go-ethereum-rpc.md) | JSON-RPC 与 ethclient | 多链 RPC Client |
| ⭐ [S-BC-03](interview/12-blockchain-web3/S-BC-03-tx-signing-key-mgmt.md) | 交易签名与密钥管理 | KMS/HSM |
| ⭐ [S-BC-09](interview/12-blockchain-web3/S-BC-09-abigen-contract-bindings.md) | abigen 合约调用 | 合约集成 |

### DEX / CEX 业务

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-EXCH-02](interview/14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md) | 充值提现与钱包体系 | 多链充提、确认数 |
| ⭐ [S-EXCH-03](interview/14-dex-cex-engineering/S-EXCH-03-account-ledger.md) | 账户与复式记账 | 账务、返佣 |
| ⭐ [S-EXCH-05](interview/14-dex-cex-engineering/S-EXCH-05-risk-reconciliation.md) | 风控与对账 | 黑名单、审计 |
| ⭐ [S-EXCH-06](interview/14-dex-cex-engineering/S-EXCH-06-dex-amm-liquidity.md) | DEX AMM 与 LP | 恒定乘积、外盘迁移 |
| ⭐ [S-EXCH-10](interview/14-dex-cex-engineering/S-EXCH-10-kline-event-aggregation.md) | 链上事件驱动 K 线 | K 线、排行榜 |
| ⭐ [S-EXCH-11](interview/14-dex-cex-engineering/S-EXCH-11-websocket-market-hub.md) | WebSocket 行情 Hub | 实时推送 |
| ⭐ [S-EXCH-12](interview/14-dex-cex-engineering/S-EXCH-12-token-launch-rebate.md) | Token 发行与返佣提现 | 毕业、分账、提现 |

### 合约与 API

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-SOLID-04](interview/13-solidity-contracts/S-SOLID-04-upgradeable-proxy.md) | UUPS 可升级合约 | 灰度迁移/回滚 |
| ⭐ [S-SOLID-02](interview/13-solidity-contracts/S-SOLID-02-security-reentrancy.md) | 合约安全 | Operator/暂停 |
| ⭐ [S-SOLID-08](interview/13-solidity-contracts/S-SOLID-08-contract-go-boundary.md) | 合约与 Go 边界 | 链上链下分层 |
| ⭐ [S-NET-05](interview/06-network-governance/S-NET-05-websocket-gateway.md) | WebSocket 网关 | 长连接治理 |
| ⭐ [S-NET-03](interview/06-network-governance/S-NET-03-gin-middleware.md) | Gin 中间件 | REST API 分层 |
| ⭐ [S-RAB-01](interview/middleware/rabbitmq/S-RAB-01-exchange-async-pipeline.md) | RabbitMQ 拆分链上链路 | 监听与写入解耦 |

### 数据与稳定性

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-ARCH-04](interview/03-system-design/S-ARCH-04-idempotency.md) | 幂等设计 | 事件、提现重试 |
| ⭐ [S-DB-05](interview/middleware/mysql/S-DB-05-gorm-pitfalls.md) | GORM 陷阱 | ORM 与事务 |
| ⭐ [S-DB-02](interview/middleware/mysql/S-DB-02-transaction-mvcc.md) | 事务与 MVCC | 账务一致性 |
| ⭐ [S-DIST-01](interview/middleware/redis/S-DIST-01-redis-cluster.md) | Redis 集群 | 行情缓存 |
| ⭐ [S-DIST-02](interview/middleware/redis/S-DIST-02-distributed-lock.md) | 分布式锁 | 提现排队 |
| ⭐ [S-CODE-03](interview/08-coding-senior/S-CODE-03-graceful-shutdown.md) | 优雅关闭 | 滚动发布 |

---

## P1 强化（19 题）

| 题 ID | 题目 |
|-------|------|
| 🔶 [S-EXCH-01](interview/14-dex-cex-engineering/S-EXCH-01-cex-matching-engine.md) | CEX 撮合引擎 |
| 🔶 [S-EXCH-07](interview/14-dex-cex-engineering/S-EXCH-07-aggregator-slippage.md) | 聚合路由与滑点 |
| 🔶 [S-EXCH-08](interview/14-dex-cex-engineering/S-EXCH-08-mev-sandwich.md) | MEV 与三明治 |
| 🔶 [S-EXCH-09](interview/14-dex-cex-engineering/S-EXCH-09-hybrid-cex-dex.md) | CeDeFi 混合 |
| 🔶 [S-BC-06](interview/12-blockchain-web3/S-BC-06-defi-backend-patterns.md) | DeFi 后端模式 |
| 🔶 [S-BC-07](interview/12-blockchain-web3/S-BC-07-l2-cross-chain-bridge.md) | L2 与跨链 |
| 🔶 [S-SOLID-03](interview/13-solidity-contracts/S-SOLID-03-erc-standards.md) | ERC 标准 |
| 🔶 [S-SOLID-07](interview/13-solidity-contracts/S-SOLID-07-defi-patterns.md) | DeFi 合约模式 |
| 🔶 [S-ARCH-08](interview/03-system-design/S-ARCH-08-rate-limiting.md) | 限流 |
| 🔶 [S-ARCH-09](interview/03-system-design/S-ARCH-09-circuit-breaker.md) | 熔断 |
| 🔶 [S-ARCH-16](interview/03-system-design/S-ARCH-16-observability.md) | 可观测性 |
| 🔶 [S-ARCH-15](interview/03-system-design/S-ARCH-15-release-strategy.md) | 灰度发布 |
| 🔶 [S-RMQ-02](interview/middleware/rocketmq/S-RMQ-02-order-transaction-delay.md) | RocketMQ 事务/顺序 |
| 🔶 [S-DIST-04](interview/middleware/kafka/S-DIST-04-kafka-semantics.md) | Kafka 消费语义 |
| 🔶 [S-ES-01](interview/middleware/elasticsearch/S-ES-01-inverted-index.md) | ES 倒排索引 |
| 🔶 [S-ES-03](interview/middleware/elasticsearch/S-ES-03-sync-ops.md) | ES 数据同步 |
| 🔶 [S-CLOUD-04](interview/09-cloud-native/S-CLOUD-04-rolling-update-probes-pdb.md) | 滚动发布与探针 |
| 🔶 [S-LEAD-01](interview/07-engineering-leadership/S-LEAD-01-incident-postmortem.md) | 事故复盘 |
| 🔶 [S-NET-04](interview/06-network-governance/S-NET-04-jwt-auth.md) | JWT 鉴权 |

---

## P2 基础巩固（12 题）

| 题 ID | 题目 |
|-------|------|
| ○ [S-CONC-01](interview/01-runtime-concurrency/S-CONC-01-gmp-overview.md) | GMP 模型 |
| ○ [S-CONC-05](interview/01-runtime-concurrency/S-CONC-05-channel.md) | Channel |
| ○ [S-CONC-08](interview/01-runtime-concurrency/S-CONC-08-sync-primitives.md) | Mutex/atomic |
| ○ [S-CONC-12](interview/01-runtime-concurrency/S-CONC-12-context.md) | Context |
| ○ [S-CONC-13](interview/01-runtime-concurrency/S-CONC-13-goroutine-leak.md) | goroutine 泄漏 |
| ○ [S-MEM-01](interview/02-memory-gc/S-MEM-01-tri-color-gc.md) | 三色标记 GC |
| ○ [S-MEM-04](interview/02-memory-gc/S-MEM-04-escape-analysis.md) | 逃逸分析 |
| ○ [S-MEM-10](interview/02-memory-gc/S-MEM-10-pprof-heap.md) | pprof heap |
| ○ [S-ARCH-02](interview/03-system-design/S-ARCH-02-seckill.md) | 秒杀 |
| ○ [S-ARCH-06](interview/03-system-design/S-ARCH-06-cache-failure-modes.md) | 缓存三大问题 |
| ○ [S-DB-01](interview/middleware/mysql/S-DB-01-mysql-index.md) | MySQL 索引 |
| ○ [S-DB-03](interview/middleware/mysql/S-DB-03-slow-query.md) | 慢查询 |

---

## 7 天冲刺计划

| 天 | 主题 | 题量 |
|----|------|------|
| D1 | 链上索引 + 事件幂等 + RabbitMQ | S-BC-05、S-BC-04、S-RAB-01、S-ARCH-04 |
| D2 | 充提钱包 + MPC/TSS | S-EXCH-02、S-BC-10、S-BC-03 |
| D3 | K 线 + WebSocket 行情 | S-EXCH-10、S-EXCH-11、S-NET-05 |
| D4 | Token 发行 + AMM + 合约升级 | S-EXCH-12、S-EXCH-06、S-SOLID-04 |
| D5 | 账务 + 风控 + GORM | S-EXCH-03、S-EXCH-05、S-DB-05、S-DB-02 |
| D6 | Gin API + 可观测 + 发布 | S-NET-03、S-ARCH-16、S-CLOUD-04、S-CODE-03 |
| D7 | 模拟面：DEX 全链路白板串讲 | S-SOLID-08 + 架构图 |

---

## 口述模板（每题 3 分钟）

1. **业务背景**：你在项目中负责哪块、解决什么问题  
2. **架构决策**：为什么 WebSocket 优先、为什么 MQ 拆链路、为什么 N 确认  
3. **故障案例**：reorg 回滚 / 提现重试 / RPC 抖动 — 怎么发现、怎么修  
4. **指标**：lag、P99、连接数、对账差异率
