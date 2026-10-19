---
id: S-AI-14
title: Crypto Agent SDK、开放平台、Marketplace 与 Launchpad 架构
module: ai-engineering
level: architect
frequency: 5
tags: [agent-sdk, open-platform, marketplace, launchpad, developer-ecosystem, plugin-security]
status: published
resume_focus: true
code_refs:
  - examples/senior/agentplatform
reviewed_at: "2026-10-19"
sources:
  - https://modelcontextprotocol.io/specification/2025-11-25
  - https://a2a-protocol.org/latest/
  - https://docs.elizaos.ai/plugins/architecture
  - https://whitepaper.virtuals.io/builders-hub/game-framework
  - https://docs.bankr.bot/agent-api/overview/
  - https://clawpump.tech/docs
  - https://www.rfc-editor.org/rfc/rfc8785.html
---

# Crypto Agent SDK、开放平台、Marketplace 与 Launchpad 架构

<a id="oral-card"></a>

## 要点卡

[返回 P0 知识图谱](../_meta/p0-knowledge-graph.md)

!!! abstract "30 秒回答"

    Crypto Agent 生态不是在 LLM 外包一层钱包 API。我会把系统拆成开发者平面、Agent 控制面、
    Runtime 数据面、Web3 执行面和 Commerce/Trust 面：开发者通过 SDK/CLI 创建版本化 Agent
    与 Skill；控制面负责租户、发布、策略和兼容；Runtime 负责工作流、Memory、MCP/A2A；
    执行面把模型 intent 经过模拟、限额、审批和隔离签名后上链；Commerce 面处理身份、支付、
    分账、信誉和争议。Marketplace 是能力发现和交易，Launchpad 还涉及 Agent/Token 创建、
    发行与持续运营。公共契约要 framework-neutral，并用签名制品、SBOM、权限清单、沙箱、
    版本治理和 conformance test 控制插件与供应链风险。

**3 分钟展开**

1. 稳定开放 API 不直接暴露 LangGraph node、ElizaOS action 或 GAME worker，而是定义平台
   `Agent/Version/Skill/Deployment/WalletPolicy/CommerceOffer`，由 Adapter 接不同框架。
2. Agent、Skill 和 Deployment 分离：发布版本不可变，环境绑定凭据，回滚切换 deployment，
   不能在线改包后继续沿用旧审批和审计结论。
3. 钱包能力默认 deny：模型只产生 intent；策略层校验 chain、asset、contract、calldata、
   amount、slippage、session key、budget 和 HITL，Signer 不接收自然语言。
4. Marketplace/Launchpad 需要发布审核、恶意插件隔离、计费分账、退款争议、Reputation/Sybil、
   Token 机制风险和下架/退出路径，不只是一个搜索页和发币合约。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 发布制品不可静默变化；模型不能直达无限额度签名；平台下架不等于链上资产可撤销 |
| 手画图 | `SDK/Portal → Control Plane → Runtime → Policy/Wallet → Chain`，旁路 `Marketplace/Commerce/Trust` |
| 项目落点 | OctoAgentFlow 讲 Agent 控制面和工作流设计；生产级 Launchpad 类 DEX 讲链上执行、Indexer、分账和团队交付；不要声称两者已在同一生产系统融合 |
| 一个取舍 | 平台抽象提高生态兼容和治理，但过度统一会压平框架创新；稳定核心 + 显式扩展更合适 |

**错误表达**

- ❌ “支持 LangChain、CrewAI、ElizaOS 就等于建成 Agent 开放平台。”
- ✅ “框架只是 Runtime Adapter；开放平台还要有稳定契约、租户/权限、发布、SDK、兼容、计费和运营治理。”
- ❌ “Agent 拥有钱包，所以模型决定后可以直接签名交易。”
- ✅ “钱包属于受策略约束的执行面，模型只能提交有界 intent，签名服务执行代码化 policy。”

**自测追问**：一个已发布 Skill 被发现存在供应链后门，如何阻止新调用、处理运行中任务并追溯影响面？

## 10 分钟版（五个平面 + 生态治理）

### 总体架构

