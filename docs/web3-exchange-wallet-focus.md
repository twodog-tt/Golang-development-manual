# Web3 交易所与钱包重点专题

> 本页是 `exchange_engineering` 的场景化学习投影，不再定义一套全局 P0/P1/P2。
> 正式优先级和每篇正文的证据边界以
> [角色化优先级与证据标签](interview/_meta/role-priority-matrix.md) 为准。

**图例**：⭐ **核心主线**（先形成闭环） · 🔶 **延展强化**（架构深挖） ·
○ **基础回查**（按反馈补洞）

证据标签不在本页重复维护，避免两处数据漂移。对外表述时按
`explanation_only → illustrative_artifact → deterministic_test → integration_harness`
说明仓库实际能证明到哪一层；当前没有 `external_acceptance`，不得把本地 harness
表述成生产或真实厂商验收。

---

## 按技术场景速查

| 常见场景 | 对应专题 |
|----------|----------|
| Go 工程、测试、Linux/TCP、SQL、编码练习 | [16 Go 生产工程](interview/16-go-production-engineering/index.md) · [08 编码练习](interview/08-coding-senior/index.md) · [06 网络](interview/06-network-governance/index.md) |
| BTC/Solana/Cosmos/Sui/Aptos、多链充提、SDK 交易向量、MPC | [17 多链钱包与托管](interview/17-multichain-wallet/index.md) |
| 稳定币支付、Treasury、双分录、清结算、合规 | [18 Web3 支付与稳定币](interview/18-web3-payments-stablecoin/index.md) |
| EL/CL、RPC HA、Indexer、Relayer、Validator | [19 节点、RPC 与 Staking](interview/19-node-rpc-staking/index.md) |
| PoS/BFT、fork choice、PeerDAS、协议升级与迁移 | [20 协议、共识与安全](interview/20-protocol-consensus-security/index.md) |
| Threat model、signer fencing、SBOM/provenance、安全事件响应 | [21 Web3 安全工程](interview/21-security-engineering/index.md) |
| 机构托管、DvP、RWA、ISO 20022 与 settlement evidence | [S-PAY-06](interview/18-web3-payments-stablecoin/S-PAY-06-institutional-custody-rwa-iso20022.md) |
| 预测市场 CTF、CLOB/EIP-712、预言机争议与主网上线 | [S-EXCH-23~26](interview/14-dex-cex-engineering/index.md) |
| Rollup/跨链安全、撮合/WAL、行情/FIX/STP/竞价与完整架构白板 | [12 Web3](interview/12-blockchain-web3/index.md) · [13 Solidity](interview/13-solidity-contracts/index.md) · [14 DEX/CEX](interview/14-dex-cex-engineering/index.md) |
| 微服务拆分、gRPC 治理、Database per Service、网关 BFF、事件总线 | [15 微服务（交易所）](interview/15-microservices-exchange/index.md) |
| 多链充提、reorg、MPC/TSS、热冷钱包、提现风控 | [17 多链钱包](interview/17-multichain-wallet/index.md)、[S-EXCH-02](interview/14-dex-cex-engineering/S-EXCH-02-deposit-withdraw-wallet.md) |
| 实时风控、ES、MQ、限流熔断 | [S-ARCH-08](interview/03-system-design/S-ARCH-08-rate-limiting.md)、[S-ES 系列](interview/middleware/elasticsearch/index.md) |
| PostgreSQL 资金事务、VACUUM、WAL 与 HA | [PostgreSQL 专题](interview/middleware/postgresql/index.md) |
| Terraform 状态/漂移、安全变更，Helm/GitOps 发布回滚 | [09 云原生与 SRE](interview/09-cloud-native/index.md) |
| ClickHouse 链数据、reorg、回填与 lakehouse replay | [S-NODE-10](interview/19-node-rpc-staking/S-NODE-10-chain-data-clickhouse-lakehouse.md) |
| Staff 技术战略、跨团队迁移与影响力证据 | [07 工程效能与技术领导力](interview/07-engineering-leadership/index.md) |

---

## 核心主线（交易所角色）

优先按 **Go 工程门槛 → 多链钱包 → 支付/账本 → 节点/RPC → 交易所与合约** 讲解，每篇准备 **1 个生产案例**。

