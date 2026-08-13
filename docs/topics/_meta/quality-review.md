# 知识库质量审查与领域覆盖差距

> 全库 **235 篇**（以 [topics.yaml](topics.yaml) 为准）
>
> 本轮审查与补强日期：**2026-08-13**
>
> 审查范围：资深 Golang、后端/解决方案架构，重点领域为 AI/Crypto Agent、
> Web3、交易所、钱包、链基础设施与支付
>
> 2026-07-18 已启动角色 P0 深度纠错，首批 25 篇的逐项结果见
> [P0 技术纠错审计](technical-corrections-audit.md)。

## 本轮结论

- 已逐篇复核原有 153 篇 published 正文；其中 **152 篇完成技术修正**。未修改的
  [S-LEAD-01](../07-engineering-leadership/S-LEAD-01-incident-postmortem.md)
  未发现需要纠正的技术结论。
- 第一阶段在修正基线之上新增 **30 篇 P0 正文**，分别落入
  [16 Go 生产工程](../16-go-production-engineering/index.md)、
  [17 多链钱包](../17-multichain-wallet/index.md)、
  [18 Web3 支付](../18-web3-payments-stablecoin/index.md)、
  [19 节点/RPC/Staking](../19-node-rpc-staking/index.md)，并增强编码练习、Linux/TCP 与 MySQL。
- 本阶段再新增 **8 篇正文**：可运行确定性撮合与 WAL 回放、Rollup/跨链安全，以及
  Solana、Cosmos、Aptos、Sui 四条非 EVM Go 实战；其中 4 篇为 P0、4 篇为 P1。
- 第三阶段新增 **8 篇 P1 正文**：Ethereum PoS/fork choice、CometBFT、
  Blob/PeerDAS、协议升级/状态迁移，以及行情 gap recovery、FIX session、STP、
  集合竞价与撮合性能验证；同时新增 3 个可运行模块并升级撮合器 STP。
- 第四阶段继续新增 **8 篇 P1 正文**：威胁建模/IAM、key ceremony 与 signer fencing、
  SBOM/provenance、安全测试与事件响应，canonical backfill/realtime merge、trace/state diff
  与数据质量、非 EVM 在线生命周期故障注入，以及机构托管/DvP/RWA/ISO 20022；新增
  `signerfencing`、`chainmerge`、`txlifecycle` 三个 race-tested 模块。
- 第五阶段完成两项项目化集成：`signer-project` 以 bbolt 落地 crash-safe fence/receipt，
  接通 SoftHSM2 PKCS#11 与真实 2-of-3 FROST 协议 sandbox；四条非 EVM module 增加
  localnet/testnet endpoint adapter、链身份校验、deterministic contract test 与 opt-in smoke。
- 第六阶段进入生产化验证：增加 etcd 多副本线性一致 fence、existing-key-only PKCS#11
  硬件验收器、mTLS 跨进程 FROST participant 与持久化 session ledger，并为四链建立固定
  N/N-1 source/commit/binary provenance、Toxiproxy 故障和升级 harness。当前已实跑
  CometBFT 0.38.22→0.38.23 状态复用、停启和 latency/timeout/reset；真实厂商 HSM 及
  Solana/Aptos/Sui 节点仍受本机设备/二进制条件约束，不能冒充已验收。
- 第七阶段新增 **8 篇正文**：PostgreSQL 3 篇、Terraform/IaC 与 Helm/GitOps 2 篇、
  链数据 ClickHouse/lakehouse 1 篇、Staff 技术战略与跨团队迁移 2 篇。全库优先级改为
  [七类领域轨道](role-priority-matrix.md)，旧全局 tier 只保留兼容。
- 第八阶段按候选人简历新增 **4 篇正文**：Agent 工作流/HITL 与可靠发布、Persona/Memory
  与反馈治理、TRON/TRC20 钱包生命周期、CDC/Flink/ES 实时风控数据平台；新增
  `AI Agent Platform / Infrastructure` 首选岗位轨道，并将方向定向 P0 收敛为
  shared 40 篇 + 角色增量 20 篇。
