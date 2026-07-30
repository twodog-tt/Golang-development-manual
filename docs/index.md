# Go · Agent · Web3 工程知识库

面向 **5 年+ Go 后端、AI Agent / Crypto Agent 生态与 Web3 架构方向** 的工程知识沉淀，
共 **232 篇正文**，并配套可运行示例。

> **三条并列主线**（按场景选用，不必线性通读）：
>
> 1. **Go 生产工程** — 运行时、测试、网络、数据库、消息与可观测
> 2. **Web3 / DEX** — 合约、索引、钱包、支付、节点、交易所与资金链路
> 3. **AI / Crypto Agent** — 工作流、MCP/A2A、身份/Commerce、开放平台

!!! tip "查什么 · 学什么 · 练什么"

    | 你想… | 入口 |
    |--------|------|
    | **查**专题与原文 | [专题总索引](interview-catalog.md) · 左侧导航 · 站内搜索 |
    | **学**一条能力主线 | [Go 生产工程](interview/16-go-production-engineering/index.md) · [Web3/DEX](web3-exchange-wallet-focus.md) · [AI Agent](interview/10-ai-engineering/index.md) |
    | **练**可运行证据 | 下方「可验证项目证据」· [`examples/`](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples) |

    **DEX Tech Lead 快链**（协议终面）：
    [S-EXCH-31 白板](interview/14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md) →
    [S-EXCH-30 Uniswap V2/V3](interview/14-dex-cex-engineering/S-EXCH-30-uniswap-v2-v3-protocol.md) →
    [S-EXCH-29 Staking/Farm](interview/14-dex-cex-engineering/S-EXCH-29-defi-staking-liquidity-mining-yield.md)

!!! note "若用于面试备考"

    [① 选方向与 P0](interview/_meta/role-priority-matrix.md) →
    [② 18 个高频锚点](high-frequency-roadmap.md) →
    [③ 岗位 P0 图谱](interview/_meta/p0-knowledge-graph.md) →
    [④ 专题自测](topic-quiz.md)

> **图表操作**：正文中的架构图和时序图支持点击全屏、滚轮或双指缩放、拖拽、重置、
> 新标签打开与复制 Mermaid 源码。

## 1. 按方向浏览（P0 已同步元数据）

| 目标方向 | P0 | 首要证明的能力 |
|----------|---:|----------------|
| 资深 Go 后端 | 62 | Go/运行时、测试、网络、数据库、消息与生产工程 |
| AI Agent Platform / Crypto Agent Ecosystem | 64 | 工作流、MCP/A2A、身份/Commerce、开放平台与 Web3 安全执行 |
| 多链钱包与托管 | 66 | 多链交易、归集、MPC/HSM、签名控制与恢复 |
| 支付与稳定币 | 66 | 支付状态机、账本、清结算、合规与机构资金 |
| 节点、RPC 与 Indexer | 73 | 节点/共识、canonical 数据、列存与非 EVM 兼容 |
| 交易所工程 | **75** | 撮合/WAL、DEX 协议、预测市场、账本与安全上线 |
| Staff / 后端架构师 | **80** | 系统演进、全栈白板、IaC/GitOps、安全与跨团队影响 |

P0 含 Shared P0 与方向增量，**不要求通读全部正文**。先掌握 Shared P0 的结论与边界，
再按场景补增量；P1/P2 按项目反馈回补。

[查看七类方向完整 P0/P1/P2 与证据标签](interview/_meta/role-priority-matrix.md)

## 2. 怎么用这座知识库

| 步骤 | 动作 | 完成标准 |
|------|------|----------|
| **1. 定场景** | 选 Go / Web3·DEX / Agent 之一作主入口 | 能说清要解决的工程问题 |
| **2. 读不变量** | 先看 30 秒版 / 口述卡与反模式 | 能讲结论、边界与常见错答 |
| **3. 跟证据** | 对照 `code_refs`、测试或 harness | 分清 explanation / test / harness 等级 |
| **4. 串链路** | 用白板题把模块收成端到端叙事 | 能画出权威数据源与失败模式 |
| **5. 回补缺口** | 自测或项目复盘后只补暴露的洞 | 避免平均刷完全库 |

辅助：[学习路线](learning-path-senior.md) ·
[18 个高频锚点](high-frequency-roadmap.md) ·
[技术纠错审计](interview/_meta/technical-corrections-audit.md) ·
[专题自测](topic-quiz.md)

## 3. 可验证项目证据