```mermaid
flowchart TB
  subgraph Dev["开发者平面"]
    Portal["portal / docs"]
    SDK["SDK / CLI / templates"]
    Registry["agent & skill registry"]
  end
  subgraph Control["Agent 控制面"]
    Tenant["org / project / env / quota"]
    Version["version / release / deployment"]
    Policy["policy / approval / audit"]
  end
  subgraph Runtime["Runtime 数据面"]
    Gateway["API / bot / MCP / A2A gateway"]
    Workflow["planner / workflow / HITL"]
    Memory["memory / knowledge / model routing"]
  end
  subgraph Web3["Web3 执行面"]
    Intent["typed intent + simulation"]
    Wallet["wallet policy / signer / relayer"]
    Indexer["indexer / finality / reconcile"]
  end
  subgraph Economy["Commerce / Trust 面"]
    Market["marketplace / offer / billing"]
    Launch["launchpad / token / distribution"]
    Trust["identity / reputation / validation"]
    Ledger["ledger / split / refund / dispute"]
  end
  Portal --> SDK --> Registry
  Registry --> Version
  Tenant --> Version --> Gateway
  Gateway --> Workflow --> Memory
  Workflow --> Policy --> Intent --> Wallet
  Wallet --> Indexer --> Workflow
  Registry --> Market
  Market --> Ledger
  Launch --> Ledger
  Trust --> Policy
```

### 核心域对象

| 对象 | 关键字段 | 不变量 |
|------|----------|--------|
| Organization/Project | tenant、owner、plan、region | 跨租户数据和凭据不可共享 |
| Agent | 稳定 ID、owner、metadata | Agent 是逻辑身份，不直接等于进程或钱包 |
| AgentVersion | artifact digest、manifest、schema、runtime | 发布后不可变 |
| Skill/Tool | 输入输出、side effect、权限、成本 | 描述和 annotation 不是授权 |
| Deployment | version、environment、config、replicas | 回滚是切换版本，不覆盖历史 |
| CredentialBinding | provider、scope、secret ref | 密钥不进入 manifest/prompt/log |
| WalletPolicy | chain、asset、contract、limit、approval | Signer 只执行代码化策略 |
| Offer | seller、capability、price、SLA、terms | 能力版本和价格必须可追溯 |
| CommerceJob | buyer/provider、budget、deliverable、state | 支付、交付和争议状态分离 |
| ReputationSignal | reviewer、tag、evidence、window | 原始信号可审计，聚合版本化 |

### SDK 设计：稳定核心，不泄漏框架对象

建议 SDK 分层：

```text
core        AgentRef / Intent / Task / Artifact / Error / Pagination
auth        OAuth/OIDC/API key/wallet challenge
agents      create version deploy invoke observe cancel
tools       MCP-compatible discovery and execution
a2a         remote agent discovery/task adapter
wallet      quote simulate requestApproval submit getReceipt
commerce    offer quote pay job dispute refund
webhooks    signed events 游标回放
```

工程要求：

- OpenAPI/JSON Schema/Proto 作为 wire contract，手写 SDK 只封装语言体验；
- 每次 release 生成 changelog、兼容说明、golden fixtures 和最小示例；
- timeout、context cancellation、pagination、rate-limit、retry-after 和幂等键是一等能力；
- SDK 自动重试仅限已证明安全的请求，创建任务、支付和交易默认不盲重试；
- 错误分为 validation、auth、policy、rate-limit、remote、unknown，不只返回字符串；
- Go、Python、TypeScript SDK 对相同 fixture 产生一致 canonical payload/digest；
- 废弃先 telemetry 观察，再 warning、迁移窗口和 sunset，不能突然删除。

**canonical 必须有可执行契约。** 不能直接对 Go `json.Marshal`、Python `json.dumps` 与
TypeScript `JSON.stringify` 的默认输出求 hash。生产可选择完整 JCS 或固定 schema 的明确编码，
但要固定算法/版本/哈希域、未知字段、重复键、整数、缺失与 `null` 的规则。RFC 8785 的
Unicode 字符串保持原值，不自动做 NFC/NFD 归一化；超出精确浮点范围的金额采用十进制字符串。

配套示例采用**本地 intent-v1 固定字符串 profile，不是完整 JCS**：

