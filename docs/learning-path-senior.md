# 5 年+ Go 后端面试学习路线

> 目标读者：**Go 后端** / **Tech Lead** / **区块链+后端架构师**  
> 假设：已有 3 年以上生产经验，需补齐 **Go 深度 + 系统设计 + Solidity/链上工程**

## 能力自检（开始前）

- [ ] 能白板画出 GMP 与 goroutine 生命周期
- [ ] 能读 pprof heap/cpu 并定位泄漏或热点
- [ ] 能在 15 分钟内设计带数字估算的高并发读接口
- [ ] 能讲 2 个真实项目：性能优化、一致性/事故各 1 个
- [ ] 能说明「为什么不用某中间件/某并发模型」

### 架构师岗额外自检

- [ ] 能画 **限界上下文图** 并说明集成关系（[S-SOL-01](./interview/11-solution-architecture/S-SOL-01-bounded-context-ddd.md)）
- [ ] 能讲 **遗留迁移/绞杀者** 阶段与回滚（[S-SOL-02](./interview/11-solution-architecture/S-SOL-02-strangler-fig-migration.md)）
- [ ] 能在 **45 分钟**内完成开放式白板（[S-SOL-08](./interview/11-solution-architecture/S-SOL-08-evolution-whiteboard.md)）
- [ ] 能主持或参与 **架构评审** 并输出 ADR（[S-SOL-06](./interview/11-solution-architecture/S-SOL-06-architecture-review.md)）

---

## 冲刺版（4 周，在职）

| 周 | 模块 | 阅读 | 练习 | 自测 |
|----|------|------|------|------|
| W1 | [并发与运行时](./interview/01-runtime-concurrency/) | 20 题 | `basis/` + `examples/senior/` | 画 GMP；讲 2 个泄漏案例 |
| W2 | [内存与 GC](./interview/02-memory-gc/) | 15 题 | `go test -race`、`-gcflags=-m` | 读一份 heap profile |
| W3 | [系统设计](./interview/03-system-design/) | 20 题 | 每题 15min 结构化输出 | 秒杀/幂等/缓存各 1 题口述 |
| W4 | P1 模块 + 模拟 | 分布式/DB/网络/**AI** 各选 5 题 | 2 场模拟面 | 追问链不停顿 3 层 |

### 每日建议（工作日 1.5h）

- 40min：精读 2 篇 P0 文档（含追问链）
- 30min：跑/改 1 段关联代码
- 20min：口述「30 秒版 + 1 个生产例子」

---

## 系统版（8 周）

| 周 | 内容 |
|----|------|
| 1-2 | P0：并发 + 内存（同冲刺 W1-W2，加深 pprof/trace） |
| 3-4 | P0：系统设计 20 题 + 容量估算模板 |
| 5 | [分布式与中间件](./interview/04-distributed-middleware/) |
| 6 | [数据库与存储](./interview/05-database-storage/) + `gorm/` |
| 7 | [网络与服务治理](./interview/06-network-governance/) + `gin-example/` |
| 8 | [AI 工程](./interview/10-ai-engineering/) + [工程与领导力](./interview/07-engineering-leadership/) + [云原生](./interview/09-cloud-native/) + [手写题](./interview/08-coding-senior/) |

---

## 架构师岗冲刺（6 周，在职）

> 在 **P0 系统设计 20 题** 基础上，专攻 [11 解决方案架构](./interview/11-solution-architecture/) 8 题 + 45min 白板。

| 周 | 模块 | 阅读 | 练习 | 自测 |
|----|------|------|------|------|
| W1 | P0 复习 | 03 系统设计 10 题 + 01/02 各 5 题 | 容量估算 3 题 | 15min 秒杀/幂等口述 |
| W2 | [解决方案架构](./interview/11-solution-architecture/) | S-SOL-01～04 | 画上下文图 + 迁移阶段图 | 讲 1 个真实拆分/迁移故事 |
| W3 | 解决方案架构 + 中间件 | S-SOL-05～08 + middleware | 多租户 + Outbox 方案口述 | 45min 白板模拟 ×1 |
| W4 | 领导力 + 云原生 | 07 + 09 全读 | ADR 写 1 篇 | 架构评审角色扮演 |
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

---

## 模块优先级

```
P0（必过）: 01 并发 → 02 内存 → 03 系统设计
P1（大厂二面）: middleware + 06 网络 + 10 AI + 12 Web3 + 14 交易所（JD 相关时）
P2（Lead 面）: 07 工程领导力 → 08 手写题 → 09 云原生
架构师岗（P2+）: 11 解决方案架构 + 03 系统设计 + 07 领导力
Web3 架构师: 12 区块链 Go（9 题）+ 13 Solidity（8 题）+ S-SOLID-08 边界题
交易所工程师: 14 DEX/CEX（9 题）+ 03 幂等/限流 + 12 索引/充提 + 13 DeFi 合约
```

---

## Web3 架构师冲刺（6 周）

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | [12 Web3 Go](./interview/12-blockchain-web3/) S-BC-01～04 | 画 EVM 账户；跑 `ethrpc` 测试 |
| W2 | 12 Web3 S-BC-05～09 | 跑 `erc20bind`；讲索引 reorg |
| W3 | [13 Solidity](./interview/13-solidity-contracts/) S-SOLID-01～04 | Review 重入/Proxy 题 |
| W4 | 13 Solidity S-SOLID-05～08 | Foundry 清单；链上链下分工口述 |
| W5 | 11 解决方案架构 + 07 领导力 | 45min 白板（DeFi/多链场景） |
| W6 | 模拟 | Go + Solidity 联合追问 |

---

## 交易所工程师冲刺（4 周）

| 周 | 模块 | 自测 |
|----|------|------|
| W1 | [14 DEX/CEX](./interview/14-dex-cex-engineering/) S-EXCH-01～03 | 画撮合→账务链路；讲复式记账 |
| W2 | 14 S-EXCH-04～05 + [03 幂等](./interview/03-system-design/S-ARCH-04-idempotency.md) | 强平流程；对账维度 |
| W3 | 14 S-EXCH-06～08 + [13 DeFi](./interview/13-solidity-contracts/S-SOLID-07-defi-patterns.md) | AMM 公式；MEV 防护 |
| W4 | 14 S-EXCH-09 + [12 索引](./interview/12-blockchain-web3/S-BC-05-indexer-reorg.md) | CeDeFi 白板；充提状态机 |

题单索引：[interview/_meta/questions.yaml](./interview/_meta/questions.yaml)
