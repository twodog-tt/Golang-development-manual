# 专题 ↔ 可运行代码映射

| 文档 ID | 文档 | 代码路径 | 说明 |
|-------|------|----------|------|
| S-CONC-05, S-CONC-06 | Channel | `basis/channel/main.go` | 无缓冲/有缓冲 channel |
| S-CONC-08, S-CONC-11 | Mutex / WaitGroup | `basis/sync/main.go` | Mutex vs atomic |
| S-CONC-01~04, S-CONC-16 | Goroutine | `basis/goroutine/main.go` | WaitGroup、任务调度 |
| S-CONC-17 | Pipeline | `gin-example/example_28/main.go` | errgroup 多服务 |
| S-MEM-07 | interface | `basis/struct/main.go` | 接口与嵌入 |
| S-MEM-05 | slice | `basis/point/main.go` | 指针与 slice 引用 |
| S-DB-05 | GORM | `gorm/demo/main.go` | 见 [middleware/mysql/S-DB-05](../middleware/mysql/S-DB-05-gorm-pitfalls.md) |
| S-DIST-01～03 | Redis | — | [middleware/redis/](../middleware/redis/index.md) |
| S-DIST-04 | Kafka 消费语义 | — | [middleware/kafka/](../middleware/kafka/index.md) |
| S-KAFKA-01～03 | Kafka 架构/Producer/交易总线 | — | [middleware/kafka/](../middleware/kafka/index.md) |
| S-RMQ-01～04 | RocketMQ | — | [middleware/rocketmq/](../middleware/rocketmq/index.md) |
| S-RAB-01 | RabbitMQ 交易所异步 | — | [middleware/rabbitmq/](../middleware/rabbitmq/index.md) |
| S-ES-01～03 | Elasticsearch | — | [middleware/elasticsearch/](../middleware/elasticsearch/index.md) |
| S-DIST-05 | 分布式事务 | — | [middleware/distributed/](../middleware/distributed/index.md) |
| S-DB-05 | sqlx | `gorm/sqlx/sqlx1/main.go`, `sqlx2/main.go` | 原生 SQL |
| S-NET-03 | Gin 校验 | `gin-example/example_12/main.go` | 自定义 validator |
| S-NET-03 | Gin 绑定 | `gin-example/example_3/main.go` | 嵌套结构体绑定 |
| S-NET-03 | Gin JSON | `gin-example/example_1/main.go` | AsciiJSON |
| S-CODE-01 | LRU | [S-CODE-01](../08-coding-senior/S-CODE-01-concurrent-lru.md) | `examples/senior/lru/` |
| S-CODE-02 | 令牌桶 | [S-CODE-02](../08-coding-senior/S-CODE-02-token-bucket.md) | `examples/senior/ratelimit/` |
| S-CODE-03 | 优雅关闭 | [S-CODE-03](../08-coding-senior/S-CODE-03-graceful-shutdown.md) | `examples/senior/graceful_shutdown/` |
| S-CLOUD-04 | 滚动发布与探针 | [S-CLOUD-04](../09-cloud-native/S-CLOUD-04-rolling-update-probes-pdb.md) | `examples/senior/graceful_shutdown/` |
| S-CLOUD-01～08 | 云原生 K8s/Docker | — | [09-cloud-native/](../09-cloud-native/index.md) |
| S-CODE-04 | errgroup | [S-CODE-04](../08-coding-senior/S-CODE-04-errgroup.md) | `examples/senior/errgroup/` |
| S-CODE-05 | 连接池 | [S-CODE-05](../08-coding-senior/S-CODE-05-connection-pool.md) | `examples/senior/connpool/` |
| S-CODE-06 | Singleflight 缓存 | [S-CODE-06](../08-coding-senior/S-CODE-06-singleflight-cache.md) | `examples/senior/singleflightcache/` |
| S-CODE-07 | 有界批处理 | [S-CODE-07](../08-coding-senior/S-CODE-07-bounded-batch-executor.md) | `examples/senior/batchexec/` |
| S-AI-01～06 | AI 工程 | — | [10-ai-engineering/](../10-ai-engineering/index.md) |
| S-AI-01 | 流式 LLM Mock | [S-AI-01](../10-ai-engineering/S-AI-01-llm-api-integration.md) | `examples/senior/llmclient/` |
| S-AI-02 | 简易 RAG | [S-AI-02](../10-ai-engineering/S-AI-02-rag-architecture.md) | `examples/senior/rag/` |
| S-AI-07 | MCP Server | [S-AI-07](../10-ai-engineering/S-AI-07-mcp-server-go.md) | `examples/senior/mcp/` |
| S-SOL-01～08 | 解决方案架构 | — | [11-solution-architecture/](../11-solution-architecture/index.md) |
| S-BC-01～12 | 区块链 Web3 | — | [12-blockchain-web3/](../12-blockchain-web3/index.md) |
| S-BC-02 | JSON-RPC 客户端 | [S-BC-02](../12-blockchain-web3/S-BC-02-go-ethereum-rpc.md) | `examples/senior/ethrpc/` |
| S-BC-09 | abigen ERC20 实战 | [S-BC-09](../12-blockchain-web3/S-BC-09-abigen-contract-bindings.md) | `examples/senior/erc20bind/` |
| S-BC-12 | 跨链应用层 Guard | [S-BC-12](../12-blockchain-web3/S-BC-12-cross-chain-message-bridge-security.md) | `examples/senior/bridgeguard/` |
| S-GOENG-01～06 | Go 生产工程 | — | [16-go-production-engineering/](../16-go-production-engineering/index.md) |
| S-WALLET-01～11 | 多链钱包与托管 | — | [17-multichain-wallet/](../17-multichain-wallet/index.md) |
| S-WALLET-02 | Bitcoin Coin Selection | [S-WALLET-02](../17-multichain-wallet/S-WALLET-02-bitcoin-utxo-psbt-fee-bump.md) | `examples/senior/coinselect/` |
| S-WALLET-08 | Solana 离线交易 | [S-WALLET-08](../17-multichain-wallet/S-WALLET-08-solana-go-sdk-transaction.md) | `examples/non-evm-sdk/solana/` |
| S-WALLET-09 | Cosmos DIRECT 签名 | [S-WALLET-09](../17-multichain-wallet/S-WALLET-09-cosmos-go-sdk-sign-mode-direct.md) | `examples/non-evm-sdk/cosmos/` |
| S-WALLET-10 | Aptos BCS 交易 | [S-WALLET-10](../17-multichain-wallet/S-WALLET-10-aptos-go-sdk-bcs-transaction.md) | `examples/non-evm-sdk/aptos/` |
| S-WALLET-11 | Sui 能力与预占 | [S-WALLET-11](../17-multichain-wallet/S-WALLET-11-sui-go-capability-adapter.md) | `examples/non-evm-sdk/sui/` |
| S-PAY-01～06 | Web3 支付与稳定币 | — | [18-web3-payments-stablecoin/](../18-web3-payments-stablecoin/index.md) |
| S-PAY-01 | 支付状态机 | [S-PAY-01](../18-web3-payments-stablecoin/S-PAY-01-payment-state-idempotency-reversal.md) | `examples/senior/paymentstate/` |
| S-NODE-01～09 | 节点、RPC 与 Staking | — | [19-node-rpc-staking/](../19-node-rpc-staking/index.md) |
| S-NODE-02 | Hedged RPC Pool | [S-NODE-02](../19-node-rpc-staking/S-NODE-02-rpc-ha-quorum-hedging-cache.md) | `examples/senior/rpcpool/` |
| S-NODE-07 | Canonical Chain Merge | [S-NODE-07](../19-node-rpc-staking/S-NODE-07-canonical-backfill-realtime-merge.md) | `examples/senior/chainmerge/` |
| S-NODE-09 | 非 EVM 生命周期故障注入 | [S-NODE-09](../19-node-rpc-staking/S-NODE-09-non-evm-online-sdk-fault-injection.md) | `examples/senior/txlifecycle/` |
| S-SOLID-01～08 | Solidity 合约 | — | [13-solidity-contracts/](../13-solidity-contracts/index.md) |
| S-SOLID-02 | 重入防护合约 | [S-SOLID-02](../13-solidity-contracts/S-SOLID-02-security-reentrancy.md) | `examples/solidity/ReentrancyGuard.sol` |
| S-PROTO-01～04 | 协议、共识与安全 | — | [20-protocol-consensus-security/](../20-protocol-consensus-security/index.md) |
| S-SEC-01～04 | Web3 安全工程 | — | [21-security-engineering/](../21-security-engineering/index.md) |
| S-SEC-02 | Signer Fencing | [S-SEC-02](../21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | `examples/senior/signerfencing/` |
| S-SEC-02 | Durable Fence + HSM/MPC | [S-SEC-02](../21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | `examples/signer-project/` |
| S-EXCH-01～31 | DEX / CEX 交易所 | — | [14-dex-cex-engineering/](../14-dex-cex-engineering/index.md) |
| S-EXCH-17 | 确定性撮合引擎 | [S-EXCH-17](../14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md) | `examples/senior/matchingengine/` |
| S-EXCH-18 | WAL、快照与回放 | [S-EXCH-18](../14-dex-cex-engineering/S-EXCH-18-wal-snapshot-replay.md) | `examples/senior/walreplay/` |
| S-EXCH-19 | 行情 Gap Recovery | [S-EXCH-19](../14-dex-cex-engineering/S-EXCH-19-market-data-sequence-gap-recovery.md) | `examples/senior/marketdatarecovery/` |
| S-EXCH-20 | FIX Session 恢复 | [S-EXCH-20](../14-dex-cex-engineering/S-EXCH-20-fix-session-sequence-recovery.md) | `examples/senior/fixsession/` |
| S-EXCH-21 | STP 自成交防护 | [S-EXCH-21](../14-dex-cex-engineering/S-EXCH-21-self-trade-prevention-surveillance.md) | `examples/senior/matchingengine/` |
| S-EXCH-22 | 集合竞价与 Benchmark | [S-EXCH-22](../14-dex-cex-engineering/S-EXCH-22-call-auction-performance-validation.md) | `examples/senior/callauction/`, `examples/senior/matchingengine/` |
| S-EXCH-28 | 多级代理极差分润 | [S-EXCH-28](../14-dex-cex-engineering/S-EXCH-28-affiliate-tiered-rate-rebate.md) | — |
| S-EXCH-29 | DeFi Staking / LM / Farm | [S-EXCH-29](../14-dex-cex-engineering/S-EXCH-29-defi-staking-liquidity-mining-yield.md) | — |
| S-EXCH-30 | Uniswap V2/V3 协议 | [S-EXCH-30](../14-dex-cex-engineering/S-EXCH-30-uniswap-v2-v3-protocol.md) | — |
| S-EXCH-31 | DEX Tech Lead 白板 | [S-EXCH-31](../14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md) | — |
| S-MSVC-01～06 | 微服务（交易所场景） | — | [15-microservices-exchange/](../15-microservices-exchange/index.md) |
| — | 算法练习 | `algorithm/lc_*` | LeetCode 参考实现 |

## 使用方式

1. 阅读 `docs/topics/` 下对应 Markdown。
2. 按上表进入代码目录：`go run .` 或 `go test`。
3. 建议：**先讲清不变量再对照代码**，并补充自己的生产案例。