- 第九阶段按预测市场技术负责人 JD 新增 **4 篇正文**：CTF/Outcome Token/市场生命周期、
  CLOB-first/EIP-712/链上结算、体育电竞数据源/乐观预言机/争议仲裁，以及安全不变量/
  测试矩阵/主网上线；四篇均为 `exchange_engineering` 与 `staff_architect` P0。
- 第十阶段按 Crypto AI Agent 生态技术经理/架构师 JD 新增 **4 篇 P0 正文**：MCP/A2A
  跨框架互操作、ERC-8004 身份/信誉/验证、x402/x402b/ERC-8183 Agent Commerce，以及
  Agent SDK/开放平台/Marketplace/Launchpad；`AI Agent Platform` 轨道升级为
  `AI Agent Platform / Crypto Agent Ecosystem`，增量 P0 从 20 篇扩展为 24 篇。
- 第十一阶段按候选人 Launchpad 类 DEX 项目补充 **1 篇方向定向 P1 正文**：PancakeSwap
  V2/V3 的池模型、集中流动性、fee tier、position NFT、报价、Indexer 和毕业迁池差异。
- 第十二阶段按 CEX/DEX 多级代理履历补充 **1 篇方向定向 P1 正文**：极差费率、代理树绑定、
  异步计佣、佣金账本结算与代理后台 subtree 隔离，并对照 CEX 手续费事实与 DEX Indexer 输入。
- 第十三阶段按 DEX Tech Lead JD 补充 **3 篇方向定向 P0/P1 正文**：DeFi Staking/
  流动性挖矿/Yield Farming、Uniswap V2/V3 协议深挖、DEX Tech Lead 45 分钟架构白板。
- 第十四阶段补充 **2 篇解释型正文**：经典共识与链上共识对照（S-PROTO-05），以及
  EVM 独立链、侧链、Optimistic/ZK Rollup 的全景分类与 Go 接入差异（S-BC-14）。
- 证据标签已覆盖 235 篇：`explanation_only=177`、`illustrative_artifact=31`、
  `deterministic_test=20`、`integration_harness=7`、`external_acceptance=0`。
  最后一项为零是诚实边界，不把仓库 harness 写成真实厂商或生产验收。
- 修正目标不是统一文风，而是删除绝对化、版本过时或会导致错误追问答案的表述，并修复相关 Go 示例。
- 当前知识库的优势是 **Go 核心、分布式系统、中间件、EVM/Solidity、CEX/DEX 架构广度**。
- 原先“过度偏 EVM 与交易所概念架构”的主要缺口，以及 PoS/BFT、PeerDAS、
  行情/FIX/STP/竞价、安全工程、机构金融与链数据质量的第一轮深度已补齐。当前剩余短板转为：
  **厂商硬件上的 HSM/HA 实机证据、MPC coordinator 高可用与 share 防回滚、三条大型非 EVM
  节点的真实 localnet 执行语料、机构 rail 的 scheme-specific 实战、大规模列存/lakehouse
  的容量与成本 benchmark，以及候选人本人可核验的 Staff 案例指标**。本轮完成的是知识结构、
  可运行实现和可验收 harness，不把它们夸大成生产托管集群、四套全部实跑的完整链节点或
  用户亲历项目。

## 实际题目分布

| 模块 | 题数 |
|------|-----:|
| Go 并发与运行时 | 20 |
| Go 内存与 GC | 15 |
| Go 生产工程 | 6 |
| 系统设计 | 21 |
| 数据库与中间件 | 26 |
| 网络治理 | 7 |
| 工程领导力 | 5 |
| 资深编码练习 | 7 |
| 云原生 | 10 |
| AI 工程 | 14 |
| 解决方案架构 | 8 |
| 区块链 Web3 | 14 |
| 多链钱包与托管 | 12 |
| Web3 支付与稳定币 | 6 |
| 节点、RPC 与 Staking | 10 |
| 协议、共识与安全 | 5 |
| Web3 安全工程 | 4 |
| Solidity | 8 |
| DEX/CEX/预测市场 | 31 |
| 交易所微服务 | 6 |
| **合计** | **235** |

