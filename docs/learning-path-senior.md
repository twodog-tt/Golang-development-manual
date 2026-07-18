# 5 年+ Go 后端面试学习路线

> 目标读者：**Go 后端** / **Tech Lead** / **区块链+后端架构师**  
> 假设：已有 3 年以上生产经验，需补齐 **Go 生产工程 + 系统设计 + 多链钱包/支付/节点基础设施**

## 角色化 P0：共享门槛 + 一个岗位增量

不再使用统一 P0。先并行完成 **shared Go/生产工程门槛**，同时只选择一个目标岗位增量：

| 轨道 | 有效 P0 | 增量重点 |
|------|--------:|----------|
| 资深 Go 后端 | 62 | PostgreSQL/MySQL、消息、网络、IaC/GitOps |
| 多链钱包与托管 | 65 | 多链交易、归集、MPC/HSM、恢复 |
| 支付与稳定币 | 65 | 支付状态机、账本、清结算、合规 |
| 节点/RPC/Indexer | 73 | 共识、canonical 数据、ClickHouse/lakehouse |
| 交易所工程 | 68 | 撮合/WAL、行情/FIX、账本与风控 |
| Staff/后端架构师 | 74 | 技术战略、跨团队迁移、发布与风险治理 |

完整题号、P1/P2 与证据标签见
[角色化优先级矩阵](./interview/_meta/role-priority-matrix.md)；依赖关系见
[知识图谱](./interview/_meta/p0-knowledge-graph.md)。

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

### 架构师岗额外自检

- [ ] 能画 **限界上下文图** 并说明集成关系（[S-SOL-01](./interview/11-solution-architecture/S-SOL-01-bounded-context-ddd.md)）
- [ ] 能讲 **遗留迁移/绞杀者** 阶段与回滚（[S-SOL-02](./interview/11-solution-architecture/S-SOL-02-strangler-fig-migration.md)）
- [ ] 能在 **45 分钟**内完成开放式白板（[S-SOL-08](./interview/11-solution-architecture/S-SOL-08-evolution-whiteboard.md)）
- [ ] 能主持或参与 **架构评审** 并输出 ADR（[S-SOL-06](./interview/11-solution-architecture/S-SOL-06-architecture-review.md)）
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

---

## 冲刺版（4 周，在职）

| 周 | 模块 | 阅读 | 练习 | 自测 |
|----|------|------|------|------|
| W1 | Go 语言 + 生产工程 | 01 核心 10 题 + [16 全部](./interview/16-go-production-engineering/index.md) | `go test -race`、错误/接口重构 | 画 GMP；讲 error 与 panic 边界 |
| W2 | 内存 + 手写 + Linux/SQL | 02 高频 + S-CODE-06/07 + S-NET-06/07 + S-DB-06/07 + S-PG-01~03 | 跑 5 个新示例、读 heap profile | 手写有界批处理；排查一次 TCP/SQL |
| W3 | [系统设计](./interview/03-system-design/index.md) | 20 题 | 每题 15min 结构化输出 | 秒杀/幂等/缓存各 1 题口述 |
| W4 | 目标岗位主线 + 模拟 | 按角色矩阵只选一个增量 P0 | 证据标签为 test/harness 的题才运行；2 场模拟面 | 追问链不停顿 3 层 |

### 每日建议（工作日 1.5h）

- 40min：精读 2 篇 P0 文档（含追问链）
- 30min：跑/改 1 段关联代码
- 20min：口述「30 秒版 + 1 个生产例子」

---

## 系统版（8 周）

| 周 | 内容 |
|----|------|
| 1 | 01 并发 + 02 内存高频 |
| 2 | [16 Go 生产工程](./interview/16-go-production-engineering/index.md) + [08 手写题](./interview/08-coding-senior/index.md) |
| 3-4 | 03 系统设计 20 题 + 容量估算模板 |
| 5 | [网络](./interview/06-network-governance/index.md) + [MySQL](./interview/middleware/mysql/index.md) + [PostgreSQL](./interview/middleware/postgresql/index.md) |
| 6 | Redis/Kafka/RocketMQ/ES + Terraform/Helm/GitOps |
| 7 | 目标岗位专题：普通后端选 11/15；Web3 选 17/18 与四条 SDK 实战 |
| 8 | Web3 节点/RPC + 安全工程 + 协议/共识 + Rollup/跨链，或 AI/领导力 + 2 场完整模拟 |

---

## 架构师岗冲刺（6 周，在职）

> 在 **P0 系统设计 20 题** 基础上，专攻 [11 解决方案架构](./interview/11-solution-architecture/index.md) 8 题 + 45min 白板。

