# 5 年+ Go / Agent / Web3 工程学习路线

> 目标读者：有一定生产经验、希望系统补齐 **Go 生产工程 + 系统设计 + Web3 链上基建 / DEX / Agent 控制面** 的工程师。  
> 用法：先定主线 **Go · Web3 链上基建 · AI Agent** 之一，再选下方学习计划；完整查阅见 [首页](./index.md) 与 [专题总索引](./topic-catalog.md)。

## 领域核心：共享门槛 + 一个领域增量

不再使用全库统一核心清单。先完成 **shared Go/生产工程门槛**，同时只选择 **一个** 工程领域增量：

| 轨道 | 核心篇数 | 增量重点 |
|------|--------:|----------|
| Go 生产工程 | 62 | PostgreSQL/MySQL、消息、网络、IaC/GitOps |
| AI / Crypto Agent 控制面 | 64 | 工作流/HITL、MCP/A2A、身份/Commerce、开放平台 |
| 多链钱包与托管签名 | 66 | 多链交易、归集、MPC/HSM、恢复 |
| 支付与稳定币 | 66 | 支付状态机、账本、清结算、合规 |
| 节点/RPC/Indexer | 73 | 共识、canonical 数据、ClickHouse/lakehouse |
| 交易所工程 | **75** | 撮合/WAL、DEX 协议、预测市场、账本与风控 |
| 系统演进与架构协作 | **80** | 技术战略、跨团队迁移、发布与风险治理 |

完整清单、延展阅读与证据标签见
[领域能力优先级](./topics/_meta/role-priority-matrix.md)；依赖关系见
[领域知识图谱](./topics/_meta/p0-knowledge-graph.md)。

## 能力自检（开始前）

- [ ] 能白板画出 GMP 与 goroutine 生命周期
- [ ] 能读 pprof heap/cpu 并定位泄漏或热点
- [ ] 能在 15 分钟内设计带数字估算的高并发读接口
- [ ] 能讲 2 个真实项目：性能优化、一致性/事故各 1 个
- [ ] 能说明「为什么不用某中间件/某并发模型」
- [ ] 能说明 error 契约、panic/recover 边界、consumer-owned interface
- [ ] 能设计 table test、fake/integration test、race/fuzz/benchmark 门禁
- [ ] 能排查 FD、epoll、TCP 队列、TIME_WAIT 与连接池问题
- [ ] 能写窗口函数/CTE，并解释金额类型、账本约束和死锁重试
- [ ] 能区分 PostgreSQL tuple/VACUUM/SSI 与 MySQL undo/MVCC 话术
- [ ] 能说明 Terraform state/plan 边界与 Helm/GitOps 数据回滚边界

### 架构协作额外自检

- [ ] 能画 **限界上下文图** 并说明集成关系（[S-SOL-01](./topics/11-solution-architecture/S-SOL-01-bounded-context-ddd.md)）
- [ ] 能讲 **遗留迁移/绞杀者** 阶段与回滚（[S-SOL-02](./topics/11-solution-architecture/S-SOL-02-strangler-fig-migration.md)）
- [ ] 能在 **45 分钟**内完成开放式白板（[S-SOL-08](./topics/11-solution-architecture/S-SOL-08-evolution-whiteboard.md)）
- [ ] 能主持或参与 **架构评审** 并输出 ADR（[S-SOL-06](./topics/11-solution-architecture/S-SOL-06-architecture-review.md)）
- [ ] 能对比 EVM、UTXO、Solana、Cosmos、Sui/Aptos 的冲突域和 finality
- [ ] 能说明 Solana/Cosmos/Aptos Go SDK 的签名 bytes 与提交/执行边界，以及 Sui SDK/capability 版本边界
- [ ] 能画支付 intent → 链确认 → 账本 → 清结算 → 对账状态机
- [ ] 能区分 ISO 20022 消息、清算义务、资金腿与法律 finality，并画 RWA 申赎/DvP
- [ ] 能说明 EL/CL、RPC quorum/hedging、validator slashing 与节点升级
- [ ] 能证明 backfill/realtime 的 canonical hash overlap，并解释 trace/client/decoder 数据质量
- [ ] 能设计 ClickHouse 排序键、reorg version 与 lakehouse 重放分层
- [ ] 能区分 LMD-GHOST/FFG 与 CometBFT round/lock，并解释 PeerDAS 的当前状态
- [ ] 能区分 Rollup unsafe/safe/finalized 与 withdrawal ready，并设计跨链 message replay/exposure guard
- [ ] 能解释 signer-side fencing、SBOM 与 provenance 的差别，并跑安全故障注入
- [ ] 能运行撮合/WAL、行情恢复、FIX 与集合竞价，解释 sequence、STP 和性能边界
- [ ] 能用真实指标讲一个 Staff 技术战略或跨团队迁移案例，不套用虚构数据
- [ ] （DEX Tech Lead）能讲 Uniswap V2/V3 差异、rewardPerToken 激励会计，并完成 45min DEX 白板（[S-EXCH-31](./topics/14-dex-cex-engineering/S-EXCH-31-dex-tech-lead-whiteboard.md)）