原始 153 题基线中的中间件为 21 题；此前新增 2 篇 MySQL，本阶段再新增 3 篇 PostgreSQL，
当前数据库与中间件共 26 题。

## 已修正的主要技术问题

### Go 运行时、并发与内存

- 调度器：修正 P/M 绑定、阻塞 syscall 的 retake/handoff、runq work stealing
  方向、无 P 的 M 能否执行 Go 用户代码等实现细节。
- 版本语义：补齐 Go 1.23 timer channel、Go 1.25 容器感知
  `GOMAXPROCS` 与 `WaitGroup.Go`、Go 1.26 Green Tea GC、
  `goroutineleak` profile、`new(expr)` 和泛型自引用约束。
- 并发原语：修正“无缓冲 channel 代表业务已处理”“`sync.Map.Range`
  不能修改 map”“`sync.Pool` 每次 GC 立即全清空”等错误表述。
- 内存模型：避免把 Go data race 简化成 C/C++ 式“任意未定义行为”；明确
  DRF-SC、happens-before 与 race detector 只能发现已执行路径。
- GC：区分 tracing/mark-sweep 正确性模型与 Green Tea 的 page/span
  扫描组织优化；不把 Go 1.26 误说成分代或移动式 GC。

### 分布式系统与中间件

- 幂等、消息投递与事务：明确“至少一次 + 业务幂等”，不把 Kafka 事务、
  RocketMQ 事务消息或数据库事务扩大成跨外部系统的 exactly-once。
- Kafka：修正 `acks=all`、ISR、幂等 producer、分区内顺序和
  `max.in.flight` 的边界。
- Redis 锁：增加租约过期、fencing token、主从切换与“锁不能替代数据库原子条件更新”。
- 延迟任务、缓存、熔断、限流、多活：删除固定阈值和单一实现等绝对化结论，
  补充时钟、重试、回源保护、降级与可观测边界。

### EVM、索引器与托管钱包

- Ethereum：补充 typed transaction、EIP-1559、EIP-7702、
  `latest/safe/finalized` 和执行层状态语义。
- RPC/订阅：WebSocket 只作为实时提示，不是持久消息队列；断线后必须从持久
  扫块水位回补。
- Reorg：从“删除固定 N 个块”改为校验 `parentHash`、寻找共同祖先、回滚派生状态并重放。
- 身份模型：链上 observation 使用
  `(chain_id, block_hash, tx_hash, log_index)`；业务幂等不能简单把每个
  `block_hash` 当成一笔新充值。
- 签名与托管：MPC/HSM/KMS 不能替代交易策略、额度、目标地址/方法白名单、
  nonce reservation、审计和灾难恢复。
- ERC-4337、代理、ABI/event：按 EntryPoint/代理版本区分，避免用一套字段或
  EIP-1967 slot 概括所有实现。

### Solidity、DeFi 与 MEV

- 修正 `indexed` event、动态类型 topic、代理升级模式、gas 优化版本边界。
- 将“防重入 = 加一个 modifier”“slippage 一定防 sandwich”“模拟成功 =
  上链成功”等表述改为威胁模型和状态变化语境。
- 强化 invariant、fuzz/property test、权限/升级治理、oracle 与闪电贷组合风险。

### CEX/DEX、账本与衍生品

- 撮合按市场/订单簿保持确定性顺序，但“撮合和所有仓位必须在同一个线程”
  不是通用架构；跨品种全仓风险通常还需要 account/risk shard 串行化。