| 周 | 模块 | 阅读 | 练习 | 自测 |
|----|------|------|------|------|
| W1 | P0 复习 | 03 系统设计 10 题 + 01/02 各 5 题 | 容量估算 3 题 | 15min 秒杀/幂等口述 |
| W2 | [解决方案架构](./interview/11-solution-architecture/index.md) | S-SOL-01～04 | 画上下文图 + 迁移阶段图 | 讲 1 个真实拆分/迁移故事 |
| W3 | 解决方案架构 + 中间件 | S-SOL-05～08 + middleware | 多租户 + Outbox 方案口述 | 45min 白板模拟 ×1 |
| W4 | 领导力 + 云原生 | S-LEAD-01~05 + S-CLOUD-04/07/09/10 | ADR + 迁移门禁各 1 篇 | 架构评审角色扮演 |
| W5 | AI + 网络（可选） | 10 + 06 各 4 题 | MCP/RAG 架构串联 | 企业知识库综合题 |
| W6 | 模拟 | 03 + 11 抽题 | 45min 白板 ×2 + 追问 | 录像复盘 |

**架构师模拟题组合示例**（见 [S-SOL-08](./interview/11-solution-architecture/S-SOL-08-evolution-whiteboard.md)）：

- 多租户 SaaS 订单 + 报表：S-SOL-05 + S-SOL-03 + S-ARCH-12
- 遗留单体迁 Go 微服务：S-SOL-02 + S-SOL-01 + S-ARCH-19
- 企业 AI 知识平台：S-SOL-05 + S-AI-02 + S-SOL-07

---

## 系统设计答题模板（15 分钟）

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
基础：01 并发 → 02 内存 → 16 Go 生产工程 → 08 手写题
进阶：06 网络 → middleware（MySQL↔PostgreSQL→Redis→MQ→ES）→ 03 系统设计 → 09 云原生
高阶：11 解决方案架构 → 15 微服务（交易所）
专题：12 EVM → 17 多链钱包 → 18 支付 → 19 节点/RPC → 20 协议/共识 → 21 安全工程 → 13 Solidity → 14 DEX/CEX
综合：07 工程领导力 · 10 AI 工程（JD 相关时提前）
```

**岗位速查**

- **大厂 Go 后端**：01/02/16/08 + 06 + MySQL/PostgreSQL + S-CLOUD-09/10
- **架构师岗**：03/11 + S-CLOUD-09/10 + S-LEAD-04/05 + 45min 白板
- **Web3 钱包/支付**：16 → 17 → 18 → 21 + 12 索引/签名
- **节点/RPC/Indexer**：06 → 19（含 NODE-10）→ 20 → 21 + 12 索引 + 09 云原生
- **Web3 架构师**：17 → 18 → 19 → 20 → 21 + 03/11 + 12/13
- **交易所工程师**：14（重点 17~22）+ 15 + 17 充提 + 18 账本/对账

---

## Web3 架构师冲刺（6 周）

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | [16 Go 生产工程](./interview/16-go-production-engineering/index.md) + [21 安全工程](./interview/21-security-engineering/index.md) + S-NET-06/07 + S-PG-01~03 | 跑 `singleflightcache`、`signerfencing` 与 `signer-project`；解释数据库/HSM evidence 边界 |
| W2 | [17 多链钱包](./interview/17-multichain-wallet/index.md) | 画五类链能力矩阵；跑四链离线门禁与 Cosmos localnet fault/upgrade gate；区分 harness 与真实节点证据 |
| W3 | [18 支付与稳定币](./interview/18-web3-payments-stablecoin/index.md) | 画支付/账本/DvP/RWA/ISO 20022；跑 `paymentstate` |
| W4 | [19 节点、RPC 与 Staking](./interview/19-node-rpc-staking/index.md) + [20 协议/共识](./interview/20-protocol-consensus-security/index.md) | 画 EL/CL、canonical merge、ClickHouse/lakehouse、LMD-GHOST/FFG；跑 `rpcpool`、`chainmerge`、`txlifecycle` |
| W5 | [12 Web3 Go](./interview/12-blockchain-web3/index.md) + [13 Solidity](./interview/13-solidity-contracts/index.md) | 讲 PeerDAS、Rollup finality、跨链消息认证与合约边界；跑 `bridgeguard` |
| W6 | 03/11/14/15 + S-LEAD-04/05 | 45min 白板 ×2 + 一个带真实指标的 Staff 案例 |

---

## 交易所工程师冲刺（4 周）

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | [14 DEX/CEX](./interview/14-dex-cex-engineering/index.md) S-EXCH-01/17~22 | 跑撮合、WAL、行情、FIX、竞价；解释 sequence、STP 与 benchmark 边界 |
| W2 | [15 微服务](./interview/15-microservices-exchange/index.md) + [18 账本/清结算](./interview/18-web3-payments-stablecoin/index.md) | 交易事实、账本事实、结算边界 |
| W3 | [17 多链钱包](./interview/17-multichain-wallet/index.md) + [19 Relayer/RPC](./interview/19-node-rpc-staking/index.md) | 充提、nonce/UTXO、广播恢复 |
| W4 | 14 DEX/永续 + 12 Rollup/跨链 + 13 合约 | CEX/CeDeFi 45min 白板 |

题单索引：[interview/_meta/questions.yaml](./interview/_meta/questions.yaml)