| 项目证据 | 知识入口 | 仓库实现 | 当前证据边界 |
|----------|----------|----------|--------------|
| 确定性撮合与崩溃恢复 | [撮合引擎](interview/14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md) · [WAL 回放](interview/14-dex-cex-engineering/S-EXCH-18-wal-snapshot-replay.md) | [matchingengine](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/matchingengine) · [walreplay](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/walreplay) | `deterministic_test`：证明确定性语义与恢复测试，不冒充生产性能 |
| 多副本 Signer Fence 与签名后端 | [Key Ceremony 与 Fencing](interview/21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | [signer-project](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/signer-project) | `integration_harness`：覆盖 etcd、PKCS#11/SoftHSM、跨进程 FROST；不等于真实硬件验收 |
| 四条非 EVM 链 Adapter | [多链钱包专题](interview/17-multichain-wallet/index.md) | [Solana/Cosmos/Aptos/Sui](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/non-evm-sdk) | `integration_harness`：覆盖 fixture、故障与兼容门禁；testnet/localnet 不等于生产 |
| Launchpad / DEX 链路 | [Web3 全栈](interview/14-dex-cex-engineering/S-EXCH-14-web3-exchange-fullstack-architecture.md) · [DEX TL 白板](interview/14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md) | [EVM 合约绑定示例](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/erc20bind) | `illustrative_artifact`：讲 Indexer、Reorg、协议边界；不声称复现完整生产 DEX |

证据标签约束对外表达：有代码 ≠ 生产验收，SoftHSM ≠ 真实 HSM，
localnet/testnet ≠ 主网兼容。完整定义见
[角色优先级与证据标签](interview/_meta/role-priority-matrix.md)。

[工作流/HITL](interview/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) 与
[Persona/Memory](interview/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md)
目前是结构化设计依据，不应表述成「仓库已复现完整生产 Agent 平台」。

## 4. 专题索引

| 层级 | 模块 | 适用内容 |
|------|------|----------|
| **Go 基础与生产工程** | [01 并发](interview/01-runtime-concurrency/index.md) → [02 内存](interview/02-memory-gc/index.md) → [16 Go 生产工程](interview/16-go-production-engineering/index.md) → [08 手写题](interview/08-coding-senior/index.md) | Go 深度、测试、性能与编码门槛 |
| **网络与中间件** | [06 网络](interview/06-network-governance/index.md) → [中间件](interview/middleware/index.md) | Linux/TCP、API、gRPC、数据库、缓存与 MQ |
| **系统与组织架构** | [03 系统设计](interview/03-system-design/index.md) → [09 云原生](interview/09-cloud-native/index.md) → [11 解决方案架构](interview/11-solution-architecture/index.md) → [15 微服务](interview/15-microservices-exchange/index.md) → [07 领导力](interview/07-engineering-leadership/index.md) | 架构白板、K8s、演进、迁移与 Staff 影响力 |
| **AI / Crypto Agent** | [10 AI 工程](interview/10-ai-engineering/index.md) → [工作流/HITL](interview/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) → [MCP/A2A](interview/10-ai-engineering/S-AI-11-mcp-a2a-vendor-neutral-interoperability.md) → [Agent Commerce](interview/10-ai-engineering/S-AI-13-x402-erc8183-agent-commerce.md) → [开放平台/Launchpad](interview/10-ai-engineering/S-AI-14-crypto-agent-open-platform-marketplace-launchpad.md) | Agent 生态 |
| **Web3 基础设施** | [12 EVM](interview/12-blockchain-web3/index.md) → [17 多链钱包](interview/17-multichain-wallet/index.md) → [18 支付](interview/18-web3-payments-stablecoin/index.md) → [19 节点/RPC](interview/19-node-rpc-staking/index.md) → [20 协议/共识](interview/20-protocol-consensus-security/index.md) → [21 安全](interview/21-security-engineering/index.md) → [13 Solidity](interview/13-solidity-contracts/index.md) → [14 交易所](interview/14-dex-cex-engineering/index.md) | 多链、资金、节点、协议、安全、合约与交易 |

左侧导航适合按专题查漏；正文与搜索保留完整题目标题。

## 5. 中间件速查

| 类型 | 题数 | 重点 |
|------|-----:|------|
| [MySQL + GORM](interview/middleware/mysql/index.md) | 7 | 索引、MVCC、复杂 SQL、资金表与锁 |
| [PostgreSQL](interview/middleware/postgresql/index.md) | 3 | VACUUM/索引、SSI/锁、WAL/HA/pgx |
| [Redis](interview/middleware/redis/index.md) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](interview/middleware/kafka/index.md) | 4 | 架构、Producer 与消费语义 |
| [RocketMQ](interview/middleware/rocketmq/index.md) | 4 | 架构、事务/顺序/延迟与排障 |
| [Elasticsearch](interview/middleware/elasticsearch/index.md) | 3 | 倒排索引、DSL 与数据同步 |
| [分布式事务](interview/middleware/distributed/index.md) | 1 | TCC / Saga |

## 资料、索引与维护

- [专题总索引](interview-catalog.md)
- [题单 YAML](interview/_meta/questions.yaml)
- [角色优先级与证据标签](interview/_meta/role-priority-matrix.md)
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)