- 只允许 `amount/domain/memo/operation/tenant/version` 六个精确字段，全部为字符串且必填；
- domain=`agent-platform/intent`、operation=`pay`、version=`1`；amount 为正 uint256 最小单位的规范十进制文本，无正号/前导零；
- ASCII 字段名固定排序，UTF-8 输出，字符串按 JSON 转义，Unicode 保持原值；拒绝重复键、未知字段、非法 UTF-8/孤立代理项、尾随内容、数字/null/嵌套对象；
- 正文上限 8192 字节，tenant 为 1..128 个 Unicode 码点、memo 至多 1024 个；
- digest=`SHA256(UTF8("agent-platform:wire:v1") || 0x00 || canonicalBytes)`，不与 EIP-712、交易签名或其他业务域混用。

Go/Python/TypeScript 共用 33 组正反向向量，包括 `2^53+1`、最大 uint256、重排/转义、
组合与分解 Unicode、重复键和签名域替换。该 profile 只证明这六个字段的互通，不代表三个完整 SDK 已实现。

### Framework Adapter

不同框架关注点不同：

- LangGraph/OpenAI Agents 等侧重 graph/run、tool、handoff、checkpoint/HITL；
- ElizaOS 使用 actions、providers、evaluators、services 等插件组件；
- Virtuals GAME 将目标、状态和可执行函数组织为 Agent 决策能力；
- Crypto-native 平台还会把钱包、交易、Token 和商业协议嵌入 Runtime。

公共平台只标准化必要语义：

```text
Load(version) → Start(intent) → Observe(run) → Pause/Resume → Cancel → ExportReceipt
```

框架特有 memory、planner、worker 层级放在 extension namespace。Adapter 必须声明：

- 支持哪些 checkpoint/HITL/streaming/cancel 语义；
- 外部副作用是否可查询、幂等和补偿；
- 运行时版本及兼容范围；
- 未支持能力是拒绝还是降级，禁止静默模拟成功。

### Agent/Skill 发布流水线

```mermaid
stateDiagram-v2
  [*] --> Draft
  Draft --> Validating: submit
  Validating --> Rejected: policy / test failed
  Validating --> Reviewing: automated gates passed
  Reviewing --> Published: approval + signed artifact
  Reviewing --> Rejected
  Published --> Deprecated: replacement available
  Published --> Suspended: incident / policy violation
  Deprecated --> Retired: sunset
  Suspended --> Published: same digest + fresh review
  Suspended --> Revoked: malicious / unrecoverable
```

发布门禁：

1. manifest/schema 校验和 canonical digest；
2. 依赖锁定、license、SBOM、漏洞和 provenance；
3. secret/PII 扫描、静态分析和恶意 prompt/instruction 检测；
4. Tool 权限、网络 egress、文件系统、CPU/内存/时间/成本声明；
5. 沙箱动态测试、mock wallet、恶意输入和超时取消；
6. 兼容/回归/conformance 测试；
7. 高风险 Skill 人工审核和签名发布；
8. 上线 canary、kill switch、版本回滚和审计。

Skill 包被签名只证明制品来源和完整性，不证明代码无漏洞。运行时仍需最小权限、沙箱、egress
allowlist 和高风险 action 审批。

**制品审批与部署审批分开绑定。** AgentVersion 固定 artifact、manifest/schema、Runtime 兼容版本
和声明权限的 digest；修复代码或依赖后创建新版本，不能在 Suspended 的旧版本上覆盖文件。
同一制品经调查无需改包时，才允许新一轮审核后恢复。Deployment 审批另绑定 tenant/project/env、
制品版本、配置指纹（不含 secret 明文）、有效权限、WalletPolicy/凭据绑定版本及审批有效期；
配置或策略变化必须重审。生产发布与恢复入口核对真实输入和审批摘要，不能只核对标签或“已签名”。
回滚也要检查目标状态、当前授权和 checkpoint/数据兼容性，禁止回到 Suspended/Revoked 版本。

**进程内插件不是沙箱。** ElizaOS 插件可运行 init、注册 routes/models/adapters 并访问 Runtime
设置；宿主把它加载到共享进程后，Tool permission 声明无法阻止直接读环境或发起网络请求。
可信内部插件与第三方代码要分级准入：没有受验证隔离适配器时拒绝第三方 native 包；可解释的
声明式插件只能执行宿主固定操作。任意代码应进入独立容器/VM，宿主 secret、Signer socket、
Docker socket 不挂载，网络默认拒绝，凭据通过逐操作 broker 按主体/资源发放。容器还需非 root、
只读根目录、禁提权、移除 capabilities、资源上限与有界临时盘；安全声明必须由真实负向探针验证。