### Go 生产工程与基础门槛

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-GOENG-01](interview/16-go-production-engineering/S-GOENG-01-errors-contract-panic-boundary.md) | 错误契约与 Panic 边界 | `Is/As`、wrap、recover fail closed |
| ⭐ [S-GOENG-03](interview/16-go-production-engineering/S-GOENG-03-testing-table-fake.md) | 单元测试与 Test Double | table、fake、确定性 |
| ⭐ [S-GOENG-04](interview/16-go-production-engineering/S-GOENG-04-fuzz-benchmark-race.md) | Fuzz/Benchmark/Race | 回归门禁 |
| ⭐ [S-GOENG-05](interview/16-go-production-engineering/S-GOENG-05-modules-toolchain-reproducible.md) | Modules/Toolchain | MVS、go.work、可复现构建 |
| ⭐ [S-CODE-06](interview/08-coding-senior/S-CODE-06-singleflight-cache.md) | Singleflight 缓存 | context、击穿、TTL |
| ⭐ [S-CODE-07](interview/08-coding-senior/S-CODE-07-bounded-batch-executor.md) | 有界批处理 | 背压、取消、保序 |
| ⭐ [S-NET-06](interview/06-network-governance/S-NET-06-linux-fd-epoll-netpoll.md) | Linux/epoll/netpoll | FD、readiness、阻塞 syscall |
| ⭐ [S-NET-07](interview/06-network-governance/S-NET-07-tcp-lifecycle-queues-timewait.md) | TCP 故障排查 | backlog、TIME_WAIT、重传 |
| ⭐ [S-DB-07](interview/middleware/mysql/S-DB-07-financial-schema-locking.md) | 资金表与锁 | DECIMAL、双分录、deadlock |
| ⭐ [S-PG-02](interview/middleware/postgresql/S-PG-02-isolation-locking-ledger.md) | PostgreSQL 资金并发 | 隔离级别、锁、40001、整笔事务重试 |
| ⭐ [S-PG-03](interview/middleware/postgresql/S-PG-03-wal-replication-pgx-ha.md) | PostgreSQL WAL 与 HA | commit evidence、复制、failover、pgx |

