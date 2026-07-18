---
id: S-LEAD-05
title: 跨团队高风险迁移、灰度切换与回滚边界
module: engineering-leadership
level: staff
frequency: 5
go_version: "1.24+"
tags: [migration, staff-engineer, shadow-read, cdc, backfill, canary, rollback, compatibility]
status: published
resume_focus: true
code_refs: []
sources:
  - https://abseil.io/resources/swe-book/html/ch10.html
  - https://abseil.io/resources/swe-book/html/ch22.html
  - https://google.github.io/eng-practices/review/developer/small-cls.html
  - https://sre.google/workbook/canarying-releases/
  - https://sre.google/sre-book/release-engineering/
  - https://sre.google/sre-book/postmortem-culture/
---

# 跨团队高风险迁移、灰度切换与回滚边界

## 30 秒版（开场）

> 高风险迁移先定义 source of truth、不变量和回滚边界，再谈双写或切流。我通常采用 expand → 可重放复制/回填 → shadow compare → 按 cohort 切读 → 切写/冻结旧入口 → contract/decommission。双写如果没有同一事务或 outbox/CDC，只会制造两个真相；应用版本回滚也不能逆转已写入的新 schema、事件或链上副作用。Staff 面要讲清技术门禁、跨团队 owner、风险登记、停止条件和可量化结果，而不是“大家配合完成迁移”。

## 3 分钟版（一面深度）

迁移前先写五项：

1. **Authority**：哪个系统在每个阶段是写入与读取事实源。
2. **Invariants**：例如账本按币种守恒、同一 observation 只映射一个 canonical version、
   同一业务幂等键最多产生一次不可逆副作用。
3. **Compatibility**：N/N-1 代码、schema、event 和 API 的读写矩阵。
4. **Gates**：完整性、正确性、新鲜度、性能、可运维性和安全门禁。
5. **Rollback**：能回退流量、代码、schema 还是数据；超过哪个点只能 forward-fix。

## 10 分钟版（迁移状态机）

```mermaid
flowchart LR
  A["discover + invariants"] --> B["expand compatibility"]
  B --> C["replicate / backfill"]
  C --> D["shadow read + reconcile"]
  D --> E["canary cohorts"]
  E --> F["cut read authority"]
  F --> G["cut write authority"]
  G --> H["stabilize + rollback window"]
  H --> I["contract + decommission"]
```

每一阶段有显式入口/出口条件，失败就停在当前阶段，不靠“先全部切完再观察”。

### 1. Discover：不要迁移未知系统

- 列出所有 writer、reader、批处理、报表、应急脚本、数据修复和隐含 session/缓存状态。
- 画数据 lineage：输入、派生、复制、消费者、保留和删除要求。
- 用流量与查询日志验证依赖，不只访谈 owner。
- 为每条不变量指定验证查询、owner 和容忍阈值。
- 确认监管、密钥、PII、数据驻留及审计要求。

迁移计划中最危险的通常不是“主服务”，而是无人维护的 cron、人工 SQL 和只在月末运行的流程。

### 2. Expand：先兼容，再切流

- 新增 nullable/default 字段或新 event version，旧代码仍能读写。
- 消费者先支持新旧格式，生产者后切新格式。
- API 用 additive change；删除/改义另开 contract 阶段。
- 数据库索引用并发构建/受控窗口，并验证 valid/usable。
- 升级序列覆盖滚动期间 N 与 N-1 同时运行。

若旧版本无法理解新写入，就不存在真正的代码回滚；这必须在发布前暴露。

### 3. Replicate/Backfill：只保留一个写入 authority

优先模式：

```text
source transaction
  -> domain rows + outbox/cursor
  -> idempotent relay/CDC
  -> target upsert/append by stable source identity
  -> checkpoint + reconciliation
```

不受控的应用双写有两个独立网络结果：A 成功/B 失败、A timeout/B 成功等，会产生两个事实源。
若确需双写，应明确哪个成功决定业务 ACK、失败如何持久化补偿，以及目标如何按 source identity
幂等；“两个都写，失败就重试”不是协议。

Backfill 要：

- 固定 snapshot/watermark，记录范围与 decoder/schema version；
- 小批、可暂停、可重试，checkpoint 单调推进；
- 与 realtime 在稳定 identity/连续 hash 上交接；
- 限速保护线上流量，并测剩余时间；
- 对历史坏数据有 quarantine，而不是为过门禁静默丢弃。

### 4. Shadow compare：影子不是第二个副作用入口

Shadow read 将同一请求送给新旧读路径，但只让当前 authority 响应用户。比较应先规范化：

- 排序、时间精度、空值、分页和最终性水位；
- 金额/计数守恒、集合差异和字段级 diff；
- 允许的 eventual lag 窗口与超时；
- mismatch 分类：真实数据错、decoder 版本差、查询语义差或观测噪声。

对支付、签名、下单和广播不能简单 shadow 执行真实写副作用。可使用 dry-run、模拟、录制输入或
在隔离账户执行。

### 5. Canary cohort：按故障域切，不按随机百分比迷信

合理 cohort 可以是：

- 只读内部流量 → 低风险租户 → 小链/小市场 → 普通租户 → 高价值业务；
- 单 shard/region → 多 shard → 全量；
- finalized 历史查询 → safe/head 近实时查询。

每批有最小观察窗口、自动/人工停止条件和回退开关。随机 1% 对共享数据库 migration、全局 CRD
或单一撮合状态机可能没有隔离意义。

### 6. Cutover 与 rollback