### 多租户与凭据

资源层级建议：

```text
organization
  └─ project
      └─ environment(dev/staging/prod)
          ├─ agent deployment
          ├─ credentials
          ├─ wallet policy
          └─ quota / billing
```

- Agent Version 不包含环境 secret；Deployment 只引用 secret manager 中的版本化凭据；
- Tool 从可信 auth context 解析 tenant，忽略模型参数中的 `tenant_id`；
- end user、agent、service account、wallet owner 分别建模；
- 跨 Agent 委托传递 delegation chain、scope、deadline 和 budget，不传万能 token；
- 成本限额同时覆盖 LLM token、Tool/API、链上 gas 和商业支付。

delegation chain 是待验证证据，不是任意 JSON 授权。逐跳校验可信根签发者与签名、主体/代理、
tenant、目标 audience/resource、父授权摘要；权限只能取父级子集，deadline 和额度不得扩张。
限制深度、重复 ID 与主体循环；执行前重验整链过期/撤销状态，不只验最后一个 token。
MCP HTTP access token 必须面向目标资源，不能透传上游万能 token。
并发子任务在同一原子事务中对所有祖先预算预留，用业务 intent 绑定叶授权、实际调用 scope 与金额去重；
UNKNOWN 不自动释放预留。真实多主机平台应持久化授权、撤销与预算，不能依靠各 Worker 内存计数。

### Web3 执行面

```mermaid
flowchart LR
  Model["model / remote agent"] --> Candidate["candidate intent"]
  Candidate --> Normalize["typed canonical intent"]
  Normalize --> Sim["quote + simulation"]
  Sim --> Guard["policy + budget + allowlist"]
  Guard --> Review["HITL if required"]
  Review --> Signer["isolated signer"]
  Signer --> Relayer["nonce / fee / broadcast"]
  Relayer --> Chain["chain"]
  Chain --> Canon["canonical indexer"]
  Canon --> Recon["receipt / reconcile"]
```

Signer 接收的是固定 schema 和 digest，不接收自然语言。策略至少校验：

- chain、account、asset、contract/program、method/instruction；
- amount/notional、slippage、gas/fee、单笔/日累计和速率；
- allowance、delegate/session key、deadline 和 nonce；
- 模拟区块、状态依赖和 simulation freshness；
- recipient 风险、Token/合约版本、pause 和 incident state；
- approval subject 与最终 calldata/message digest 一致。

广播 timeout 进入 UNKNOWN；按链特有 sender/nonce、signature、tx hash 和 finality 查询，不能让
模型重新规划一笔“看起来相同”的交易。

下架、取消和 kill switch 还要区分**未准入 / 已准入 / 结果未知 / 已确认**。停用提升 Deployment
generation，在执行网关与 Signer 的最终准入点校验 generation，阻断已领取任务及旧 Worker；
撤销委托、会话密钥、回调和后续队列调度。只在 Worker 开始前检查一次不能解决竞争窗口。
已签名/已广播的操作仍可能执行，取消响应丢失也不等于没有副作用：保留 UNKNOWN、原 intent 和
原交易证据，禁止重新规划。恢复使用独立、只读、最小权限的对账通道，不重新开放执行权限。
generation gate 的保证截止到准入点；已流出的签名不能靠数据库标记撤回，需按链查询与敞口处置。

### Marketplace、Registry 与 Launchpad 不要混

| 产品 | 核心价值 | 额外风险 |
|------|----------|----------|
| Registry | 发布、版本和发现 Agent/Skill | 恶意包、仿冒、供应链 |
| Marketplace | 报价、购买、调用、评价和分账 | 虚假能力、刷量、退款、争议 |
| Launchpad | 创建/发行/融资/分配 Agent 或 Agent Token | 证券/合规、Rug、价格操纵、流动性 |

Marketplace 的上架对象可以是：

- 托管 Agent 服务；
- 可部署 Agent 包；
- Tool/Skill/MCP Server；
- 数据、模型或工作流模板；
- 带 SLA 的人工/Agent 混合服务。

必须说明买家买的是代码、调用额度、任务结果还是 Token 权益。Token 所有权通常不自动等于：

- Agent 运行控制权；
- IP/模型权利；
- 收益分配权；
- 治理权；
- 对 Agent wallet 的签名权。