---

## 4 周学习计划（在职）

| 周 | 模块 | 阅读 | 练习 | 自测 |
|----|------|------|------|------|
| W1 | Go 语言 + 生产工程 | 01 核心 10 篇 + [16 全部](./topics/16-go-production-engineering/index.md) | `go test -race`、错误/接口重构 | 画 GMP；讲 error 与 panic 边界 |
| W2 | 内存 + 手写 + Linux/SQL | 02 高频 + S-CODE-06/07 + S-NET-06/07 + S-DB-06/07 + S-PG-01~03 | 跑 5 个新示例、读 heap profile | 手写有界批处理；排查一次 TCP/SQL |
| W3 | [系统设计](./topics/03-system-design/index.md) | 21 篇 | 每篇 15min 结构化输出 | 秒杀/幂等/缓存/实时数据各 1 篇讲解 |
| W4 | 目标领域主线 + 综合演练 | 按领域矩阵只选一个增量核心 | 证据标签为 test/harness 的篇才运行；2 场综合演练 | 深挖问答不停顿 3 层 |

### 每日建议（工作日 1.5h）

- 40min：精读 2 篇 P0 文档（含深挖问答）
- 30min：跑/改 1 段关联代码
- 20min：讲解「30 秒版 + 1 个生产例子」

---

## 系统版（8 周）

| 周 | 内容 |
|----|------|
| 1 | 01 并发 + 02 内存高频 |
| 2 | [16 Go 生产工程](./topics/16-go-production-engineering/index.md) + [08 编码练习](./topics/08-coding-senior/index.md) |
| 3-4 | 03 系统设计 21 篇 + 容量估算模板 |
| 5 | [网络](./topics/06-network-governance/index.md) + [MySQL](./topics/middleware/mysql/index.md) + [PostgreSQL](./topics/middleware/postgresql/index.md) |
| 6 | Redis/Kafka/RocketMQ/ES + Terraform/Helm/GitOps |
| 7 | 目标领域专题：通用后端选 11/15；Web3 选 17/18 与四条 SDK 实战；DEX 协议选 14（31/30/29）；Agent 选 10 |
| 8 | Web3 节点/RPC + 安全工程 + 协议/共识 + Rollup/跨链，或 AI/领导力 + 2 场综合演练 |

---

## 架构师岗学习计划（6 周，在职）

> 在 **P0 系统设计 21 篇** 基础上，专攻 [11 解决方案架构](./topics/11-solution-architecture/index.md) 8 篇 + 45min 白板。