- 区分成交事实、清算/账本事实、派生行情和可重建读模型。
- 账本按币种守恒，不能把 BTC 与 USDT 数量直接相加证明平衡；PoR 也不能只证明
  链上资产而不证明用户负债和表外义务。
- 修正 linear/inverse 合约 PnL、减仓跨零、reduce-only 超卖、强平优先级和
  保险基金/ADL 的边界。
- 提现广播超时应进入 **unknown** 状态并查链/节点，不能直接标失败后重签；
  nonce/UTXO、余额预占和已签 raw transaction 都需要持久化恢复。
- 修复并增强连接池、令牌桶、errgroup、优雅关闭、LRU 等示例的并发和生命周期问题。

### 共识、DA 与协议升级

- 将 Ethereum PoS 从“最长链/固定确认数”修正为 LMD-GHOST head、Casper FFG
  checkpoint finality 与 weak subjectivity 三个边界。
- 明确 CometBFT `+2/3 prevote`、`+2/3 precommit` 和 commit 不同；阈值按
  voting power，`1/3+` 可阻断活性。
- 修正 PeerDAS 时态：EIP-7594 已随 Fusaka 于 2025-12-03 主网上线，不能再写成
  未来方案；blob target/max 也不能继续背 EIP-4844 初始参数。
- 区分 KZG 完整性、PeerDAS 概率可用性、Rollup 状态有效性和长期归档。
- 将协议激活、共识状态迁移、本地 DB schema 与外围 decoder 拆开；新规则区块最终化后
  不能把旧二进制/旧快照称为普通无损回滚。

### 行情、FIX、STP 与竞价

- 行情本地簿采用 subscribe-buffer-snapshot-bridge；epoch 可由 venue 提供或由 adapter
  绑定一次恢复 generation；gap、非法增量或 checksum 异常后 fail closed，不再允许
  “跳过一条继续发布 BBO”。
- FIX 明确跨连接 `NextNumIn/NextNumOut`、application retransmission、
  `PossDup/OrigSendingTime` 与 `SequenceReset-GapFill`。
- 撮合器新增受信任 account scope 与 cancel-maker/taker/both；FOK 预检模拟 STP，
  STP 不生成成交、手续费或账本过账。
- 集合竞价清算价按最大成交、最小 imbalance、参考价距离和显式最终 tie-break；
  不把某 venue 的 order eligibility/collar 泛化为行业规则。
- 性能结论要求声明固定工作负载、持久化边界、p99/p999 与环境，使用重复 benchmark +
  benchstat；不让订单簿随 `b.N` 无限增长，也不以空簿 microbenchmark 宣称端到端 QPS。

### 安全工程、链数据与机构金融

- 把“有 HSM/MPC”修正为四层控制：身份与授权、准确 intent/policy binding、signer-side
  fencing/idempotency，以及密钥材料保护；旧 leader 必须在最终签名边界被 epoch 拒绝。
- 区分 SBOM、provenance、artifact signature 和 admission policy；按 SLSA v1.2 Build Track
  使用 L1/L2/L3，不再沿用旧单轨 SLSA 1～4；SPDX 3.1 截至 2026-07-17 仍为 RC。
- Backfill/realtime 不按高度覆盖：raw evidence 按 hash 保留，候选 head 沿 parent lineage
  原子 adopt，finalized 以下 fail closed，handoff 按连续 hash overlap 而非 `MAX(height)`。
- Trace/state diff 被明确为 client/tracer/provider-specific 的重执行观测；缺失或超时不能解释成
  “没有 internal call”，decoder 需绑定 code/package/protocol 与版本并可从 raw 重建。
- 非 EVM 广播 timeout/not-found 统一保持 UNKNOWN，准入拒绝与 committed execution failure
  分开，成功/失败使用对称终态门槛；只有未观察执行且过期已被链特有证据证明时才允许刷新
  资源重建。Sui effects finality 与 checkpoint 分离，并纳入 2026 年 7 月 JSON-RPC →
  gRPC/GraphQL 迁移事实。