这些权利要在合约、平台条款和权限模型中逐项定义，不能靠“Agent Token”一个词概括。

**报价 → 付款 → 权益 → 调用必须绑定同一对象。** 服务端保存不可变 Quote，包含已认证买家/租户、
seller、Offer 修订、能力/制品与权限 digest、价格、链/资产、计价单位、额度、SLA/条款和到期时间。
付款验证器把真实收款对象、金额、资产和业务关联核对为该 Quote 的证据；不能把客户端 `paid=true`
或任意有效钱包签名当作购买证明。以 chain/tx/event 等结算身份防止同一支付重复授予权益。
调用核对买家与已购买版本，卖方新版本不能静默替换旧权益或增加权限；调用额度按 intent 原子去重。
支付 UNKNOWN 不授予权益，也不自动重付；退款/重组阻断后续调用，但不能追回已交付内容。
过期后才查到付款的订单进入专门的核对/退款流程，不要求用户重新付款。

### Launchpad 生命周期

```text
prototype
  → security/review
  → identity + metadata
  → token/ownership configuration
  → launch mechanism + liquidity/distribution
  → runtime deployment
  → revenue/fee routing
  → upgrades/governance
  → suspension/retirement
```

在 DEX/Launchpad 经验上还要增加：

- Agent runtime 与 Token 合约升级是否耦合；
- 收入来自 Agent 服务、交易费还是 Token 激励；
- 分账合约如何处理退款、chargeback 和协议费用；
- 团队/投资人 vesting、LP、价格操纵和 MEV；
- Agent 被停用后 Token、资金库和用户任务如何退出；
- 链下平台下架无法撤销用户已持有的链上资产。

上述顺序不是可直接重跑的脚本。建立稳定的 `(tenant, launch_id)`，绑定 creator/account、链、
发行参数、流动性配置、Runtime 制品和费用路由摘要；每步有稳定 operation key 与参数承诺。
外部调用前持久化 UNKNOWN，返回丢失后只查询原操作，不能更换 launch_id 后重复发行/注资。
适配器必须提供链特有的交易/账户/nonce/mint 等关联及 canonical/finality 证据；
“查不到”或 API 的 launch readiness 检查不证明没有发行，缺少查询能力时转人工核对。

分开保存发行、流动性、Runtime 部署与费用投影：发行已完成而 Runtime 失败，保留原资产与步骤，
从核实后的未完成步骤继续或进入人工接管；不能“回滚 Deployment”就声称撤销了链上资产。
只有明确证明未产生副作用的失败才允许重新执行该步。Saga 补偿也必须是独立授权、可追踪的
资金动作；本地夹具不自动回购/撤池/退款。对账写入以 revision CAS 防过期查询覆盖新状态。
已确认步骤重组时暂停依赖步骤、转 `review_chain`，对费用投影追加幂等冲正，禁止整条流水线重跑。

### Commerce 与 Trust

- ERC-8004 可作为身份/反馈/验证锚点，但 Draft 和 Sybil 边界必须显式；
- x402 适合短请求，job escrow 适合长交付；两者都进入统一 Commerce Ledger；
- 绑定真实完成/支付/退款证据可提高伪造评价的门槛，但是否提高刷量成本取决于不可复用性和真实经济成本，仍不能完全防 Sybil；
- 平台抽成、开发者应收、Evaluator fee、Affiliate 和退款分别记账；
- Reputation 聚合算法、reviewer 集合和时间窗口都要版本化；
- 高风险钱包权限不能由 Marketplace 星级自动放开。

信誉证据需要 job 与结算事件唯一关联，可信投影 revision 单调递增，拒绝过期观察覆盖新状态；
退款、撤销及链重组后重新计算有效权重，不能删除原信号后
释放证据给新账户再次评价。多地址不等于多个经济主体；已知相同经济主体自评应排除，但主体聚类
本身有误差，不能声称完全防 Sybil。低成本自成交、全额退款和同一交付结果反复包装仍需额外治理。

## 本地可运行示例与故障验证

仓库根目录执行（Go 1.24.2+、Python 3、Node 22.15+；TS 通过 Node type stripping 运行，无 npm 依赖）：

```bash
go run ./examples/senior/agentplatform/cmd/demo
go test -race -timeout 60s ./examples/senior/agentplatform/...
python3 examples/senior/agentplatform/wire/verify.py
# 独立验证真实 Linux 容器边界，需 Docker 和本地 alpine:3.22.1 镜像
python3 examples/senior/agentplatform/verify_sandbox.py
```