### 多链钱包、支付与节点基础设施

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-WALLET-01](interview/17-multichain-wallet/S-WALLET-01-chain-adapter-capability-matrix.md) | Chain Adapter | 能力矩阵、错误抽象 |
| ⭐ [S-WALLET-02](interview/17-multichain-wallet/S-WALLET-02-bitcoin-utxo-psbt-fee-bump.md) | Bitcoin UTXO/PSBT | Coin selection、RBF/CPFP |
| ⭐ [S-WALLET-03](interview/17-multichain-wallet/S-WALLET-03-solana-account-pda-transaction.md) | Solana | PDA、blockhash、commitment |
| ⭐ [S-WALLET-06](interview/17-multichain-wallet/S-WALLET-06-deposit-sweep-reservation-recovery.md) | 地址与归集恢复 | nonce/UTXO/object reservation |
| ⭐ [S-WALLET-07](interview/17-multichain-wallet/S-WALLET-07-mpc-dkg-reshare-recovery.md) | MPC 深度 | DKG、reshare、故障恢复 |
| ⭐ [S-WALLET-08](interview/17-multichain-wallet/S-WALLET-08-solana-go-sdk-transaction.md) | Solana Go 实战 | 社区 SDK、blockhash、commitment |
| ⭐ [S-WALLET-09](interview/17-multichain-wallet/S-WALLET-09-cosmos-go-sdk-sign-mode-direct.md) | Cosmos Go 实战 | TxBuilder、DIRECT、sequence |
| ⭐ [S-WALLET-10](interview/17-multichain-wallet/S-WALLET-10-aptos-go-sdk-bcs-transaction.md) | Aptos Go 实战 | 官方 SDK、BCS、VM status |
| ⭐ [S-WALLET-11](interview/17-multichain-wallet/S-WALLET-11-sui-go-capability-adapter.md) | Sui Go 能力适配 | Object、Address Balance、gasless |
| ⭐ [S-PAY-01](interview/18-web3-payments-stablecoin/S-PAY-01-payment-state-idempotency-reversal.md) | 支付状态机 | Webhook、冲正、乱序 |
| ⭐ [S-PAY-04](interview/18-web3-payments-stablecoin/S-PAY-04-ledger-clearing-settlement-reconciliation.md) | 清结算与对账 | 双分录、三方对账 |
| ⭐ [S-PAY-05](interview/18-web3-payments-stablecoin/S-PAY-05-compliance-travel-rule-sanctions.md) | 合规架构 | KYC/KYB、Travel Rule、制裁 |
| ⭐ [S-PAY-06](interview/18-web3-payments-stablecoin/S-PAY-06-institutional-custody-rwa-iso20022.md) | 机构金融 | custody、DvP、RWA、ISO 20022 |
| ⭐ [S-NODE-01](interview/19-node-rpc-staking/S-NODE-01-ethereum-node-architecture-sync.md) | Ethereum 节点 | EL/CL、full/archive |
| ⭐ [S-NODE-02](interview/19-node-rpc-staking/S-NODE-02-rpc-ha-quorum-hedging-cache.md) | RPC HA | quorum、hedging、cache |
| ⭐ [S-NODE-04](interview/19-node-rpc-staking/S-NODE-04-chain-data-platform.md) | 链上数据平台 | backfill、trace、schema |
| ⭐ [S-NODE-05](interview/19-node-rpc-staking/S-NODE-05-relayer-transaction-manager.md) | Relayer/Tx Manager | nonce、fee、replacement |
| ⭐ [S-NODE-07](interview/19-node-rpc-staking/S-NODE-07-canonical-backfill-realtime-merge.md) | Canonical Merge | hash lineage、overlap、finalized guard |
| ⭐ [S-NODE-09](interview/19-node-rpc-staking/S-NODE-09-non-evm-online-sdk-fault-injection.md) | 非 EVM 在线可靠性 | UNKNOWN、同 bytes 重播、expiry |

