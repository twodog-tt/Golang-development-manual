# 专题索引

> 全库 **233 篇**。左侧导航按 **基础 → 进阶 → 高阶 → 专题 → 综合** 组织；本页为速查表。
> **⭐ 方向定向**：[角色优先级与证据标签](topics/_meta/role-priority-matrix.md) ·
> [Web3 交易所重点专题](web3-exchange-wallet-focus.md)

## 基础 · Go 语言与生产工程（48 篇）

| 模块 | 篇数 | 入口 |
|------|------|------|
| 01 并发与运行时 | 20 | [01-runtime-concurrency/](topics/01-runtime-concurrency/index.md) |
| 02 内存与 GC | 15 | [02-memory-gc/](topics/02-memory-gc/index.md) |
| 16 Go 生产工程 | 6 | [16-go-production-engineering/](topics/16-go-production-engineering/index.md) |
| 08 编码练习 | 7 | [08-coding-senior/](topics/08-coding-senior/index.md) |

## 进阶 · 网络与中间件（33 篇）

| 模块 | 篇数 | 入口 |
|------|------|------|
| 06 网络与服务治理 | 7 | [06-network-governance/](topics/06-network-governance/index.md) |
| 中间件与数据库 | 26 | [middleware/](topics/middleware/index.md) |

中间件子目录：MySQL(7)、PostgreSQL(3)、Redis(3)、Kafka(4)、RocketMQ(4)、RabbitMQ(1)、ES(3)、分布式事务(1)。

## 高阶 · 系统设计与架构（45 篇）

| 模块 | 篇数 | 入口 |
|------|------|------|
| 03 系统设计 | 21 | [03-system-design/](topics/03-system-design/index.md) |
| 09 云原生 | 10 | [09-cloud-native/](topics/09-cloud-native/index.md) |
| 11 解决方案架构 | 8 | [11-solution-architecture/](topics/11-solution-architecture/index.md) |
| 15 微服务（交易所场景） | 6 | [15-microservices-exchange/](topics/15-microservices-exchange/index.md) |

## 专题 · Web3 核心基础设施（87 篇）

| 模块 | 篇数 | 入口 |
|------|------|------|
| 12 区块链与 Web3 | 13 | [12-blockchain-web3/](topics/12-blockchain-web3/index.md) |
| 17 多链钱包与托管 | 12 | [17-multichain-wallet/](topics/17-multichain-wallet/index.md) |
| 18 Web3 支付与稳定币 | 6 | [18-web3-payments-stablecoin/](topics/18-web3-payments-stablecoin/index.md) |
| 19 节点、RPC 与 Staking | 10 | [19-node-rpc-staking/](topics/19-node-rpc-staking/index.md) |
| 20 协议、共识与安全 | 4 | [20-protocol-consensus-security/](topics/20-protocol-consensus-security/index.md) |
| 21 Web3 安全工程 | 4 | [21-security-engineering/](topics/21-security-engineering/index.md) |
| 13 Solidity 与合约 | 8 | [13-solidity-contracts/](topics/13-solidity-contracts/index.md) |
| 14 DEX / CEX / 预测市场 | 31 | [14-dex-cex-engineering/](topics/14-dex-cex-engineering/index.md) |

## 综合 · 领导力与 AI（19 篇）

| 模块 | 篇数 | 入口 |
|------|------|------|
| 07 工程与领导力 | 5 | [07-engineering-leadership/](topics/07-engineering-leadership/index.md) |
| 10 AI 工程与编程 | 14 | [10-ai-engineering/](topics/10-ai-engineering/index.md) |

## 角色化 P0

1. **共享 Go/生产工程门槛**：[16 Go 生产工程](topics/16-go-production-engineering/index.md) → [08 编码练习](topics/08-coding-senior/index.md) → [06 Linux/TCP](topics/06-network-governance/index.md) → [PostgreSQL](topics/middleware/postgresql/index.md) / [MySQL](topics/middleware/mysql/index.md)。
2. **重点方向 Agent / Crypto Agent**：[10 AI 工程](topics/10-ai-engineering/index.md) → [工作流/HITL](topics/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) → [MCP/A2A](topics/10-ai-engineering/S-AI-11-mcp-a2a-vendor-neutral-interoperability.md) → [Agent Commerce](topics/10-ai-engineering/S-AI-13-x402-erc8183-agent-commerce.md) → [开放平台/Launchpad](topics/10-ai-engineering/S-AI-14-crypto-agent-open-platform-marketplace-launchpad.md)。
3. **Web3 证据主线**：[17 多链钱包](topics/17-multichain-wallet/index.md) → [18 支付与稳定币](topics/18-web3-payments-stablecoin/index.md) → [19 节点/RPC](topics/19-node-rpc-staking/index.md)。

完整文档 ID：[角色优先级矩阵](topics/_meta/role-priority-matrix.md)；
依赖图：[知识图谱](topics/_meta/p0-knowledge-graph.md)。

## 高频 Top 10

1. [S-CONC-01 GMP](topics/01-runtime-concurrency/S-CONC-01-gmp-overview.md)
2. [S-CONC-05 Channel](topics/01-runtime-concurrency/S-CONC-05-channel.md)
3. [S-CONC-12 Context](topics/01-runtime-concurrency/S-CONC-12-context.md)
4. [S-CONC-13 goroutine 泄漏](topics/01-runtime-concurrency/S-CONC-13-goroutine-leak.md)
5. [S-MEM-01 三色标记 GC](topics/02-memory-gc/S-MEM-01-tri-color-gc.md)
6. [S-MEM-04 逃逸分析](topics/02-memory-gc/S-MEM-04-escape-analysis.md)
7. [S-ARCH-02 秒杀](topics/03-system-design/S-ARCH-02-seckill.md)
8. [S-ARCH-04 幂等](topics/03-system-design/S-ARCH-04-idempotency.md)
9. [S-ARCH-06 缓存三大问题](topics/03-system-design/S-ARCH-06-cache-failure-modes.md)
10. [S-ARCH-10 MQ 语义](topics/03-system-design/S-ARCH-10-mq-semantics.md)

专题元数据：[topics.yaml](topics/_meta/topics.yaml)
