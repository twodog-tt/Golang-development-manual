# Go · Agent · Web3 工程知识库

面向 **Go 生产工程、Web3 链上基建与资金链路、AI / Crypto Agent 控制面** 的开源工程知识沉淀，
共 **233 篇正文**，并配套可运行示例。

> **三条并列主线**（按工程场景选用，不必线性通读）：
>
> 1. **Go 生产工程** — 运行时、测试、网络、数据库、消息与可观测
> 2. **Web3 链上基建** — 合约、索引、钱包、支付、节点、交易所与资金闭环
> 3. **AI / Crypto Agent** — 工作流、MCP/A2A、身份/Commerce、开放平台与执行边界

!!! tip "查什么 · 学什么 · 练什么"

    | 你想… | 入口 |
    |--------|------|
    | **先建立领域心智模型** | [概念地图](maps/index.md)（钱包 / Indexer / 资金 / Agent / [易混点](maps/confusion-cards.md)） |
    | **查**专题与原文 | [专题总索引](topic-catalog.md) · 左侧导航 · 站内搜索 |
    | **学**一条工程主线 | [Go 生产工程](topics/16-go-production-engineering/index.md) · [Web3 场景地图](web3-exchange-wallet-focus.md) · [AI Agent](topics/10-ai-engineering/index.md) |
    | **练**可运行证据 | 下方「可验证项目证据」· [`examples/`](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples) |

    **DEX 协议综合演练**（可选深挖）：
    [S-EXCH-31 白板](topics/14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md) →
    [S-EXCH-30 Uniswap V2/V3](topics/14-dex-cex-engineering/S-EXCH-30-uniswap-v2-v3-protocol.md) →
    [S-EXCH-29 Staking/Farm](topics/14-dex-cex-engineering/S-EXCH-29-defi-staking-liquidity-mining-yield.md)

!!! note "辅助导航（可选）"

    [① 领域能力优先级与证据标签](topics/_meta/role-priority-matrix.md) →
    [② 18 个高频锚点](high-frequency-roadmap.md) →
    [③ 领域知识图谱](topics/_meta/p0-knowledge-graph.md) →
    [④ 专题自测](topic-quiz.md)

> **图表操作**：正文中的架构图和时序图支持点击全屏、滚轮或双指缩放、拖拽、重置、
> 新标签打开与复制 Mermaid 源码。

## 1. 按工程领域浏览

| 工程领域 | 建议先读 | 解决什么问题 |
|----------|----------|--------------|
| Go 生产工程 | [16](topics/16-go-production-engineering/index.md) · [01](topics/01-runtime-concurrency/index.md) · [02](topics/02-memory-gc/index.md) | 运行时、测试、性能与生产排障 |
| 网络与中间件 | [06](topics/06-network-governance/index.md) · [中间件](topics/middleware/index.md) | API、连接、数据库、缓存、消息 |
| 系统设计与云原生 | [03](topics/03-system-design/index.md) · [09](topics/09-cloud-native/index.md) | 幂等、一致性、发布、可观测与容量 |
| 多链钱包与托管签名 | [概念地图](maps/wallet-custody.md) · [17](topics/17-multichain-wallet/index.md) | Adapter、充提归集、MPC/HSM、恢复 |
| 支付与账本 | [18](topics/18-web3-payments-stablecoin/index.md) · [资金地图](maps/exchange-funds.md) | 支付状态机、清结算、合规边界 |
| 节点、RPC 与 Indexer | [概念地图](maps/indexer-node-data.md) · [19](topics/19-node-rpc-staking/index.md) | 扫块、reorg、canonical、交易管理 |
| 交易所与协议 | [资金地图](maps/exchange-funds.md) · [14](topics/14-dex-cex-engineering/index.md) | CEX/DEX 资金与协议、行情、对账 |
| AI / Crypto Agent | [概念地图](maps/agent-control-plane.md) · [10](topics/10-ai-engineering/index.md) | 可审计工作流、互操作、执行边界 |
| 安全与领导力 | [21](topics/21-security-engineering/index.md) · [07](topics/07-engineering-leadership/index.md) | 威胁模型、签名 fencing、事故与演进 |

各领域的核心/延展阅读清单与证据标签见
[领域能力优先级与证据标签](topics/_meta/role-priority-matrix.md)。
**不要求通读全库**：先掌握共享 Go/生产不变量，再按当前工程问题补一个领域。

## 2. 怎么用这座知识库

| 步骤 | 动作 | 完成标准 |
|------|------|----------|
| **1. 定场景** | 选 Go / Web3 链上基建 / Agent 之一作主入口 | 能说清要解决的工程问题 |
| **2. 读不变量** | 先看 30 秒版 / 要点卡与反模式 | 能讲结论、边界与常见误区 |
| **3. 跟证据** | 对照 `code_refs`、测试或 harness | 分清 explanation / test / harness 等级 |
| **4. 串链路** | 把模块收成端到端架构（事实源、状态机、失败模式） | 能画出权威数据源与恢复路径 |
| **5. 回补缺口** | 项目复盘或自测后只补暴露的洞 | 避免平均通读全库 |

辅助：[学习路线](learning-path-senior.md) ·
[18 个高频锚点](high-frequency-roadmap.md) ·
[技术纠错审计](topics/_meta/technical-corrections-audit.md) ·
[专题自测](topic-quiz.md)

## 3. 可验证项目证据

