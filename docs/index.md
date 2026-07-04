# Go 后端与区块链架构师面试手册

面向 **5 年+ Go 后端 + 区块链/Web3 架构师** 的面试知识库（**153 篇正文**）。

> **定位**：Go 运行时与系统设计 + 链上工程（Solidity）+ 链下工程（Go RPC/索引）+ 解决方案架构 + AI 工程。
>
> **⭐ Web3 交易所 / 钱包方向**：[重点准备题单](resume-focus-web3.md)

> **如何使用左侧导航**：菜单为 **三级结构**（分组 → 模块 → 题目 ID）；按 **基础 → 进阶 → 高阶 → 专题** 排列，点击分组可展开子目录。

## 推荐刷题顺序（由易到难）

| 层级 | 模块 | 说明 |
|------|------|------|
| **基础** | [01 并发](interview/01-runtime-concurrency/index.md) → [02 内存](interview/02-memory-gc/index.md) → [08 手写题](interview/08-coding-senior/index.md) | Go 语言深度 + 编码 |
| **进阶** | [06 网络](interview/06-network-governance/index.md) → [中间件](interview/middleware/index.md) | API、gRPC、DB/缓存/MQ |
| **高阶** | [03 系统设计](interview/03-system-design/index.md) → [09 云原生](interview/09-cloud-native/index.md) → [11 解决方案架构](interview/11-solution-architecture/index.md) → [15 微服务](interview/15-microservices-exchange/index.md) | 架构白板、K8s、服务治理 |
| **专题** | [12 Web3](interview/12-blockchain-web3/index.md) → [13 Solidity](interview/13-solidity-contracts/index.md) → [14 交易所](interview/14-dex-cex-engineering/index.md) | 链上链下 + 交易业务 |
| **综合** | [07 领导力](interview/07-engineering-leadership/index.md) · [10 AI](interview/10-ai-engineering/index.md) | 软技能与 AI 专项 |

快捷入口：[学习路线](learning-path-senior.md) · [**模拟面试**](mock-interview.md) · [Web3 重点题单](resume-focus-web3.md)

## Web3 架构师速查

| 链上（13） | 链下（12） | 交易所（14） |
|------------|------------|--------------|
| Solidity、ERC、升级、审计 | RPC、索引、签名、abigen | CEX 撮合/账务、DEX AMM/MEV |
| [13-solidity-contracts](interview/13-solidity-contracts/index.md) | [12-blockchain-web3](interview/12-blockchain-web3/index.md) | [14-dex-cex-engineering](interview/14-dex-cex-engineering/index.md) |

## 中间件速查

| 类型 | 题数 | 入口 |
|------|------|------|
| [MySQL + GORM](interview/middleware/mysql/index.md) | 5 | 索引、MVCC、慢查询、分库分表 |
| [Redis](interview/middleware/redis/index.md) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](interview/middleware/kafka/index.md) | 4 | 架构、Producer、消费语义 |
| [RocketMQ](interview/middleware/rocketmq/index.md) | 4 | 架构、事务/顺序/延迟、排障 |
| [Elasticsearch](interview/middleware/elasticsearch/index.md) | 3 | 倒排索引、DSL、同步 |
| [分布式事务](interview/middleware/distributed/index.md) | 1 | TCC / Saga |

## 其他链接

- [Web3 交易所重点准备](resume-focus-web3.md)
- [面试题总索引](interview-catalog.md)
- [题单 YAML](interview/_meta/questions.yaml)
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)

## 可运行代码

`basis/` · `gin-example/` · `gorm/` · `algorithm/` · `examples/senior/` · `examples/solidity/`