- ISO 20022 被修正为业务模型和消息标准，不是结算网络；区分 transport ACK、scheme status、
  settlement account movement、legal finality，以及 RWA token 与法定登记/受益权。

## 对外表述时应避免的错误表达

| 不建议这样说 | 更准确的表达 |
|--------------|--------------|
| Go 1.26 改成了分代 GC | Green Tea 仍是 tracing mark-sweep，优化 page/span 级标记与扫描局部性 |
| 无缓冲 channel 发送成功说明对方处理完成 | 只说明值已交给接收操作；业务完成需要单独 ACK |
| `sync.Pool` 每轮 GC 一定全部清空 | current local 转 victim、旧 victim 丢弃；任何对象都可能随时拿不到 |
| Kafka exactly-once 能保证数据库不重复写 | Kafka EOS 只覆盖其事务边界，外部副作用仍需幂等、outbox/inbox 或补偿 |
| 所有链等 12 个确认就安全 | 按链的 finality 模型、资产价值和风险策略选择 safe/finalized/确认数 |
| `tx_hash + log_index` 足以记录所有 reorg 历史 | observation 还要带 block hash；业务幂等与链上观察 identity 分层 |
| MPC 钱包天然安全 | MPC 只改变密钥/签名信任分布，仍需策略、审批、隔离、恢复与审计 |
| 提现广播超时就是失败，可以直接重签 | 先进入 unknown 并查询 tx/nonce/UTXO，避免重复广播或双花 |
| 撮合、仓位和全仓风控必须在同一线程 | 订单簿按 market shard 确定性处理，账户级风险按 margin unit 串行化并定义一致性协议 |
| PoR 能证明交易所偿付能力 | 还需 proof of liabilities、负债完整性、控制权和表外义务审计 |
| Solana `sendTransaction` 返回 signature 就表示成功 | 只表示 RPC 接受提交；继续查询执行错误与 required commitment |
| Sui 的所有 Coin 和 Gas 永远都必须选择 Coin Object | Coin Object 仍存在；支持资产可使用 Address Balance/hybrid，当前 gasless stablecoin transfer 由 Address Balances 驱动 |
| L2 交易 finalized 就表示跨链提现可领取 | 交易 finality 与 withdrawal prove/challenge/claim 是不同状态 |
| 跨链消息用源链 tx hash 去重就够了 | 必须认证准确 event/message，并绑定 source/destination domain、emitter、payload 与 nonce |
| Ethereum PoS 就是最长链，等 12 块就最终 | LMD-GHOST 选 head，FFG 推进 checkpoint finality；按 safe/finalized 与风险策略处理 |
| CometBFT 收到三分之二投票就最终 | 说明 height/round、prevote/precommit；commit 需要同轮某块的 `+2/3 precommit` |
| PeerDAS 还是未来方案 | 已随 Fusaka 于 2025-12-03 主网上线；当前节点按协议 custody/sample columns |
| KZG proof 证明 blob 一定可用 | KZG 证明片段与 commitment 一致；DA 还依赖 custody、采样、纠删码和网络假设 |
| FIX 重连后 sequence 从 1 开始 | session sequence 跨 TCP 连接持久化，按 ROE 才能重置 |
| 有 STP 就不会有 wash trading | STP 只防配置 scope 内直接自成交，仍需关联账户与市场操纵 surveillance |
| 集合竞价取成交量最大价格即可 | 并列时还需 imbalance、reference/collar 与 venue-specific tie-break |
| 用了 HSM/MPC 就不会盗签 | 它们保护/分散密钥材料；signer 仍需验证身份、intent、policy、epoch、幂等和额度 |
| 生成 SBOM 就完成供应链安全 | SBOM 只说明组成；还需可信 provenance、签名身份授权、digest 准入和漏洞/运行时控制 |
| Backfill 和实时流按高度 UPSERT 即可 | observation 按 hash 保留，canonical assignment 沿 parent lineage 提交并保护 finalized 水位 |
| Trace 为空说明没有内部调用 | trace 可能因 client、pruning、reexec、timeout 或 provider 限制缺失，必须记录 coverage/error |
| Cosmos sync 返回 code 0 就已上链成功 | `sync` 只等待 CheckTx；继续查询 committed execution result |
| Sui 必须进 checkpoint 才有交易最终性 | effects 可在 checkpoint 前达到协议 finality；是否等 checkpoint 是业务证据策略 |
| ISO 20022 ACK 就表示钱已结算 | ISO 20022 是消息标准；ACK、业务状态、资金腿和法律 finality 是不同证据 |
| RWA token holder 天然就是法律所有人 | 权利取决于发行结构、登记簿、合同、托管与司法辖区，必须持续对账 |