| 层 | 常见可回退项 | 不一定可回退项 |
|----|--------------|----------------|
| 流量 | route/feature flag/read authority | 已对外发出的 Webhook/交易 |
| 应用 | 兼容窗口内回退镜像 | 旧代码看不懂的新数据 |
| 数据库 | 保留旧列/双读 | destructive DDL、错误 backfill |
| 事件 | consumer 切回旧 topic/version | 已被外部消费者处理的事件 |
| 链上 | 停止后续广播 | 已最终化交易/合约调用 |
| IaC/CRD | Git desired state/部分资源 | provider 外部副作用、CRD 数据与存储迁移 |

Rollback plan 必须写明“回退后由谁继续同步数据”。只切回旧读路径，却停止 target→source
兼容同步，会在第二次切换时重新出现数据洞。

## 可复用案例：链数据从单体 PostgreSQL 迁到 raw lake + ClickHouse

以下仍是模板，必须替换成真实项目证据。

### Situation

> 单体 PostgreSQL 同时承担 canonical cursor、raw event、API 点查和全表分析。随着 `[链数/
> 日增量]` 增长，backfill 与在线查询互相影响；reorg 修复按高度覆盖，无法保留 fork 证据。
> 涉及 indexer、数据平台、钱包、风控和 BI `[N]` 个团队。

### Task

> 我负责在不中断 `[关键 SLO]` 的前提下，把 raw evidence、canonical control plane 和分析
> serving 分层；迁移 `[N]` 个消费者，并保留 `[时间]` 的可回退窗口。

### Action

1. 定义 observation identity、canonical version、finalized watermark 和金额/计数守恒门禁。
2. 比较“垂直扩 PostgreSQL”“直接全量换 ClickHouse”“raw lake + 小型 control plane +
   ClickHouse serving”，并记录成本与不可逆点。
3. 先让旧系统写 outbox/CDC，以 block hash 身份幂等落 raw lake 与 ClickHouse；禁止 target
   反向成为第二个写 authority。
4. 按高度区间 backfill，与 realtime 用连续 parent hash overlap 交接。
5. 对查询做 shadow compare；分别统计 missing、extra、wrong-canonical、decoder mismatch。
6. 先切 finalized 历史读，再切近实时 API；canonical commit 仍由小型事务控制面负责。
7. 达到 `[完整性/延迟/成本]` 门禁后冻结旧写入口，保留旧表只读，最后按消费者归零证据下线。

### Result

```text
backfill 完成时间: [真实值]
canonical mismatch: [切流前/后真实值]
API P95/P99: [真实值]
在线 PostgreSQL CPU/I/O: [真实值]
单位查询扫描/存储成本: [真实值]
迁移完成率与遗留范围: [真实值]
```

如果没有完整成功案例，也可以诚实讲“迁到哪一阶段、为何暂停、阻止了什么风险”。Staff 信号来自
高质量判断和风险控制，不是必须把项目包装成全赢。

## 跨团队运行机制

| 机制 | 目的 |
|------|------|
| 单一 migration DRI + 每系统 owner | 避免“所有人负责”等于没人负责 |
| 决策日志与风险登记 | 记录 owner、概率、影响、触发器、缓解与截止日期 |
| 每阶段 go/no-go review | 用门禁而非进度压力决定是否推进 |
| 统一 dashboard | 各团队看到同一水位、diff、lag 和 SLO |
| office hour / migration kit | 降低消费者 adoption 成本 |
| break-glass 与停止权 | 当 guardrail 触发时任何值班方可暂停 |
| deprecation deadline | 给旧路径 owner、迁移支持和升级机制 |

## 追问链

1. **双写为什么危险？**  
   两个系统没有共同提交边界，会出现部分成功和 unknown；必须有单一 authority、outbox/CDC、
   幂等和补偿协议。
2. **Shadow 一致率 99.99% 能否切？**  
   先看剩余 0.01% 的类型和业务影响；一笔资金错误可能比百万条展示差异更重要。
3. **如何选择 canary cohort？**  
   选能隔离故障、代表真实工作负载且可回退的边界，不盲目随机百分比。
4. **代码能回滚为何数据不能？**  
   新版本可能已写入旧代码不理解的 schema/event，或触发不可逆外部副作用。
5. **何时删除旧系统？**  
   消费者归零、回滚窗口结束、数据保留/审计完成、恢复演练通过且 owner 签字后。
6. **项目延期怎么向管理层汇报？**  
   以风险、未通过门禁、选项和新预测说明；不隐藏坏消息，也不只报“完成百分比”。

## 反模式与错误表达

- “新旧系统双写一段时间，最终自然一致。”
- “Shadow 结果差不多就可以切。”
- “先全量迁完，再补监控和回滚。”
- “应用版本能 rollback，所以数据库也能。”
- “随机切 1% 流量适合所有系统。”
- “迁移完成”但旧 writer、cron 和人工脚本仍在运行。
- 用示例里的链数、性能或成本数字冒充自己的项目结果。

## 延伸阅读

- [Software Engineering at Google：Large-Scale Changes](https://abseil.io/resources/swe-book/html/ch22.html)
- [Google SRE Workbook：Canarying Releases](https://sre.google/workbook/canarying-releases/)
- [S-NODE-10 链数据列存](../19-node-rpc-staking/S-NODE-10-chain-data-clickhouse-lakehouse.md)
- [S-CLOUD-10 Helm 与 GitOps](../09-cloud-native/S-CLOUD-10-helm-gitops-rollout-rollback.md)

