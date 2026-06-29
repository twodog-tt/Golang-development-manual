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
| S-AI-01～06 | AI 工程 | — | [10-ai-engineering/](../10-ai-engineering/index.md) |
| S-AI-01 | 流式 LLM Mock | [S-AI-01](../10-ai-engineering/S-AI-01-llm-api-integration.md) | `examples/senior/llmclient/` |
| S-AI-02 | 简易 RAG | [S-AI-02](../10-ai-engineering/S-AI-02-rag-architecture.md) | `examples/senior/rag/` |
| S-AI-07 | MCP Server | [S-AI-07](../10-ai-engineering/S-AI-07-mcp-server-go.md) | `examples/senior/mcp/` |
| S-SOL-01～08 | 解决方案架构 | — | [11-solution-architecture/](../11-solution-architecture/index.md) |
| S-BC-01～10 | 区块链 Web3 | — | [12-blockchain-web3/](../12-blockchain-web3/index.md) |
| S-BC-02 | JSON-RPC 客户端 | [S-BC-02](../12-blockchain-web3/S-BC-02-go-ethereum-rpc.md) | `examples/senior/ethrpc/` |
| S-BC-09 | abigen ERC20 实战 | [S-BC-09](../12-blockchain-web3/S-BC-09-abigen-contract-bindings.md) | `examples/senior/erc20bind/` |
| S-SOLID-01～08 | Solidity 合约 | — | [13-solidity-contracts/](../13-solidity-contracts/index.md) |
| S-SOLID-02 | 重入防护合约 | [S-SOLID-02](../13-solidity-contracts/S-SOLID-02-security-reentrancy.md) | `examples/solidity/ReentrancyGuard.sol` |
| S-EXCH-01～12 | DEX / CEX 交易所 | — | [14-dex-cex-engineering/](../14-dex-cex-engineering/index.md) |
| — | 算法面 | `algorithm/lc_*` | LeetCode 参考实现 |

## 使用方式

1. 阅读 `docs/interview/` 下对应 Markdown。
2. 按上表进入代码目录：`go run .` 或 `go test`。
3. 资深面建议：**先口述再对照代码**，并补充自己的生产案例。
