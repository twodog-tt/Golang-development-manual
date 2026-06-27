---
id: S-SOL-08
title: 45 分钟架构演进白板模板
module: solution-architecture
level: architect
frequency: 5
go_version: "1.22+"
tags: [whiteboard, system-design, evolution, capacity, architect-interview]
status: published
code_refs: []
sources:
  - https://www.hellointerview.com/learn/system-design/in-a-hurry/introduction
  - https://sre.google/sre-book/service-level-objectives/
---

# 45 分钟架构演进白板模板

## 30 秒版（开场）

> 架构师终面常给 **开放式业务题**（设计 IM / 外卖 / 企业知识平台），45 分钟考察 **澄清 → 估算 → MVP → 扩展 → 非功能 → 演进**。本模板整合 [03 系统设计](../03-system-design/) 单题能力，形成 **完整叙事**；不是新知识点，而是 **面试交付结构**。

## 3 分钟版（一面深度）

1. **是什么**：可复用的白板时间盒与话术，展示 Staff/架构师级结构化思维。
2. **为什么**：高级候选人差在「散点懂很多，45min 讲不完整」；本模板是 **架构师岗必练**。
3. **怎么做**：按下面 7 段推进，每段留 3～8 分钟，留 5 分钟 Q&A。

## 10 分钟版：45 分钟时间盒

```mermaid
flowchart LR
  A[0-5 澄清] --> B[5-10 估算]
  B --> C[10-20 MVP]
  C --> D[20-28 扩展]
  D --> E[28-35 非功能]
  E --> F[35-40 演进路线图]
  F --> G[40-45 Q&A]
```

### 第 1 段：澄清需求（0～5 min）

必问清单：

| 维度 | 示例问题 |
|------|----------|
| 规模 | DAU、QPS 峰值、数据量级 |
| 延迟 | P99 目标、强一致 vs 最终一致 |
| 功能 | 核心路径 vs 二期 |
| 约束 | 预算、团队、已有系统 |

**话术**：「我先确认范围，避免过度设计；如果 100 万 DAU 和 1 亿 DAU 方案会不同。」

### 第 2 段：容量估算（5～10 min）

背公式（与 [S-ARCH-18 容量](../03-system-design/S-ARCH-18-capacity-planning.md) 一致）：

- QPS = DAU × 人均请求 / 86400 × 峰值系数（3～10）
- 存储 = 日增 × 保留天数 × 副本系数
- 带宽 = QPS × 平均响应体

写出 **数量级** 即可，精确到个位无意义。

### 第 3 段：MVP 架构（10～20 min）

白板上至少包含：

```
[Client] → [LB/GW] → [Go API] → [Cache?] → [DB]
                      ↓
                    [MQ?]
```

- 标 **读写路径** 与 **数据所有权**
- Go 选型理由一句：goroutine 高并发、部署简单
- 引用具体模式：秒杀 → [S-ARCH-02](../03-system-design/S-ARCH-02-seckill.md)；幂等 → [S-ARCH-04](../03-system-design/S-ARCH-04-idempotency.md)

### 第 4 段：扩展与瓶颈（20～28 min）

按 **10x 压力** 讲改动：

| 瓶颈 | 手段 |
|------|------|
| 读 | 缓存、CDN、读副本 |
| 写 | 分库分表、异步化 |
| 热点 | 本地缓存、请求合并 |
| 单点 | 多 AZ、MQ 集群 |

画 **Before / After** 小图，不要堆组件名词。

### 第 5 段：非功能（28～35 min）

架构师差异化段落：

| 主题 | 要点 |
|------|------|
| 可用性 | 多副本、熔断 [S-ARCH-09](../03-system-design/S-ARCH-09-circuit-breaker.md) |
| 可观测 | 指标/日志/链路 [S-ARCH-16](../03-system-design/S-ARCH-16-observability.md) |
| SLO | 错误预算 [S-ARCH-17](../03-system-design/S-ARCH-17-slo-error-budget.md) |
| 安全 | [S-SOL-07](./S-SOL-07-security-audit-architecture.md) 一句 |
| 多租户 | 若 B 端则 [S-SOL-05](./S-SOL-05-multi-tenant-saas.md) |

### 第 6 段：演进路线图（35～40 min）

三阶段叙事（面试官最爱）：

| 阶段 | 目标 | 架构变化 |
|------|------|----------|
| MVP | 3 个月上线 | 单体 Go + MySQL + Redis |
| 10x | 大促 | 读写分离、MQ 削峰 |
| 100x | 多区域 | 分片、多活 [S-ARCH-13](../03-system-design/S-ARCH-13-multi-active-dr.md)、域拆分 [S-SOL-01](./S-SOL-01-bounded-context-ddd.md) |

提及 **绞杀者** 若从遗留迁移：[S-SOL-02](./S-SOL-02-strangler-fig-migration.md)。

### 第 7 段：Q&A（40～45 min）

准备 3 个 **主动风险**：

- 「最大风险是 DB 写瓶颈，缓解是异步下单 + 对账」
- 「回滚策略是特性开关 + 网关切流」
- 「团队能力不足则 Phase 1 不上 Mesh」

## 生产场景（练习建议）

用本仓库题目 **组卷模拟**：

| 模拟题 | 关联题单 |
|--------|----------|
| 企业知识库 + AI 问答 | S-AI-02 + S-SOL-05 + S-ARCH-16 |
| 订单中台从单体演进 | S-SOL-01/02/03 + S-ARCH-12 |
| 多租户 SaaS 报表 | S-SOL-05 + S-ES-02 + S-SOL-03 |

**录音 45 分钟自述**，回听是否跳步、是否缺数字。

## 架构取舍

| 过早 100x 设计 | 只讲 MVP |
|----------------|----------|
| 面试官觉虚 | 觉无深度 |

**平衡**：MVP 具体 + 演进 credible。

## 追问链

1. **估算离谱怎么办？** → 说明假设，给 sensitivity（DAU ±2x 时 DB 变化）。
2. **Go 为何不用 Java？** → 团队、GC 延迟、容器密度 — 不贬损，讲 tradeoff。
3. **如何体现架构师而非高级开发？** → 多讲 **域、演进、治理、组织**（S-SOL-01/06）。
4. **白板画乱？** → 从左到右数据流，组件 ≤7 个/图，分图。

## 反模式与事故

- **跳过澄清直接画 Kafka** → 被追问惨
- **无数字** → 不可信
- **不会说「这块 Phase 2」** → 时间不够答不完

## 代码示例

非代码题；配合 [learning-path-senior.md](../../learning-path-senior.md) 架构师 4 周计划演练。

## 延伸阅读

- 本手册 [03 系统设计](../03-system-design/) 20 题作为组件知识库
- [Google SRE - SLO](https://sre.google/sre-book/service-level-objectives/)