## 近 6 个月岗位样本映射

本轮抽样以 2026-07-16 仍可访问的在招或近期发布岗位为主：

- [Binance Wallet Blockchain Engineer](https://jobs.lever.co/binance/b20adcce-7e6a-4d8b-b178-f55c521fe097)：
  链上采集、解析、索引与分析，要求 Bitcoin、Ethereum 等主流公链。
- [Crypto.com Senior Golang Developer, Onchain Wallet](https://jobs.lever.co/crypto/12fb33f6-6409-4c74-a17d-9fb2283f88d2)：
  40+ 链，明确列出 EVM、BTC、Solana、Cosmos、Sui。
- [Galaxy Senior Backend Developer](https://job-boards.greenhouse.io/galaxydigitalservices/jobs/6017789004)：
  Ethereum execution/consensus client、节点、indexer、staking/validator、Kubernetes、
  key management 与 spec-first/AI-assisted workflow。
- [Jump Crypto Production Engineer](https://job-boards.greenhouse.io/jumpcrypto/jobs/6472539)：
  validator、light client、RPC node、PoS、Linux/Kubernetes/bare metal 和安全密钥管理。
- [Ava Labs Staff Backend Engineer, Institutional Custody](https://job-boards.greenhouse.io/avalabs/jobs/5628550004)：
  MPC/multisig、UTXO/account model、共识、安全编码与解决方案架构。
- [Hyphen Chain Protocol Engineer](https://job-boards.greenhouse.io/hyphenconnect/jobs/4975326007)：
  Cosmos SDK/Tendermint、Evmos、共识/网络、链升级、pruning/sync/RPC、property test。
- [Hyphen Senior Software Engineer, Web3 Payments](https://job-boards.greenhouse.io/hyphenconnect/jobs/5026250007)：
  签名请求、replay/idempotency、多链 settlement、KMS/HSM、stablecoin、确认/重试/故障转移。
- [Ondo Senior Backend Engineer](https://job-boards.greenhouse.io/ondofinance/jobs/4296894009)：
  RWA、托管/KYC/支付集成、EVM 与 non-EVM、审计性和可观测性。
- [Crypto.com Senior Golang Developer, Equities](https://jobs.lever.co/crypto/2b31ba5f-853a-4ab6-8573-87c9e552f3a4)：
  高性能交易、股票/衍生品、event sourcing 与 TDD。
- [Alpaca Senior Software Engineer, Payments & Treasury](https://job-boards.greenhouse.io/alpaca/jobs/6113944004)：
  payment lifecycle、treasury、双重记账、对账、事件驱动与 ISO 20022。
- [CoinsPaid Senior Golang Engineer](https://builtin.com/job/senior-golang-engineer/9331587)：
  Kafka/NSQ/NATS/Rabbit、Kubernetes、event sourcing、可观测性与 crypto payment。

## 知识图谱补强优先级（实施状态）

### P0：本轮落地与剩余项

0. **[已完成] Go 工程门槛**
   - 错误契约、包边界、测试、Fuzz/Benchmark/Race、Modules/Toolchain、
     静态分析与供应链。
   - 增加 Singleflight、有界批处理、Linux/epoll/TCP、复杂 SQL 与资金表。

1. **[已完成] 多链钱包与 chain adapter**
   - Bitcoin UTXO：coin selection、change、fee rate、RBF/CPFP、mempool、
     confirmation、地址与脚本类型、PSBT。
   - Solana：account model、PDA、blockhash/nonce、commitment、versioned
     transaction、priority fee 与程序日志索引。
   - Cosmos：SDK module、CometBFT/Tendermint、IBC、sequence、gas、升级。
   - Sui/Aptos：object/resource model、并行执行、交易版本与索引差异。
   - 统一 adapter：构造、模拟、签名、广播、确认、replacement、reorg/finality。
   - 增加 Solana 社区 SDK、Cosmos 官方 SDK、Aptos 官方 SDK 的离线 transaction
     vectors，以及 Sui capability-aware reservation 示例；明确 SDK 身份与版本约束。
   - 四个 module 已增加可配置 localnet/testnet endpoint adapter、链身份 pin、wire contract
     test 和 opt-in read-only/broadcast smoke；Sui 在线路径使用当前 GraphQL，无 deprecated
     JSON-RPC fallback。

2. **[已完成第一轮] 节点、RPC、验证者与 staking 基础设施**
   - Ethereum execution layer / consensus layer、full/archive/pruned node、sync。
   - mempool、P2P、RPC gateway、请求合并/缓存/限流、多 provider 一致性。
   - validator lifecycle、slashing、withdrawal credential、密钥迁移、升级与灾备。
   - MEV-Boost/builder relay、深层 P2P/mempool 与 key ceremony 仍放入后续协议专题。

3. **[已完成] 生产级托管与钱包安全**
   - MPC/TSS 的 DKG、reshare、quorum、round failure、participant replacement。
   - HSM/KMS/MPC 选型、冷热分层、地址生成、归集、UTXO/nonce reservation。
   - policy engine、审批、限额、allowlist、签名审计与密钥恢复演练。

4. **[已完成] 稳定币支付、资金与清结算**
   - payment intent/state machine、webhook、replay/idempotency、退款/冲正。
   - 主流稳定币多链 settlement、issuer freeze/blacklist 风险、跨链转移。
   - treasury、liquidity/rebalancing、double-entry ledger 与 reconciliation。
   - KYC/KYB、Travel Rule、制裁筛查的系统边界与审计数据流。

5. **[已完成第二轮，待继续外围] 交易系统的可运行深度**
   - 已增加最小可运行 matching engine + WAL/snapshot/replay + deterministic/race test。
   - 已覆盖价格时间优先、GTC/IOC/FOK/Post-only、稳定事件 ID、torn tail 与 checksum
     损坏边界；明确教学实现不等于生产级 STP/竞价/高性能数据结构。
   - 已新增 market-data sequence/snapshot bridge/gap recovery 与 FIX session 恢复。
   - 已新增 STP cancel policy、集合竞价清算/分配和 Go microbenchmark/benchstat 方法。
   - account/risk shard、cross-margin 一致性、清算、对账和故障注入。
   - 后续继续 CPU cache、分配、GC、网络内核与带 WAL/网络的端到端压测。

### P1：架构师和基础设施岗位拉开差距

1. **[共识/DA 已完成第一轮] 协议、共识与 Rollup 内部**：已补 PoS/BFT、
   LMD-GHOST/FFG、weak subjectivity、EIP-4844/PeerDAS、协议升级/状态迁移，以及
   Rollup finality、DA、proof、forced inclusion、桥认证/重放/限额；后续可继续
   batcher/proposer/challenger、PBS/MEV 与协议级故障注入。
2. **[生产化实现已完成一轮，实机验收待补] 安全工程体系**：已补 STRIDE/攻击树、IAM/secrets、
   signer fencing/key ceremony、SBOM/SLSA provenance、fuzz/property/differential test 与
   NIST SP 800-61r3 事件响应；`signer-project` 已实现 bbolt 持久 fence、崩溃窗口测试、
   SoftHSM2 PKCS#11 与真实 FROST DKG/门限签名 sandbox；生产化路径另实现 etcd
   lease/mutex/CAS 多副本 fence、existing-key-only HSM acceptance，以及 mTLS 独立 participant
   与持久 session ledger。etcd 只线性化 metadata/receipt，不能撤销已发给 HSM/MPC 的调用；
   PKCS#11 报告不是硬件证明；真实厂商 HSM、coordinator HA 与 share/ledger 防回滚仍需实机验收。
3. **[设计层已完成，外部 benchmark 待补] 大规模链上数据平台**：已补 canonical backfill +
   realtime merge、trace/state diff、schema/versioned decoder、provider 差异、ClickHouse
   MergeTree/ReplacingMergeTree、reorg version 与 Iceberg 冷热分层；真实容量、成本、恢复时间
   和深 reorg 重算仍需目标数据集 benchmark。
4. **[已完成第一轮] 机构金融域**：已补 custody/omnibus、clearing/settlement、DvP/PvP、
   RWA lifecycle 与 ISO 20022 消息/结算边界；具体银行/托管 rail 和司法辖区规则仍需岗位定向。

### P2：作为资深度加分项

1. **[Staff 案例结构已补]** spec-first、ADR、API/versioning、跨团队技术治理、mentor、
   on-call 与事故指挥；候选人仍需用本人真实项目替换 `S-LEAD-04/05` 占位符。
2. 对 Crypto Agent 生态岗位，`S-AI-11~14` 已进入角色 P0；对纯 Go/Web3 基础设施岗位，
   它们仍按 JD 选择，不应挤占多链、节点和资金安全的准备时间。
3. Go 1.26 安全与工具链：`crypto/hpke`、FIPS 相关能力、现代化 `go fix`；
   按目标岗位再扩展，不列为 Web3 后端通用 P0。

## 后续推荐顺序

1. 在已完成的四链 N/N-1 harness 上补齐 Solana/Aptos/Sui 的真实节点实跑，并为四链增加
   funded transaction、simulation、VM/program failure、finality、expiry、资源冲突和
   provider differential corpus；外网 smoke 保持 opt-in。
2. 在已完成的 etcd fence、HSM acceptance 与跨进程 FROST 上接入至少一种厂商 HSM，
   增加 HSM HA/failover/rolling-upgrade 证据，并继续做 coordinator HA、participant 侧
   fence/policy authority、share HSM/KMS sealing、不可回滚恢复和 reshare/key recovery。
3. 为 canonical/trace 管线增加真实 execution client/provider differential corpus，以及
   ClickHouse/lakehouse 的 reorg、重算、容量和成本 benchmark。
4. 按目标岗位选择一个机构 rail 深挖 ISO 20022 usage guideline、statement/reconciliation、
   return/investigation 与 RWA transfer-agent/custodian integration。
5. 继续交易系统 account/risk shard、cross-margin、故障注入，以及带 WAL/网络/发布的端到端性能实验。

## 校验要求

本轮为 **知识库与元数据修改**，交付前校验：

- `.venv/bin/python scripts/verify_knowledge_metadata.py`
- 三个文档生成脚本可重复执行
- `.venv/bin/mkdocs build --strict`
- `git diff --check`

本轮不调用 Docker、不启动本地数据库/链节点、不运行项目集成 harness，也不把历史 harness
标签解释成“本轮重新验收通过”。

新增题目继续遵循 [template.md](template.md) 的 `30s / 3min / 10min`
结构；涉及资金、共识、签名和版本语义时，必须同时写出适用边界、失败状态和恢复路径。
