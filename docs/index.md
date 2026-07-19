# Go · Agent · Web3 架构师面试手册

面向 **5 年+ Go 后端、AI Agent Platform / Infrastructure 与 Web3 架构岗位** 的
面试知识库，共 **219 篇正文**。

> **当前简历主线**：先证明资深 Go 与生产工程基本盘，再重点讲 Agent 工作流、HITL、
> Persona/Memory、Guardrail 和外部副作用；交易所、钱包、实时风控与链上工程作为处理
> 资金、高吞吐、可恢复数据链路的差异化证据。

!!! tip "从这里开始"

    [① 选择目标岗位](interview/_meta/role-priority-matrix.md) →
    [② 查看简历定向 P0 图谱](interview/_meta/p0-knowledge-graph.md) →
    [③ 开始模拟面试](mock-interview.md)

    Web3 定向准备可直接进入 [交易所与钱包重点题单](resume-focus-web3.md)。

> **图表操作**：正文中的架构图和时序图支持点击全屏、滚轮或双指缩放、拖拽、重置、
> 新标签打开与复制 Mermaid 源码。

## 1. 先选岗位，再决定 P0

| 目标岗位 | P0 | 首要证明的能力 |
|----------|---:|----------------|
| **AI Agent Platform / Infrastructure（简历首选）** | **60** | 工作流、HITL、Memory、Guardrail、成本与外部副作用 |
| 资深 Go 后端 | 62 | Go/运行时、测试、网络、数据库、消息与生产工程 |
| 多链钱包与托管 | 66 | 多链交易、归集、MPC/HSM、签名控制与恢复 |
| 支付与稳定币 | 66 | 支付状态机、账本、清结算、合规与机构资金 |
| 交易所工程 | 68 | 撮合/WAL、行情/FIX、账本、风控与充提 |
| 节点、RPC 与 Indexer | 73 | 节点/共识、canonical 数据、列存与非 EVM 兼容 |
| Staff / 后端架构师 | 75 | 系统演进、迁移、IaC/GitOps、安全与跨团队影响 |

这里的 P0 已包含约 40 篇 Shared P0，不代表需要逐字背诵全部正文。先掌握 Shared P0 的
30 秒版与关键不变量，再只进入一个岗位的增量 P0；P1 按目标 JD 补充，P2 只根据面试反馈回补。

[查看七类岗位完整 P0/P1/P2 与证据标签](interview/_meta/role-priority-matrix.md)

## 2. 五步面试准备法

| 步骤 | 动作 | 完成标准 |
|------|------|----------|
| **1. 选择岗位** | 在七类角色中确定一个主投出口 | 能用一句话说明“为什么是我” |
| **2. Shared P0** | Go 并发、生产工程、网络、数据库、幂等、MQ、可观测 | 每题能讲 30 秒结论、边界与反例 |
| **3. 岗位增量** | 只补目标角色的 P0，再按 JD 选择 P1 | 不再按统一题单平均用力 |
| **4. 项目举证** | 用个人真实项目关联设计、代码、故障与取舍 | 明确本人职责、约束、指标和证据等级 |
| **5. 模拟与回补** | 进行模拟面试，记录失分点 | 只回补暴露出的知识洞与错误表达 |

辅助入口：[学习路线](learning-path-senior.md) ·
[P0 技术纠错审计](interview/_meta/technical-corrections-audit.md) ·
[模拟面试](mock-interview.md)

## 3. 可验证项目证据

| 项目证据 | 知识入口 | 仓库实现 | 当前证据边界 |
|----------|----------|----------|--------------|
| 确定性撮合与崩溃恢复 | [撮合引擎](interview/14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md) · [WAL 回放](interview/14-dex-cex-engineering/S-EXCH-18-wal-snapshot-replay.md) | [matchingengine](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/matchingengine) · [walreplay](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/walreplay) | `deterministic_test`：证明确定性语义与恢复测试，不冒充生产性能 |
| 多副本 Signer Fence 与签名后端 | [Key Ceremony 与 Fencing](interview/21-security-engineering/S-SEC-02-key-ceremony-signer-fencing-recovery.md) | [signer-project](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/signer-project) | `integration_harness`：覆盖 etcd、PKCS#11/SoftHSM、跨进程 FROST；不等于真实硬件验收 |
| 四条非 EVM 链 Adapter | [多链钱包专题](interview/17-multichain-wallet/index.md) | [Solana/Cosmos/Aptos/Sui](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/non-evm-sdk) | `integration_harness`：覆盖 fixture、故障与兼容门禁；testnet/localnet 不等于生产 |
| Launchpad 类 DEX 全链路 | [链上 DEX + 链下 Go](interview/14-dex-cex-engineering/S-EXCH-14-web3-exchange-fullstack-architecture.md) | [EVM 合约绑定示例](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/erc20bind) | `illustrative_artifact`：用于讲 Indexer、Reorg、K 线与副作用边界，不声称仓库复现完整生产系统 |

证据标签用于约束面试表达：有代码不等于生产验收，SoftHSM 不等于真实 HSM，
localnet/testnet 也不等于主网兼容。完整定义见
[角色优先级与证据标签](interview/_meta/role-priority-matrix.md)。

Agent Platform 仍是简历主线；公开仓库中的
[工作流/HITL](interview/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) 与
[Persona/Memory](interview/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md)
目前属于结构化设计依据，不应表述成“仓库已经复现完整生产平台”。

## 4. 专题索引

| 层级 | 模块 | 适用内容 |
|------|------|----------|
| **Go 基础与生产工程** | [01 并发](interview/01-runtime-concurrency/index.md) → [02 内存](interview/02-memory-gc/index.md) → [16 Go 生产工程](interview/16-go-production-engineering/index.md) → [08 手写题](interview/08-coding-senior/index.md) | Go 深度、测试、性能与编码门槛 |
| **网络与中间件** | [06 网络](interview/06-network-governance/index.md) → [中间件](interview/middleware/index.md) | Linux/TCP、API、gRPC、数据库、缓存与 MQ |
| **系统与组织架构** | [03 系统设计](interview/03-system-design/index.md) → [09 云原生](interview/09-cloud-native/index.md) → [11 解决方案架构](interview/11-solution-architecture/index.md) → [15 微服务](interview/15-microservices-exchange/index.md) → [07 领导力](interview/07-engineering-leadership/index.md) | 架构白板、K8s、演进、迁移与 Staff 影响力 |
| **AI Agent** | [10 AI 工程](interview/10-ai-engineering/index.md) → [工作流/HITL](interview/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) → [Persona/Memory](interview/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md) | Agent 平台主线 |
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

- [面试题总索引](interview-catalog.md)
- [题单 YAML](interview/_meta/questions.yaml)
- [角色优先级与证据标签](interview/_meta/role-priority-matrix.md)
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)
