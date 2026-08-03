# 领域能力优先级与证据标签

> 当前共 **233 篇 published 正文**。领域优先级与证据标签的机器事实源为
> [role-evidence.yaml](./role-evidence.yaml)（文件名历史兼容，语义为 **domain/role 能力切片**），
> 一致性由 `scripts/verify_knowledge_metadata.py` 校验。更新时间：**2026-08-03**。

本页回答两件事：

1. **读哪些**：共享工程门槛 + 某一工程领域的核心/延展清单  
2. **能声称什么**：证据标签（explanation / test / harness）与时效标签  

这是开源知识库的**阅读导航与诚实边界**，不是求职岗位说明书。

## 先选领域，不再套统一核心清单

旧 [topics.yaml](./topics.yaml) 中的全局 P0/P1/P2 保留给导航和历史兼容，但不再代表
所有读者的统一阅读顺序。有效优先级按以下规则计算：

```text
领域核心 = shared.core ∪ domain.core
领域延展 = (shared.extended ∪ domain.extended) - 领域核心
其余 = 其余 published 正文
```

（YAML 中仍使用 `p0/p1/p2` 字段名以兼容校验脚本；对外叙述用「核心 / 延展」。）

| 工程领域 | 核心 | 延展 | 其余 | 核心重点 |
|----------|---:|---:|---:|---------|
| Go 生产工程 | 62 | 72 | 99 | Go/运行时、测试、网络、PostgreSQL/MySQL、消息、IaC/GitOps |
| **AI / Crypto Agent 控制面** | **64** | **90** | **79** | 工作流、MCP/A2A、Agent 身份/Commerce、开放平台、Web3 安全执行 |
| 多链钱包与托管签名 | 66 | 91 | 76 | 多链交易、TRON、归集、MPC/HSM、签名控制、恢复 |
| 支付与稳定币 | 66 | 98 | 69 | 支付状态机、账本、TRC20、清结算、合规、机构资金 |
| 节点 / RPC / Indexer | 73 | 86 | 74 | 节点/共识、canonical 数据、列存、非 EVM 兼容 |
| 交易所工程 | **75** | **106** | **52** | 撮合/WAL、DEX 协议、预测市场 CTF/CLOB、账本与安全上线 |
| 系统演进与架构协作 | **80** | **89** | **64** | 系统设计、DEX/预测市场白板、迁移、IaC/GitOps、安全、跨团队协作 |

核心篇数不是“全部逐字通读”。建议先掌握 shared 核心的 30 秒版与不变量，再只进入
**一个**领域增量；延展用于该领域深挖，其余正文按项目反馈补洞。

## Shared 核心：Go / 生产工程共同门槛

| 能力域 | 核心内容 |
|--------|----------|
| Go 并发与运行时 | GMP、channel、锁、context、泄漏、内存模型、并发预算、netpoll |
| 内存与性能 | GC、GOGC、逃逸、slice/map、OOM、pprof |
| 生产工程 | error/panic、接口、table/fake、fuzz/race/benchmark、modules、供应链 |
| 手写与网络 | 优雅关闭、errgroup、singleflight、有界批处理、Linux/epoll/TCP |
| 数据库 | 复杂 SQL、资金表、PostgreSQL 并发/WAL/HA |
| 系统设计 | 幂等、一致性、消息语义、订单状态机、可观测、SLO、容量与 ADR |

这约 40 篇解决「资深 Go 工程师基本盘」。领域增量解决「这类系统特有的正确性与边界」。

## 七个领域增量

### AI / Crypto Agent 控制面

在 shared 核心之外，新增约 24 篇领域定向核心：

| 能力域 | 核心题号 | 项目证据 |
|--------|----------|----------|
| 模型与工具入口 | `S-AI-01/03/07` | Go LLM 接入、Tool/Skill、MCP Server |
| 协议互操作 | `S-AI-11` | MCP/A2A 边界、统一任务状态与 vendor-neutral conformance |
| 工作流与人工控制 | `S-AI-09` | Review Queue、Execution Queue、Publishing Pipeline |
| Persona/Memory | `S-AI-04/10` | Bot/Scene Context、反馈规则再注入 |
| 安全与成本 | `S-AI-05/06`、`S-SEC-01/04` | Guardrail、注入隔离、token/cost、事件响应 |
| Agent 身份与商业协议 | `S-AI-12/13` | ERC-8004、x402/x402b、ERC-8183、支付/托管/对账 |
| 开放生态与 Launchpad | `S-AI-14` | SDK、发布治理、Marketplace、钱包执行和 Agent Launchpad |
| 平台控制面 | `S-ARCH-08/09/15`、`S-NET-04` | 限额、熔断、版本兼容、OAuth2/凭据 |
| 实时数据与消息 | `S-ARCH-21`、`S-KAFKA-02`、`S-RAB-01`、`S-ES-03` | CDC/Flink/ES、RabbitMQ 行情与异步任务 |
| SaaS 与 Web3 证据 | `S-SOL-05`、`S-WALLET-12` | 多租户平台、EVM/TRON USDT 与 TRC20 |

这条路径不要求把 LangGraph、AutoGen、CrewAI 的 API 逐个背熟。核心主张应是：
**理解框架能力，但能用 Go 自建可审计状态机，并清楚框架 checkpoint 与业务幂等的边界**。

### Go 生产工程

优先升级：

