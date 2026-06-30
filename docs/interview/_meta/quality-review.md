# 习题质量审查记录

> 全库 **143 题**（`questions.yaml`）· 审查日期：2026-06  
> 题单源数据：[questions.yaml](questions.yaml)

## 模块分布

| 模块 | 题数 | 入口 |
|------|------|------|
| 01 并发与运行时 | 20 | [01-runtime-concurrency](../01-runtime-concurrency/index.md) |
| 02 内存与 GC | 15 | [02-memory-gc](../02-memory-gc/index.md) |
| 03 系统设计 | 20 | [03-system-design](../03-system-design/index.md) |
| 中间件与数据库 | 23 | [middleware](../middleware/index.md) |
| 06 网络 | 5 | [06-network-governance](../06-network-governance/index.md) |
| 07 领导力 | 3 | [07-engineering-leadership](../07-engineering-leadership/index.md) |
| 08 手写题 | 5 | [08-coding-senior](../08-coding-senior/index.md) |
| 09 云原生 | 8 | [09-cloud-native](../09-cloud-native/index.md) |
| 10 AI 工程 | 8 | [10-ai-engineering](../10-ai-engineering/index.md) |
| 11 解决方案架构 | 8 | [11-solution-architecture](../11-solution-architecture/index.md) |
| 12 区块链 Web3 | 10 | [12-blockchain-web3](../12-blockchain-web3/index.md) |
| 13 Solidity | 8 | [13-solidity-contracts](../13-solidity-contracts/index.md) |
| 14 DEX/CEX | 12 | [14-dex-cex-engineering](../14-dex-cex-engineering/index.md) |

**状态**：全部 `status: published`；无 draft/skeleton。

## 图示覆盖

| 指标 | 数量 |
|------|------|
| 含 Mermaid 图 | 143 / 143 |
| 本次补图 | 6 题（见下表） |

原先无图的 5 题 + RocketMQ 延迟路径图已补齐。

## 本次修正（技术准确性）

| 题 ID | 修正要点 |
|-------|----------|
| S-CONC-01 | mark assist 发生在并发标记阶段，非 STW |
| S-CONC-04 | Go 1.25 cgroup GOMAXPROCS 下限与自动覆盖条件 |
| S-CONC-05 | select 多 channel 加锁顺序，非「单锁」 |
| S-CONC-12 | WithTimeout 未 cancel 泄漏 timer/context，非必然 goroutine 泄漏 |
| S-MEM-01 | STW 阶段（mark 开始/终止）；堆写屏障 vs 栈扫描 |
| S-MEM-03 | `SetGCPercent(-1)` 会关闭 GC，不能用于只读 |
| S-RMQ-01 | Consumer Group vs BROADCASTING 语义区分 |
| S-RMQ-02 | RocketMQ 4.x 18 档延迟 vs 5.0+ 任意时刻 timer |
| S-KAFKA-01 | `acks=all` = min.insync.replicas 确认，非 ISR 全量 |
| S-KAFKA-02 | kafka-go 无原生幂等 Producer；幂等开启后 max.in.flight≤5 可保序 |
| S-EXCH-06 | Uniswap 公式标题与 0.3% fee 表述一致 |

## 本次补图

| 题 ID | 图示内容 |
|-------|----------|
| S-SOLID-01 | storage slot 打包与 mapping 槽位 |
| S-SOLID-03 | ERC-20 approve → transferFrom 时序 |
| S-SOLID-05 | EIP-2929 冷/暖存储访问 |
| S-SOLID-07 | 闪电贷 + AMM 价格操纵路径 |
| S-ES-02 | bool 查询与聚合结构 |
| S-RMQ-02 | 延迟消息投递路径 |

## 审查结论

1. **P0 Go 核心**（并发/内存/系统设计 55 题）：结构与官方 Go 文档一致；本次修正集中在 GC 写屏障与 GOMAXPROCS 容器语义。
2. **中间件**（23 题）：Kafka/RocketMQ 已与官方文档对齐；MySQL/Redis/ES 为成熟考点，未发现硬伤。
3. **Web3/Solidity/DEX**（30 题）：链上公式与 EIP 引用可核对；复杂布局/DeFi 已补图。
4. **AI/云原生/架构**（24 题）：偏工程实践与官方最佳实践，无矛盾表述。

## 后续建议（未改动的合理简化）

以下内容为面试口述简化，**非错误**，若二面深挖可延伸阅读：

- 容量估算数量级（S-ARCH-18）需结合压测校准
- 跨链桥「乐观/原生」安全假设（S-BC-07）随协议变化
- LLM 成本模型（S-AI-06）随定价变动

## 维护方式

新增题目请遵循 [template.md](template.md)：`30s / 3min / 10min` + 追问链 + 反模式；架构/流程类 **优先加 Mermaid**。
