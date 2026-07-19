# 高频必背题单：18 个核心锚点

这不是 219 篇正文的缩略目录，而是第一轮必须形成口述能力的 **18 个高频锚点**。
顺序按知识依赖排列：**Go 运行时与并发 → 生产后端与系统设计 → Agent Platform**。

> 其中 17 题的题库频率为最高级 `5`。`S-AI-06` 的频率为 `4`，但它直接对应当前简历主线中的
> 成本、延迟、可观测和容量追问，因此按岗位相关性进入核心题单。

[进入配套的 18 张口述卡](high-frequency-oral-cards.md)

## 使用规则

每题已经配套一张口述卡，不背整篇正文：

1. **一句话结论**：30 秒内说明它是什么、解决什么问题。
2. **三个不变量**：讲清正确性边界，而不是堆 API 名称。
3. **一张图**：能闭卷画出状态、调用或数据流。
4. **一个取舍**：说明为什么不选择另一种方案。
5. **一个真实案例**：只使用自己确实参与过的项目证据。
6. **一个错误表达**：提前记住面试中不能说什么。

验收顺序：先闭卷讲 **30 秒版**，再讲 **3 分钟版**；只有面试官追问时才展开 10 分钟细节。
复习间隔建议使用：当天、1 天、3 天、7 天、14 天、30 天。

## 第一周：Go 运行时、并发与内存

目标：先建立资深 Go 岗位的语言基本盘。顺序不要打乱，后面的泄漏和 GC 排查依赖前面的
调度、同步与取消语义。