- `S-CONC-02/04/15/20`、`S-MEM-12`：版本边界、race 与 GC 压力；
- `S-CODE-01/02/05`：并发容器、限流与连接池；
- `S-DB-01~03`、`S-PG-01`：索引、隔离、慢查询、VACUUM；
- `S-DIST-02/04`、`S-KAFKA-02`：fencing、消息边界与 producer 可靠性；
- `S-CLOUD-04/07/09/10`：发布、排障、Terraform 与 GitOps。

### 多链钱包与托管签名

优先升级 `S-WALLET-01~12`、`S-BC-03/05/10/12`、`S-SEC-01/02/04`、
`S-NODE-02/05/09` 和 `S-PAY-03~05`。阅读时必须保留各链 nonce/UTXO/object/sequence、
提交、执行和 finality 差异；并区分 **托管信任模型** 与 **MPC 签名实现**。

### 支付与稳定币

优先升级 `S-PAY-01~06`、钱包归集/签名、交易所账本/对账、节点交易管理器和安全控制。
核心表达是：**链上 observation、支付 intent、账本事实、scheme/资金腿和法律 finality
不是同一状态**。

### 节点、RPC 与 Indexer

优先升级 `S-NODE-01~10`、`S-PROTO-01~04`、Rollup/跨链、非 EVM 链模型与云发布。
新增 [S-NODE-10](../19-node-rpc-staking/S-NODE-10-chain-data-clickhouse-lakehouse.md)
要求能同时解释 parent lineage、ClickHouse MergeTree 和 lakehouse replay。

### 交易所工程

优先升级 `S-EXCH-01~05/10/11/13/15~26`、**`S-EXCH-29/30/31`**、交易所微服务、账本、充提、
节点交易管理器与 signer 控制。三条工程主线：

- **CEX / 撮合外围与资金**：`S-EXCH-01~05/17~22` + 微服务/账本；性能结论必须附 workload、持久化边界、
  P99/P999 和恢复语义。
- **DEX 协议与链下投影**：`S-EXCH-31 → 30 → 29`（白板 → Uniswap V2/V3 → Staking/LM/Yield），
  再按反馈补 `S-EXCH-06/27` 与合约/链上安全。
- **预测市场**：`S-EXCH-23 → 24 → 25 → 26` 形成 CTF/生命周期、CLOB/EIP-712/结算、
  数据源/争议、安全/上线闭环。

### 系统演进与架构协作

优先升级领导力、解决方案架构、跨地域/迁移、IaC/GitOps、安全与数据平台。
若当前问题是 **DEX 协议端到端**，把 `S-EXCH-31` 作为白板入口，再按需补
`S-EXCH-30/29`；若是预测市场全链路，再把 `S-EXCH-23~26` 作为核心，
重点讲链上/链下边界、机制风险、里程碑和跨团队上线门禁。
[S-LEAD-04](../07-engineering-leadership/S-LEAD-04-staff-strategy-influence-case.md) 和
[S-LEAD-05](../07-engineering-leadership/S-LEAD-05-cross-team-migration-case.md)
使用占位符训练案例；没有真实数据就明确说目标/估算，不能把模板包装成个人经历。

## 证据标签不是“完成度星级”

证据分三维，防止不同类型的证明互相冒充。

### 内容依据

| 标签 | 含义 |
|------|------|
| `source_anchored` | 正文给出可回查资料；仍要按时效标签复核 |
| `experience_pattern` | 组织/案例方法论，只能替换成个人真实经历后使用 |

### 可复现程度

| 标签 | 当前篇数 | 能证明什么 | 不能证明什么 |
|------|---------:|------------|--------------|
| `explanation_only` | 174 | 结构化答案、SQL/配置与来源 | 代码已运行、环境已验收 |
| `illustrative_artifact` | 31 | 仓库有相关代码或配置 | 测试当前通过、外部系统兼容 |
| `deterministic_test` | 20 | 有不依赖外部服务的测试/回放门禁 | localnet、硬件或生产行为 |
| `integration_harness` | 7 | 有 localnet/testnet/HSM/MPC/故障 harness | 每个目标版本都已实跑 |
| `external_acceptance` | 0 | 真实厂商/生产环境的具名验收 | — |

高等级只表示证据类型更接近外部环境，不表示题目更重要。当前 `external_acceptance=0` 是刻意的
诚实边界：仓库不能把 SoftHSM、测试网或本地 harness 写成真实厂商 HSM/生产验收。

### 时效性

| 标签 | 当前篇数 | 复核方式 |
|------|---------:|----------|
| `stable` | 145 | 目标数据库/语言版本使用前抽查 |
| `version_sensitive` | 73 | 复核官方 release/spec/SDK 文档 |
| `vendor_or_regulatory_sensitive` | 14 | 结合目标厂商、司法辖区和法律/合规意见 |

`vendor_or_regulatory_sensitive` 优先于普通版本敏感；它提醒你对外说明适用范围，而不是套一个
全球通用结论。

## 使用方式

1. 在 [role-evidence.yaml](./role-evidence.yaml) 选择一个 domain/role 切片。
2. 先过 shared 核心，再过该领域的增量核心。
3. 每题先看可复现标签：`explanation_only` 就只声称“设计/理解”；有 test/harness 才说明覆盖范围。
4. 遇到 version/vendor/regulatory 标签，按目标场景再核官方资料。
5. 每周运行：

```bash
.venv/bin/python scripts/verify_knowledge_metadata.py
```

校验会检查 233 个 ID、正文、sources、30 秒版、深挖问答、领域引用和证据标签互斥关系。