demo 预期 `passed=true`、`launch_effects=3`（发行/流动性/部署各一次）、
`fee_before_reorg=10`、`fee_after_reorg=0`、`launch_state="review_chain"`，其他拒绝/恢复布尔值均为 true。

| 问题 | 可执行验证 |
|------|------------|
| 审批与不可变版本 | 制品/manifest/Runtime/权限替换拒绝；配置须另审；同 digest 复核恢复，新制品须新版本，Revoked 不可回滚 |
| 插件与真实隔离 | native 包拒绝；broker 拒绝 secret/网络/签名能力；容器内真实验证 secret/socket 不可见、只读根、无外部路由、非 root 与禁提权 |
| 委托与预算 | 伪签名/错签发者/父摘要、租户/受众/资源、扩权/续期/扩额/主体循环/过期/父撤销拒绝；32 路子任务只获总额度 8 |
| 停用与取消 | claim 后停用、切回同版本 ABA、Stop/Submit 竞争、已准入取消保持 UNKNOWN，撤权后仍能只读核对 |
| Marketplace 权益 | 报价后新版本不替换权益、跨租户/主体拒绝、支付复用/退款/重组拒绝，32 路调用最多消费额度 4，相同 intent 不重复扣额度 |
| Launchpad 恢复 | 回包丢失与真实子进程 exit73 后重建恢复、并发实例去重、部分部署失败、坏库拒绝、错回执/未确认/查不到/RPC 错不重发；重组只冲正一次 |
| SDK 编码 | 33 组共享向量，三个语言实现逐字节与 digest 对照；非法输入统一拒绝 |
| 信誉反刷量 | 同 job/支付事件不可复用、主体不符/自评拒绝、退款/重组移除权重，恢复 canonical 后只恢复原信号 |

完整操作、测试组与限制见[示例说明](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/agentplatform)。
控制面、委托、Marketplace 与信誉为 mutex 内存模型；只有 Launchpad 与独立模拟远端账本使用
Linux/macOS flock + fsync/rename 持久化。管理函数、Principal、时间及证据适配器都是可信宿主输入，
未实现生产登录/OAuth、完整 SDK、真实框架插件适配、Signer/RPC/Token 发行或生产 IAM。
容器探针独立验证该运行配置，未冒充 ElizaOS 沙箱验收；费用为固定教学投影，不是完整资金总账。
`review_chain` 保守阻断并留给人工处置，没有自动重新发行或重新开放签名权限的入口。

### 开发者体验与生态指标

平台是否成功不能只看 Agent 数量：

| 阶段 | 指标 |
|------|------|
| Onboarding | time-to-first-agent、首次部署成功率、文档搜索失败 |
| Build | SDK error、schema/conformance failure、测试环境稳定性 |
| Publish | review lead time、拒绝原因、漏洞/恶意包发现率 |
| Run | task success、P95 latency、unknown、cost、HITL rate |
| Economy | paid active agents、真实成交、退款/争议、开发者留存 |
| Safety | 越权拦截、重复支付、钱包损失、kill-switch MTTR |

下载量、注册 Agent 数和 Token 市值容易被刷，不能单独作为生态健康指标。

### 竞品研究应该看什么

不要背宣传口号，按同一张表拆解：

| 维度 | 观察问题 |
|------|----------|
| Runtime | planner、memory、workflow、checkpoint、multi-agent |
| Extension | Tool/Plugin/Skill 模型、权限和发布 |
| Wallet | 托管/非托管、chain、session、policy、recovery |
| Commerce | 定价、支付、escrow、分账、退款 |
| Launch | Agent/Token 创建、流动性、治理和退出 |
| Openness | SDK、API、MCP/A2A、导出、协议/数据可迁移性 |

例如 ElizaOS 官方文档强调插件的 actions/providers/services 等组件；Virtuals GAME 强调目标、状态、
函数和决策引擎；Bankr/ClawPump 的公开资料则可以用于观察 Agent API、钱包执行和 MCP-first
Crypto 操作。讲解时应说明“研究其公开接口和技术路线”，不要把未读源码或未运行环境说成深度
生产经验。

## 生产场景