| 顺序 | 题号 | 题目名称 | 必须讲清 |
|-----:|------|----------|----------|
| 1 | [`S-CONC-01`](interview/01-runtime-concurrency/S-CONC-01-gmp-overview.md) | [GMP 模型与 1.14 以来抢占式调度](interview/01-runtime-concurrency/S-CONC-01-gmp-overview.md) · [口述卡](high-frequency-oral-cards.md#s-conc-01) | G/M/P 分工、本地/全局队列、抢占与 syscall |
| 2 | [`S-CONC-05`](interview/01-runtime-concurrency/S-CONC-05-channel.md) | [Channel 内部实现与有缓冲/无缓冲选型](interview/01-runtime-concurrency/S-CONC-05-channel.md) · [口述卡](high-frequency-oral-cards.md#s-conc-05) | hchan、发送/接收等待队列、关闭语义与背压 |
| 3 | [`S-CONC-08`](interview/01-runtime-concurrency/S-CONC-08-sync-primitives.md) | [Mutex、RWMutex 与 atomic 选型](interview/01-runtime-concurrency/S-CONC-08-sync-primitives.md) · [口述卡](high-frequency-oral-cards.md#s-conc-08) | 临界区、竞争度、内存序与复制锁风险 |
| 4 | [`S-CONC-12`](interview/01-runtime-concurrency/S-CONC-12-context.md) | [Context 树、取消传播与泄漏](interview/01-runtime-concurrency/S-CONC-12-context.md) · [口述卡](high-frequency-oral-cards.md#s-conc-12) | 生命周期所有权、deadline、取消传播与 Value 边界 |
| 5 | [`S-CONC-13`](interview/01-runtime-concurrency/S-CONC-13-goroutine-leak.md) | [Goroutine 泄漏成因与 pprof 排查](interview/01-runtime-concurrency/S-CONC-13-goroutine-leak.md) · [口述卡](high-frequency-oral-cards.md#s-conc-13) | 阻塞点、退出协议、goroutine profile 与生产止损 |
| 6 | [`S-MEM-01`](interview/02-memory-gc/S-MEM-01-tri-color-gc.md) | [三色标记与混合写屏障](interview/02-memory-gc/S-MEM-01-tri-color-gc.md) · [口述卡](high-frequency-oral-cards.md#s-mem-01) | 并发标记、写屏障、STW 边界与 GC 调优代价 |

第一周验收：能从“请求进入 Go 服务”一路讲到 goroutine 被调度、阻塞、取消、泄漏和被 GC
观察的完整生命周期。

## 第二周：生产后端与系统设计

目标：把语言能力连接到真实服务。学习主线是：
**错误边界 → TCP 请求链路 → 数据库 → 幂等 → 消息事实 → 业务状态机**。

| 顺序 | 题号 | 题目名称 | 必须讲清 |
|-----:|------|----------|----------|
| 7 | [`S-GOENG-01`](interview/16-go-production-engineering/S-GOENG-01-errors-contract-panic-boundary.md) | [错误契约、Wrapping 与 Panic 边界](interview/16-go-production-engineering/S-GOENG-01-errors-contract-panic-boundary.md) · [口述卡](high-frequency-oral-cards.md#s-goeng-01) | errors.Is/As、稳定错误码、日志所有权与 recover 边界 |
| 8 | [`S-NET-07`](interview/06-network-governance/S-NET-07-tcp-lifecycle-queues-timewait.md) | [TCP 建连、队列、TIME_WAIT 与故障排查](interview/06-network-governance/S-NET-07-tcp-lifecycle-queues-timewait.md) · [口述卡](high-frequency-oral-cards.md#s-net-07) | 三次握手、listen/accept 队列、重传、TIME_WAIT |
| 9 | [`S-DB-01`](interview/middleware/mysql/S-DB-01-mysql-index.md) | [MySQL 索引原理与最左前缀](interview/middleware/mysql/S-DB-01-mysql-index.md) · [口述卡](high-frequency-oral-cards.md#s-db-01) | B+Tree、联合索引、回表、覆盖索引与执行计划 |
| 10 | [`S-ARCH-04`](interview/03-system-design/S-ARCH-04-idempotency.md) | [幂等设计：接口、消息、数据库层](interview/03-system-design/S-ARCH-04-idempotency.md) · [口述卡](high-frequency-oral-cards.md#s-arch-04) | 业务键、状态约束、模糊成功与重试边界 |
| 11 | [`S-ARCH-10`](interview/03-system-design/S-ARCH-10-mq-semantics.md) | [消息队列：至少一次、恰好一次、顺序性](interview/03-system-design/S-ARCH-10-mq-semantics.md) · [口述卡](high-frequency-oral-cards.md#s-arch-10) | broker 语义、消费幂等、顺序域与重复/缺口检测 |
| 12 | [`S-ARCH-12`](interview/03-system-design/S-ARCH-12-order-state-machine.md) | [支付/订单状态机设计](interview/03-system-design/S-ARCH-12-order-state-machine.md) · [口述卡](high-frequency-oral-cards.md#s-arch-12) | 合法迁移、事实与投影、补偿、超时和人工介入 |

第二周验收：能用同一个“状态机 + 幂等 + 事实事件”框架解释订单、支付、Agent 发布和链上交易。

## 第三周：Agent Platform 主线

目标：从“调用一次模型”升级到“可审计、可恢复、可控成本的 Agent 平台”。学习主线是：
**工具调用 → 工作流/HITL → Persona/Memory → 安全 → 成本 → 实时反馈数据**。

| 顺序 | 题号 | 题目名称 | 必须讲清 |
|-----:|------|----------|----------|
| 13 | [`S-AI-03`](interview/10-ai-engineering/S-AI-03-agent-tool-calling.md) | [AI Agent 与 Function Calling](interview/10-ai-engineering/S-AI-03-agent-tool-calling.md) · [口述卡](high-frequency-oral-cards.md#s-ai-03) | 模型建议与系统授权边界、schema 校验、工具副作用 |
| 14 | [`S-AI-09`](interview/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) | [Agent 工作流、Human-in-the-loop 与可靠发布控制面](interview/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md) · [口述卡](high-frequency-oral-cards.md#s-ai-09) | checkpoint 与业务幂等、Review Queue、发布模糊成功 |
| 15 | [`S-AI-10`](interview/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md) | [Persona、分层 Memory 与反馈学习治理](interview/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md) · [口述卡](high-frequency-oral-cards.md#s-ai-10) | Persona/Context/Memory 分层、租户隔离、反馈治理 |
| 16 | [`S-AI-05`](interview/10-ai-engineering/S-AI-05-llm-security.md) | [LLM 应用安全：注入、PII、护栏](interview/10-ai-engineering/S-AI-05-llm-security.md) · [口述卡](high-frequency-oral-cards.md#s-ai-05) | 不可信输入、提示注入、PII、输出与工具双重校验 |
| 17 | [`S-AI-06`](interview/10-ai-engineering/S-AI-06-llm-observability-cost.md) | [LLM 可观测性、成本与延迟优化](interview/10-ai-engineering/S-AI-06-llm-observability-cost.md) · [口述卡](high-frequency-oral-cards.md#s-ai-06) | token/cost、首 token/P99、配额、缓存与降级 |
| 18 | [`S-ARCH-21`](interview/03-system-design/S-ARCH-21-realtime-risk-cdc-flink.md) | [实时风控数据平台：CDC、Flink、ES 与可重放链路](interview/03-system-design/S-ARCH-21-realtime-risk-cdc-flink.md) · [口述卡](high-frequency-oral-cards.md#s-arch-21) | CDC 一致性、事件时间、重放、ES 幂等与反馈闭环 |

第三周验收：以个人 Agent 项目为主线，能说明一次任务如何经过输入治理、工具授权、工作流、
人工审核、外部发布、成本观测和反馈回流。

## 14 天执行方式

| 日期 | 学习动作 |
|------|----------|
| Day 1～3 | 每天新学第一周中的 2 题，并复述前一天内容 |
| Day 4 | 第一周 6 题闭卷串讲，手画 GMP 与请求取消图 |
| Day 5～7 | 每天新学第二周中的 2 题 |
| Day 8 | 用一个订单或支付案例串讲幂等、MQ 与状态机 |
| Day 9～11 | 每天新学第三周中的 2 题 |
| Day 12 | 用个人 Agent 项目串讲 6 题，不看正文 |
| Day 13 | 随机抽取 6 题做 30 秒版 + 3 分钟版 |
| Day 14 | 进行一次模拟面试，只回补暴露出的错误和知识洞 |

## 完成后再进入岗位增量

只有当这 18 题可以闭卷口述，才继续进入：

- [七类岗位 P0/P1/P2 与证据标签](interview/_meta/role-priority-matrix.md)
- [简历定向 P0 知识图谱](interview/_meta/p0-knowledge-graph.md)
- [Web3 交易所与钱包重点题单](resume-focus-web3.md)
- [模拟面试](mock-interview.md)

不要因为某题“看过”就标记完成。完成的标准是：**先说结论，能画图，再用真实项目举证，
并能指出至少一个错误表达。**