| 项目证据 | 知识入口 | 仓库实现 | 当前证据边界 |
|----------|----------|----------|--------------|
| 确定性撮合与崩溃恢复 | [撮合引擎](topics/14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md) · [WAL 回放](topics/14-dex-cex-engineering/S-EXCH-18-wal-snapshot-replay.md) | [matchingengine](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/matchingengine) · [walreplay](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/walreplay) | `deterministic_test`：证明确定性语义与恢复测试，不冒充生产性能 |
| 多副本 Signer Fence 与签名后端 | [Key Ceremony 与 Fencing](topics/21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | [signer-project](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/signer-project) | `integration_harness`：覆盖 etcd、PKCS#11/SoftHSM、跨进程 FROST；不等于真实硬件验收 |
| 四条非 EVM 链 Adapter | [多链钱包专题](topics/17-multichain-wallet/index.md) | [Solana/Cosmos/Aptos/Sui](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/non-evm-sdk) | `integration_harness`：覆盖 fixture、故障与兼容门禁；testnet/localnet 不等于生产 |
| Launchpad / DEX 链路 | [Web3 全栈](topics/14-dex-cex-engineering/S-EXCH-14-web3-exchange-fullstack-architecture.md) · [DEX 协议白板](topics/14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md) | [EVM 合约绑定示例](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/erc20bind) | `illustrative_artifact`：讲 Indexer、Reorg、协议边界；不声称复现完整生产 DEX |

证据标签约束对外表达：有代码 ≠ 生产验收，SoftHSM ≠ 真实 HSM，
localnet/testnet ≠ 主网兼容。完整定义见
[领域能力优先级与证据标签](topics/_meta/role-priority-matrix.md)。

[工作流/HITL](topics/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) 与
[Persona/Memory](topics/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md)
目前是结构化设计依据，不应表述成「仓库已复现完整生产 Agent 平台」。

## 4. 专题索引

| 层级 | 模块 | 适用内容 |
|------|------|----------|
| **Go 基础与生产工程** | [01 并发](topics/01-runtime-concurrency/index.md) → [02 内存](topics/02-memory-gc/index.md) → [16 Go 生产工程](topics/16-go-production-engineering/index.md) → [08 编码练习](topics/08-coding-senior/index.md) | Go 深度、测试、性能与编码门槛 |
| **网络与中间件** | [06 网络](topics/06-network-governance/index.md) → [中间件](topics/middleware/index.md) | Linux/TCP、API、gRPC、数据库、缓存与 MQ |
| **系统与组织架构** | [03 系统设计](topics/03-system-design/index.md) → [09 云原生](topics/09-cloud-native/index.md) → [11 解决方案架构](topics/11-solution-architecture/index.md) → [15 微服务](topics/15-microservices-exchange/index.md) → [07 领导力](topics/07-engineering-leadership/index.md) | 架构白板、K8s、演进、迁移与跨团队协作 |
| **AI / Crypto Agent** | [10 AI 工程](topics/10-ai-engineering/index.md) → [工作流/HITL](topics/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) → [MCP/A2A](topics/10-ai-engineering/S-AI-11-mcp-a2a-vendor-neutral-interoperability.md) → [Agent Commerce](topics/10-ai-engineering/S-AI-13-x402-erc8183-agent-commerce.md) → [开放平台/Launchpad](topics/10-ai-engineering/S-AI-14-crypto-agent-open-platform-marketplace-launchpad.md) | Agent 控制面与生态 |
| **Web3 基础设施** | [12 EVM](topics/12-blockchain-web3/index.md) → [17 多链钱包](topics/17-multichain-wallet/index.md) → [18 支付](topics/18-web3-payments-stablecoin/index.md) → [19 节点/RPC](topics/19-node-rpc-staking/index.md) → [20 协议/共识](topics/20-protocol-consensus-security/index.md) → [21 安全](topics/21-security-engineering/index.md) → [13 Solidity](topics/13-solidity-contracts/index.md) → [14 交易所](topics/14-dex-cex-engineering/index.md) | 多链、资金、节点、协议、安全、合约与交易 |

左侧导航适合按专题查漏；正文与搜索保留完整标题。

## 5. 中间件速查

| 类型 | 篇数 | 重点 |
|------|-----:|------|
| [MySQL + GORM](topics/middleware/mysql/index.md) | 7 | 索引、MVCC、复杂 SQL、资金表与锁 |
| [PostgreSQL](topics/middleware/postgresql/index.md) | 3 | VACUUM/索引、SSI/锁、WAL/HA/pgx |
| [Redis](topics/middleware/redis/index.md) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](topics/middleware/kafka/index.md) | 4 | 架构、Producer 与消费语义 |
| [RocketMQ](topics/middleware/rocketmq/index.md) | 4 | 架构、事务/顺序/延迟与排障 |
| [Elasticsearch](topics/middleware/elasticsearch/index.md) | 3 | 倒排索引、DSL 与数据同步 |
| [分布式事务](topics/middleware/distributed/index.md) | 1 | TCC / Saga |

## 资料、索引与维护

- [概念地图](maps/index.md)
- [专题总索引](topic-catalog.md)
- [专题元数据 YAML](topics/_meta/topics.yaml)
- [领域能力优先级与证据标签](topics/_meta/role-priority-matrix.md)
- [代码映射](topics/_meta/mapping.md)
- [引用来源](sources.md)
