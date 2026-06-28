# Go 后端与区块链架构师面试手册

面向 **5 年+ Go 后端 + 区块链/Web3 架构师** 的面试知识库（**134 篇正文**）。

> **定位**：Go 运行时与系统设计 + 链上工程（Solidity）+ 链下工程（Go RPC/索引）+ 解决方案架构 + AI 工程。

> **如何使用左侧导航**：点击分组标题可展开/折叠子目录；当前所在分组会自动展开。

## 推荐刷题顺序

1. [学习路线](learning-path-senior.md) — 后端 / 架构师 / Web3 分轨
2. **Go 核心（P0）** — 并发 → 内存 → 系统设计
3. **中间件与数据库** — MySQL、Redis、MQ、ES
4. [网络与服务治理](interview/06-network-governance/) — gRPC、Gin、JWT
5. [AI 工程与编程](interview/10-ai-engineering/) — LLM、RAG、MCP
6. [解决方案架构](interview/11-solution-architecture/) — DDD、演进、45min 白板
7. [区块链与 Web3（Go）](interview/12-blockchain-web3/) — RPC、索引、L2、4337
8. [**Solidity 与合约工程**](interview/13-solidity-contracts/) — 安全、ERC、Proxy、DeFi
9. [**DEX / CEX 交易所工程**](interview/14-dex-cex-engineering/) — 撮合、账务、AMM、MEV
10. [手写题](interview/08-coding-senior/) + **工程与软技能**

## Web3 架构师速查

| 链上（13） | 链下（12） | 交易所（14） |
|------------|------------|--------------|
| Solidity、ERC、升级、审计 | RPC、索引、签名、abigen | CEX 撮合/账务、DEX AMM/MEV |
| [13-solidity-contracts](interview/13-solidity-contracts/) | [12-blockchain-web3](interview/12-blockchain-web3/) | [14-dex-cex-engineering](interview/14-dex-cex-engineering/) |

## 中间件速查

| 类型 | 题数 | 入口 |
|------|------|------|
| [MySQL + GORM](interview/middleware/mysql/) | 5 | 索引、MVCC、慢查询、分库分表 |
| [Redis](interview/middleware/redis/) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](interview/middleware/kafka/) | 1 | 消费语义 |
| [RocketMQ](interview/middleware/rocketmq/) | 3 | 架构、事务/顺序/延迟 |
| [Elasticsearch](interview/middleware/elasticsearch/) | 3 | 倒排索引、DSL、同步 |
| [分布式事务](interview/middleware/distributed/) | 1 | TCC / Saga |

## 其他链接

- [面试题总索引](interview/README.md)
- [题单 YAML](interview/_meta/questions.yaml)
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)

## 可运行代码

`basis/` · `gin-example/` · `gorm/` · `algorithm/` · `examples/senior/` · `examples/solidity/`