开发者发布一个跨链资产管理 Agent：

1. CLI 生成 manifest、Tool schema 和 WalletPolicy，CI 产出 SBOM 和签名 artifact。
2. 沙箱只连接 test wallet，验证恶意 prompt、超额交易、重复 submit 和 provider timeout。
3. 审核通过后发布不可变 AgentVersion；生产 Deployment 绑定独立凭据和 session key。
4. 用户通过 Marketplace 购买服务，Commerce Ledger 记录报价、支付和交付。
5. 每个交易 Intent 独立经过 simulation、budget 和审批；Signer 执行有界交易。
6. Indexer 确认 canonical receipt，收入按规则分账；UNKNOWN、退款和争议进入独立状态机。

## 排查与观测

- `org/project/env/agent/version/deployment/run/intent/wallet/tx` 全链路关联；
- 统计不同框架 Adapter、SDK 版本和 Skill digest 的故障率；
- 供应链事故反查所有受影响 Deployment、运行任务、凭据和钱包动作；
- 发布、策略、审批、签名、交易、支付和分账分别保存审计事件；
- 运行时日志不包含 secret、私钥、完整 wallet signature、敏感 memory 或跨租户 prompt。

## 架构取舍

| 方案 | 优点 | 风险 |
|------|------|------|
| 全托管 Agent + Wallet | 开发体验好、平台易观测 | 托管、密钥、合规和平台集中风险 |
| 自托管 Runtime，平台只做 Registry | 开放、故障域分散 | 兼容、SLA、计费和安全难统一 |
| 混合控制面 + 可插拔执行面 | 平衡治理和生态开放 | 协议、身份与审计映射复杂 |

技术负责人需要先写清信任假设、资金边界和可迁移性，再选择托管程度；不能先选框架，再让安全和
商业模型追着实现补洞。

## 深挖问答

1. **开放平台和 Agent 框架的区别？** → 框架解决运行时编程，平台还负责租户、发布、SDK、权限、计费、兼容和运营。
2. **为什么 AgentVersion 要不可变？** → 保证审批、审计、回滚和事件都能还原当时实际代码与 schema。
3. **如何做到 vendor-neutral？** → 公共 canonical contract + Adapter + 扩展协商 + 跨 SDK conformance，而不是假装所有框架能力相同。
4. **Skill 签名后能否直接信任？** → 只能证明来源/完整性，还需漏洞扫描、沙箱、最小权限和运行时策略。
5. **Marketplace 与 Launchpad 有什么区别？** → 前者核心是能力/服务发现交易，后者还包含创建、发行、分配、流动性和持续治理。
6. **如何阻止模型盗走钱包资金？** → 模型不持根密钥，只提交 typed intent；Policy、审批、session limit、隔离 Signer 和链上对账共同约束。

## 反模式与事故

- 公共 API 直接暴露框架 checkpoint 数据结构，框架升级导致所有 SDK 破坏性变更。
- 在线修改已发布 Skill 包但版本号不变，旧审批无法证明批准了什么代码。
- MCP Tool 声明 `readOnly` 就免审，实际实现可转账或外发数据。
- 所有 Agent 共用一个平台热钱包/API key，单个租户注入导致横向失陷。
- 下架恶意 Agent 但未吊销 Deployment、session key 和 webhook，后台继续运行。
- 只按 Token 市值和 Agent 数衡量生态，忽略真实调用、退款、争议和安全事故。

## 延伸阅读

- [MCP Specification](https://modelcontextprotocol.io/specification/2025-11-25)
- [A2A Protocol](https://a2a-protocol.org/latest/)
- [ElizaOS Plugin Architecture](https://docs.elizaos.ai/plugins/architecture)
- [Virtuals GAME Framework](https://whitepaper.virtuals.io/builders-hub/game-framework)
- [Bankr Agent API](https://docs.bankr.bot/agent-api/overview/)
- [ClawPump Documentation](https://clawpump.tech/docs)
- [RFC 8785：JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html)
- 关联：[S-AI-09 Agent 工作流与 HITL](./S-AI-09-agent-workflow-hitl-publishing.md)、
  [S-ARCH-15 API 版本与发布](../03-system-design/S-ARCH-15-release-strategy.md)、
  [S-SEC-03 SBOM 与供应链](../21-security-engineering/S-SEC-03-sbom-provenance-release-admission.md)