### 链上索引与 Web3 Go

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-BC-05](interview/12-blockchain-web3/S-BC-05-indexer-reorg.md) | 链上索引器：扫块、重组与幂等 | block hash lineage、observation 与业务幂等 |
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
| ⭐ [S-EXCH-30](interview/14-dex-cex-engineering/S-EXCH-30-uniswap-v2-v3-protocol.md#oral-card) | Uniswap V2/V3 协议深挖 | Factory/Router、sqrtPrice、tick、假池校验 |
| ⭐ [S-EXCH-27](interview/14-dex-cex-engineering/S-EXCH-27-pancakeswap-v2-v3-differences.md#oral-card) | PancakeSwap V2/V3 | 集中流动性、fee tier、position NFT、迁池与索引差异 |
| ⭐ [S-EXCH-29](interview/14-dex-cex-engineering/S-EXCH-29-defi-staking-liquidity-mining-yield.md#oral-card) | Staking / 挖矿 / Farming | rewardPerToken、排放、防刷、APR 口径 |
| ⭐ [S-EXCH-10](interview/14-dex-cex-engineering/S-EXCH-10-kline-event-aggregation.md) | 链上事件驱动 K 线 | K 线、排行榜 |
| ⭐ [S-EXCH-11](interview/14-dex-cex-engineering/S-EXCH-11-websocket-market-hub.md) | WebSocket 行情 Hub | 实时推送 |
| ⭐ [S-EXCH-12](interview/14-dex-cex-engineering/S-EXCH-12-token-launch-rebate.md) | Token 发行与返佣提现 | 毕业、分账、提现 |
| ⭐ [S-EXCH-28](interview/14-dex-cex-engineering/S-EXCH-28-affiliate-tiered-rate-rebate.md#oral-card) | 多级代理极差分润 | 代理树、极差费率、计佣账本、后台隔离 |
| ⭐ [S-EXCH-17](interview/14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md) | 可运行确定性撮合 | 价格时间优先、FOK、Post-only、STP、重放 |
| ⭐ [S-EXCH-18](interview/14-dex-cex-engineering/S-EXCH-18-wal-snapshot-replay.md) | WAL、快照与回放 | durable-before-apply、torn tail、发布水位 |
| ⭐ [S-EXCH-19](interview/14-dex-cex-engineering/S-EXCH-19-market-data-sequence-gap-recovery.md) | 行情 Gap Recovery | snapshot bridge、sequence、fail closed |
| ⭐ [S-EXCH-20](interview/14-dex-cex-engineering/S-EXCH-20-fix-session-sequence-recovery.md) | FIX Session | Resend、PossDup、Gap Fill |
| ⭐ [S-EXCH-21](interview/14-dex-cex-engineering/S-EXCH-21-self-trade-prevention-surveillance.md) | STP 自成交防护 | scope、cancel policy、surveillance |
| ⭐ [S-EXCH-22](interview/14-dex-cex-engineering/S-EXCH-22-call-auction-performance-validation.md) | 集合竞价与性能验证 | clearing price、分配、benchstat |
| ⭐ [S-EXCH-23](interview/14-dex-cex-engineering/S-EXCH-23-prediction-market-ctf-lifecycle.md#oral-card) | 预测市场 CTF 与生命周期 | condition/position、split/merge/redeem、规则冻结 |
| ⭐ [S-EXCH-24](interview/14-dex-cex-engineering/S-EXCH-24-prediction-market-clob-eip712-settlement.md#oral-card) | CLOB 与链上结算 | EIP-712、防重放、取消竞态、mint/merge |
| ⭐ [S-EXCH-25](interview/14-dex-cex-engineering/S-EXCH-25-prediction-market-oracle-dispute-resolution.md#oral-card) | 预言机与争议仲裁 | 体育/电竞 feed、bond、liveness、source conflict |
| ⭐ [S-EXCH-26](interview/14-dex-cex-engineering/S-EXCH-26-prediction-market-security-testing-mainnet.md#oral-card) | 安全与主网上线 | 资金不变量、fuzz/invariant、审计、canary |

### 完整架构白板（架构师 / 终面）

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-EXCH-13](interview/14-dex-cex-engineering/S-EXCH-13-cex-end-to-end-architecture.md) | CEX 端到端交易系统 | 撮合·账务·充提·行情·45min 白板 |
| ⭐ [S-EXCH-14](interview/14-dex-cex-engineering/S-EXCH-14-web3-exchange-fullstack-architecture.md) | Web3 交易所全栈 | Indexer·K线·WS·返佣·链上链下边界 |
| ⭐ [S-EXCH-31](interview/14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md#oral-card) | DEX Tech Lead 45min 白板 | 协议权威·激励·多链门禁·带队与取舍 |
| ⭐ [S-EXCH-15](interview/14-dex-cex-engineering/S-EXCH-15-settlement-ha-disaster-recovery.md) | 清结算与 HA | 三层对账·T+0·RPO/RTO·提现熔断 |
| ⭐ [S-EXCH-16](interview/14-dex-cex-engineering/S-EXCH-16-perpetual-matching-position.md) | 永续撮合与仓位引擎 | 单向/双向持仓·Reduce-Only·撮合与仓位顺序边界 |

### 微服务（交易所场景）

| 题 ID | 题目 | 关键技术点 |
|-------|------|------------|
| ⭐ [S-MSVC-01](interview/15-microservices-exchange/S-MSVC-01-exchange-microservices-whiteboard.md) | 交易所微服务全链路白板 | CEX+DEX 服务边界·同步/异步选型 |
| ⭐ [S-MSVC-02](interview/15-microservices-exchange/S-MSVC-02-domain-decomposition.md) | 域拆分与限界上下文 | order/matching/ledger/wallet/indexer |
| ⭐ [S-MSVC-03](interview/15-microservices-exchange/S-MSVC-03-discovery-grpc-governance.md) | 服务发现与 gRPC 治理 | K8s DNS·超时·幂等重试 |
| ⭐ [S-MSVC-04](interview/15-microservices-exchange/S-MSVC-04-database-per-service.md) | Database per Service | Outbox·Saga·成交入账 |
| ⭐ [S-MSVC-05](interview/15-microservices-exchange/S-MSVC-05-gateway-bff-traffic.md) | 网关 BFF 与流量治理 | 限流维度·渠道隔离 |
| ⭐ [S-MSVC-06](interview/15-microservices-exchange/S-MSVC-06-event-bus-async-boundary.md) | 事件总线与异步边界 | Topic 划分·trade.matched·chain.swap |

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

## 延展强化

| 题 ID | 题目 |
|-------|------|
| 🔶 [S-EXCH-01](interview/14-dex-cex-engineering/S-EXCH-01-cex-matching-engine.md) | CEX 撮合引擎 |
| 🔶 [S-EXCH-16](interview/14-dex-cex-engineering/S-EXCH-16-perpetual-matching-position.md) | 永续撮合与仓位引擎 |
| 🔶 [S-EXCH-07](interview/14-dex-cex-engineering/S-EXCH-07-aggregator-slippage.md) | 聚合路由与滑点 |
| 🔶 [S-EXCH-08](interview/14-dex-cex-engineering/S-EXCH-08-mev-sandwich.md) | MEV 与三明治 |
| 🔶 [S-EXCH-09](interview/14-dex-cex-engineering/S-EXCH-09-hybrid-cex-dex.md) | CeDeFi 混合 |
| 🔶 [S-BC-06](interview/12-blockchain-web3/S-BC-06-defi-backend-patterns.md) | DeFi 后端模式 |
| 🔶 [S-BC-07](interview/12-blockchain-web3/S-BC-07-l2-cross-chain-bridge.md) | L2 与跨链 |
| 🔶 [S-BC-11](interview/12-blockchain-web3/S-BC-11-rollup-finality-da-proof-security.md) | Rollup finality、DA 与证明边界 |
| 🔶 [S-BC-12](interview/12-blockchain-web3/S-BC-12-cross-chain-message-bridge-security.md) | 跨链消息认证、重放与敞口限制 |
| 🔶 [S-PROTO-01](interview/20-protocol-consensus-security/S-PROTO-01-ethereum-pos-fork-choice-finality.md) | Ethereum PoS、Fork Choice 与弱主观性 |
| 🔶 [S-PROTO-02](interview/20-protocol-consensus-security/S-PROTO-02-bft-cometbft-round-lock-safety-liveness.md) | BFT / CometBFT 轮次与锁 |
| 🔶 [S-PROTO-03](interview/20-protocol-consensus-security/S-PROTO-03-blob-da-peerdas-security.md) | Blob、DA 与 PeerDAS |
| 🔶 [S-PROTO-04](interview/20-protocol-consensus-security/S-PROTO-04-protocol-upgrade-state-migration.md) | 协议升级与状态迁移 |
| 🔶 [S-NODE-08](interview/19-node-rpc-staking/S-NODE-08-trace-state-diff-versioned-decoder-quality.md) | Trace、State Diff、Decoder 与数据质量 |
| 🔶 [S-NODE-10](interview/19-node-rpc-staking/S-NODE-10-chain-data-clickhouse-lakehouse.md) | ClickHouse 链数据、Reorg 与 Lakehouse 分层 |
| 🔶 [S-SEC-01](interview/21-security-engineering/S-SEC-01-web3-threat-model-iam-trust-boundaries.md) | Threat Modeling、IAM 与信任边界 |
| 🔶 [S-SEC-02](interview/21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | Key Ceremony、Signer Fencing 与恢复 |
| 🔶 [S-SEC-03](interview/21-security-engineering/S-SEC-03-sbom-provenance-release-admission.md) | SBOM、SLSA Provenance 与发布准入 |
| 🔶 [S-SEC-04](interview/21-security-engineering/S-SEC-04-security-testing-incident-response.md) | 安全测试、故障注入与事件响应 |
| 🔶 [S-SOLID-03](interview/13-solidity-contracts/S-SOLID-03-erc-standards.md) | ERC 标准 |
| 🔶 [S-SOLID-07](interview/13-solidity-contracts/S-SOLID-07-defi-patterns.md) | DeFi 合约模式 |
| 🔶 [S-ARCH-08](interview/03-system-design/S-ARCH-08-rate-limiting.md) | 限流 |
| 🔶 [S-ARCH-09](interview/03-system-design/S-ARCH-09-circuit-breaker.md) | 熔断 |
| 🔶 [S-ARCH-16](interview/03-system-design/S-ARCH-16-observability.md) | 可观测性 |
| 🔶 [S-ARCH-15](interview/03-system-design/S-ARCH-15-release-strategy.md) | 灰度发布 |
| 🔶 [S-RMQ-02](interview/middleware/rocketmq/S-RMQ-02-order-transaction-delay.md) | RocketMQ 事务/顺序 |
| 🔶 [S-RMQ-04](interview/middleware/rocketmq/S-RMQ-04-ops-troubleshooting.md) | RocketMQ 堆积与死信排障 |
| 🔶 [S-DIST-04](interview/middleware/kafka/S-DIST-04-kafka-semantics.md) | Kafka 消费语义 |
| 🔶 [S-KAFKA-01](interview/middleware/kafka/S-KAFKA-01-architecture-storage.md) | Kafka 架构与 ISR |
| 🔶 [S-KAFKA-02](interview/middleware/kafka/S-KAFKA-02-producer-reliability.md) | Producer acks 与幂等 |
| 🔶 [S-KAFKA-03](interview/middleware/kafka/S-KAFKA-03-trade-event-bus.md) | 交易事件总线与 lag |
| 🔶 [S-ES-01](interview/middleware/elasticsearch/S-ES-01-inverted-index.md) | ES 倒排索引 |
| 🔶 [S-ES-03](interview/middleware/elasticsearch/S-ES-03-sync-ops.md) | ES 数据同步 |
| 🔶 [S-CLOUD-04](interview/09-cloud-native/S-CLOUD-04-rolling-update-probes-pdb.md) | 滚动发布与探针 |
| 🔶 [S-CLOUD-09](interview/09-cloud-native/S-CLOUD-09-terraform-state-drift-safe-change.md) | Terraform State、Drift 与安全变更 |
| 🔶 [S-CLOUD-10](interview/09-cloud-native/S-CLOUD-10-helm-gitops-rollout-rollback.md) | Helm、GitOps、渐进发布与回滚边界 |
| 🔶 [S-LEAD-01](interview/07-engineering-leadership/S-LEAD-01-incident-postmortem.md) | 事故复盘 |
| 🔶 [S-LEAD-04](interview/07-engineering-leadership/S-LEAD-04-staff-strategy-influence-case.md) | Staff 技术战略与无授权影响力 |
| 🔶 [S-LEAD-05](interview/07-engineering-leadership/S-LEAD-05-cross-team-migration-case.md) | 跨团队迁移、治理与量化结果 |
| 🔶 [S-PG-01](interview/middleware/postgresql/S-PG-01-mvcc-vacuum-indexes.md) | PostgreSQL MVCC、VACUUM 与索引 |
| 🔶 [S-NET-04](interview/06-network-governance/S-NET-04-jwt-auth.md) | JWT 鉴权 |

---

## 基础回查（12 题）

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
| D1 | Go 工程门槛 | S-GOENG-01/03/04/05、S-CODE-06/07 |
| D2 | Linux/TCP/资金 SQL 与 PostgreSQL | S-NET-06/07、S-DB-06/07、S-PG-01～03 |
| D3 | 多链钱包与 SDK | S-WALLET-01/02/06/07/08～12 |
| D4 | 支付、账本、机构托管与 RWA | S-PAY-01～06 |
| D5 | 节点、链数据与非 EVM 在线可靠性 | S-NODE-01/02/04～10 |
| D6 | 安全/协议或交易深挖（按岗位选） | S-SEC-01～04 + S-PROTO-01～04 + S-BC-11/12，或 S-EXCH-17～22 |
| D7 | 架构与 Staff 综合演练 | S-CLOUD-09/10 + S-LEAD-04/05；完成交易平台 45min 白板 |

---

## 讲解提纲（每篇 3 分钟）

1. **业务背景**：你在项目中负责哪块、解决什么问题  
2. **架构决策**：为什么 WebSocket 优先、为什么 MQ 拆链路、为什么 N 确认  
3. **故障案例**：reorg 回滚 / 提现重试 / RPC 抖动 — 怎么发现、怎么修  
4. **指标**：lag、P99、连接数、对账差异率
