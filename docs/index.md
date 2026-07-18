# Go 后端与区块链架构师面试手册

面向 **5 年+ Go 后端 + 区块链/Web3 架构师** 的面试知识库（**215 篇正文**）。

> **定位**：Go 运行时与系统设计 + 链上工程（Solidity）+ 链下工程（Go RPC/索引）+ 解决方案架构 + AI 工程。
>
> **⭐ Web3 交易所 / 钱包方向**：[重点准备题单](resume-focus-web3.md)

> **如何使用左侧导航**：先从 **岗位与优先级** 选择角色/P0 主线，再到 **面试专题** 按基础 → 进阶 → 高阶 → Web3 专题展开；三级题目菜单使用紧凑标题，正文和搜索保留完整标题。

## 推荐刷题顺序（由易到难）

| 层级 | 模块 | 说明 |
|------|------|------|
| **基础** | [01 并发](interview/01-runtime-concurrency/index.md) → [02 内存](interview/02-memory-gc/index.md) → [16 Go 生产工程](interview/16-go-production-engineering/index.md) → [08 手写题](interview/08-coding-senior/index.md) | Go 语言深度 + 工程交付 + 编码 |
| **进阶** | [06 网络](interview/06-network-governance/index.md) → [中间件](interview/middleware/index.md) | API、gRPC、DB/缓存/MQ |
| **高阶** | [03 系统设计](interview/03-system-design/index.md) → [09 云原生](interview/09-cloud-native/index.md) → [11 解决方案架构](interview/11-solution-architecture/index.md) → [15 微服务](interview/15-microservices-exchange/index.md) | 架构白板、K8s、服务治理 |
| **专题** | [12 EVM](interview/12-blockchain-web3/index.md) → [17 多链钱包](interview/17-multichain-wallet/index.md) → [18 支付](interview/18-web3-payments-stablecoin/index.md) → [19 节点/RPC](interview/19-node-rpc-staking/index.md) → [20 协议/共识](interview/20-protocol-consensus-security/index.md) → [21 安全工程](interview/21-security-engineering/index.md) → [13 Solidity](interview/13-solidity-contracts/index.md) → [14 交易所](interview/14-dex-cex-engineering/index.md) | 多链、资金、节点、共识、安全、合约与交易 |
| **综合** | [07 领导力](interview/07-engineering-leadership/index.md) · [10 AI](interview/10-ai-engineering/index.md) | 软技能与 AI 专项 |

快捷入口：[角色优先级与证据](interview/_meta/role-priority-matrix.md) ·
[P0 技术纠错审计](interview/_meta/technical-corrections-audit.md) ·
[学习路线](learning-path-senior.md) · [**模拟面试**](mock-interview.md) ·
[Web3 重点题单](resume-focus-web3.md)

## 角色化 P0

| 主线 | 推荐顺序 |
|------|----------|
| Go 工程门槛 | [16 Go 生产工程](interview/16-go-production-engineering/index.md) → [08 手写题](interview/08-coding-senior/index.md) → [06 Linux/TCP](interview/06-network-governance/index.md) → [MySQL](interview/middleware/mysql/index.md) / [PostgreSQL](interview/middleware/postgresql/index.md) |
| Web3 核心 | [17 多链钱包](interview/17-multichain-wallet/index.md) → [18 支付与稳定币](interview/18-web3-payments-stablecoin/index.md) → [19 节点/RPC](interview/19-node-rpc-staking/index.md) |

先完成共享门槛，再从 Go 后端、钱包、支付、节点、交易所、Staff 六条轨道选择一个增量：
[查看角色化矩阵](interview/_meta/role-priority-matrix.md) ·
[查看知识图谱](interview/_meta/p0-knowledge-graph.md)。

## 中间件速查

| 类型 | 题数 | 入口 |
|------|------|------|
| [MySQL + GORM](interview/middleware/mysql/index.md) | 7 | 索引、MVCC、复杂 SQL、资金表与锁 |
| [PostgreSQL](interview/middleware/postgresql/index.md) | 3 | VACUUM/索引、SSI/锁、WAL/HA/pgx |
| [Redis](interview/middleware/redis/index.md) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](interview/middleware/kafka/index.md) | 4 | 架构、Producer、消费语义 |
| [RocketMQ](interview/middleware/rocketmq/index.md) | 4 | 架构、事务/顺序/延迟、排障 |
| [Elasticsearch](interview/middleware/elasticsearch/index.md) | 3 | 倒排索引、DSL、同步 |
| [分布式事务](interview/middleware/distributed/index.md) | 1 | TCC / Saga |

## 其他链接

- [Web3 交易所重点准备](resume-focus-web3.md)
- [面试题总索引](interview-catalog.md)
- [题单 YAML](interview/_meta/questions.yaml)
- [角色优先级与证据](interview/_meta/role-priority-matrix.md)
- [P0 技术纠错审计](interview/_meta/technical-corrections-audit.md)
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)

## 可运行代码

`basis/` · `gin-example/` · `gorm/` · `algorithm/` · `examples/senior/` ·
`examples/non-evm-sdk/` · `examples/solidity/`