| 周 | 模块 | 阅读 | 练习 | 自测 |
|----|------|------|------|------|
| W1 | P0 复习 | 03 系统设计 10 篇 + 01/02 各 5 篇 | 容量估算 3 篇 | 15min 秒杀/幂等讲解 |
| W2 | [解决方案架构](./topics/11-solution-architecture/index.md) | S-SOL-01～04 | 画上下文图 + 迁移阶段图 | 讲 1 个真实拆分/迁移故事 |
| W3 | 解决方案架构 + 中间件 | S-SOL-05～08 + middleware | 多租户 + Outbox 方案讲解 | 45min 白板演练 ×1 |
| W4 | 领导力 + 云原生 | S-LEAD-01~05 + S-CLOUD-04/07/09/10 | ADR + 迁移门禁各 1 篇 | 架构评审角色扮演 |
| W5 | AI + 网络（可选） | 10 + 06 各 4 篇 | MCP/RAG 架构串联 | 企业知识库综合演练 |
| W6 | 综合演练 | 03 + 11 抽专题 | 45min 白板 ×2 + 深挖 | 录像复盘 |

**架构师综合演练组合示例**（见 [S-SOL-08](./topics/11-solution-architecture/S-SOL-08-evolution-whiteboard.md)）：

- 多租户 SaaS 订单 + 报表：S-SOL-05 + S-SOL-03 + S-ARCH-12
- 遗留单体迁 Go 微服务：S-SOL-02 + S-SOL-01 + S-ARCH-19
- 企业 AI 知识平台：S-SOL-05 + S-AI-02 + S-SOL-07

---

## 系统设计讲解模板（15 分钟）

```
1. 澄清需求：QPS、读写比、一致性、延迟、可用性
2. 估算：流量、存储、带宽、缓存命中率
3. 架构图：接入层 → 服务层 → 缓存/DB/MQ
4. 核心路径：Happy path + 失败降级
5. 瓶颈与扩展：热点、单点、数据分片
6. 可观测：指标、告警、链路
7. 演进：MVP → 10x → 100x 怎么改
```

---

## 项目故事准备（至少 3 个）

| 类型 | 建议结构 |
|------|----------|
| 性能 | 现象 →  profiling → 改动 → 指标（P99 -X%，CPU -Y%） |
| 一致性/事故 | 触发 → 根因 → 修复 → 预防（规范/演练） |
| 技术决策 | 备选方案 → 权衡矩阵 → 结果与复盘 |
| Staff 影响 | 诊断 → 决策权/异议 → 薄切片 → adoption → 真实指标/未达成 |

---

## 模块优先级（菜单由易到难）

```
基础：01 并发 → 02 内存 → 16 Go 生产工程 → 08 编码练习
进阶：06 网络 → middleware（MySQL↔PostgreSQL→Redis→MQ→ES）→ 03 系统设计 → 09 云原生
高阶：11 解决方案架构 → 15 微服务（交易所）
专题：12 EVM → 17 多链钱包 → 18 支付 → 19 节点/RPC → 20 协议/共识 → 21 安全工程 → 13 Solidity → 14 DEX/CEX
综合：07 工程领导力 · 10 AI 工程（场景相关时提前）
```

**领域速查**

- **大厂 Go 后端**：01/02/16/08 + 06 + MySQL/PostgreSQL + S-CLOUD-09/10
- **架构师岗**：03/11 + S-CLOUD-09/10 + S-LEAD-04/05 + 45min 白板
- **Web3 钱包/支付**：16 → 17 → 18 → 21 + 12 索引/签名
- **节点/RPC/Indexer**：06 → 19（含 NODE-10）→ 20 → 21 + 12 索引 + 09 云原生
- **Web3 架构师**：17 → 18 → 19 → 20 → 21 + 03/11 + 12/13
- **CEX 交易系统**：14（重点 EXCH-17~22）+ 15 + 17 充提 + 18 账本/对账
- **DEX Tech Lead**：14（**EXCH-31 → 30 → 29** → 06/27 → 07/08）+ 13（SOLID-02/04/06/07）+ 12（BC-05/07/08）+ LEAD-01/03/04
- **AI / Crypto Agent**：10（AI-09~14）+ 16 + 21；按工程场景再补 12/14

---

## Web3 架构师学习计划（6 周）

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | [16 Go 生产工程](./topics/16-go-production-engineering/index.md) + [21 安全工程](./topics/21-security-engineering/index.md) + S-NET-06/07 + S-PG-01~03 | 跑 `singleflightcache`、`signerfencing` 与 `signer-project`；解释数据库/HSM evidence 边界 |
| W2 | [17 多链钱包](./topics/17-multichain-wallet/index.md) | 画五类链能力矩阵；跑四链离线门禁与 Cosmos localnet fault/upgrade gate；区分 harness 与真实节点证据 |
| W3 | [18 支付与稳定币](./topics/18-web3-payments-stablecoin/index.md) | 画支付/账本/DvP/RWA/ISO 20022；跑 `paymentstate` |
| W4 | [19 节点、RPC 与 Staking](./topics/19-node-rpc-staking/index.md) + [20 协议/共识](./topics/20-protocol-consensus-security/index.md) | 画 EL/CL、canonical merge、ClickHouse/lakehouse、LMD-GHOST/FFG；跑 `rpcpool`、`chainmerge`、`txlifecycle` |
| W5 | [12 Web3 Go](./topics/12-blockchain-web3/index.md) + [13 Solidity](./topics/13-solidity-contracts/index.md) | 讲 PeerDAS、Rollup finality、跨链消息认证与合约边界；跑 `bridgeguard` |
| W6 | 03/11/14/15 + S-LEAD-04/05 | 45min 白板 ×2 + 一个带真实指标的 Staff 案例 |

---

## 交易所工程师学习计划（4 周）

分两条轨，按工程场景二选一（可并行 Shared Go）。

### A. CEX 交易系统

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | [14 DEX/CEX](./topics/14-dex-cex-engineering/index.md) S-EXCH-01/17~22 | 跑撮合、WAL、行情、FIX、竞价；解释 sequence、STP 与 benchmark 边界 |
| W2 | [15 微服务](./topics/15-microservices-exchange/index.md) + [18 账本/清结算](./topics/18-web3-payments-stablecoin/index.md) | 交易事实、账本事实、结算边界 |
| W3 | [17 多链钱包](./topics/17-multichain-wallet/index.md) + [19 Relayer/RPC](./topics/19-node-rpc-staking/index.md) | 充提、nonce/UTXO、广播恢复 |
| W4 | 14 永续（16/04）+ 12 Rollup/跨链 + 13 合约 | CEX/CeDeFi 45min 白板（EXCH-13/14/15） |

### B. DEX Tech Lead

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | **S-EXCH-31 → 30 → 29** + EXCH-06/27 | 45min DEX 白板；讲 V2/V3 与 rewardPerToken |
| W2 | EXCH-07/08 + SOLID-02/04/06/07 | 聚合/MEV、重入、升级、Foundry 审计清单 |
| W3 | BC-05/07/08/11/12 + EXCH-10/11 | Indexer/reorg、L2/桥、AA、行情同步 |
| W4 | EXCH-14 + LEAD-01/03/04 + 综合演练 | 全栈叙事 + CR/事故/带队；英文讲解要点 |

---

## AI / Crypto Agent 学习计划（4 周，可选）

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | [10 AI 工程](./topics/10-ai-engineering/index.md) S-AI-01~04 + 09 | API/RAG/Agent 工具调用 + 工作流/HITL |
| W2 | S-AI-10~12 | Persona/Memory、MCP/A2A、ERC-8004 身份 |
| W3 | S-AI-13~14 + 21 安全 | Agent Commerce、开放平台/Launchpad、威胁模型 |
| W4 | 16 Go 生产工程 + 综合演练 | 与 Go 工程门禁串联；1 场综合讲解 |

专题索引：[topics/_meta/topics.yaml](./topics/_meta/topics.yaml)
