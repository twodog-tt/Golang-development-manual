---
id: S-MSVC-06
title: 事件总线与异步服务边界（交易所）
module: microservices-exchange
level: senior
frequency: 5
go_version: "1.22+"
tags: [kafka, rocketmq, event-driven, microservices, cex, dex, async]
status: published
resume_focus: true
code_refs: []
sources:
  - https://kafka.apache.org/documentation/#semantics
  - https://microservices.io/patterns/data/transactional-outbox.html
---

# 事件总线与异步服务边界（交易所）

## 30 秒版（开场）

> 交易所 **成交、链上日志、充提状态** 适合走 **Kafka/RocketMQ** 扇出；**下单风控、余额冻结** 走同步 gRPC。Topic 按 **领域事件** 划分：`trade.matched`、`deposit.confirmed`、`chain.swap`、`withdraw.status`。关键词：**分区键保序、消费幂等、Outbox、DLQ 补账**。

## 3 分钟版（一面深度）

1. **是什么**：微服务间通过消息中间件传递领域事件，实现解耦与削峰。
2. **为什么**：成交后要同时驱动账务、行情、风控、大数据；DEX Indexer 吞吐波动大。
3. **怎么做**：本地事务 + Outbox 发消息；消费者幂等；关键资金消费者 **独立消费组**。

## 10 分钟版

### 事件流全景

```mermaid
flowchart LR
  ME[matching-svc] -->|trade.matched| K[Kafka]
  Idx[indexer-svc] -->|chain.swap| K
  Wallet[wallet-svc] -->|deposit.confirmed| K
  K --> Ledger[ledger-svc CG]
  K --> Market[market-svc CG]
  K --> Risk[risk-svc CG]
  K --> Kline[kline-svc CG]
  K --> Rebate[rebate-svc CG]
  K --> DW[数仓 Flink CG]
```

### Topic 设计（交易所）

| Topic | 生产者 | 消费者 | 分区键 |
|-------|--------|--------|--------|
| `trade.matched` | matching | ledger, market, risk | 需要保留撮合顺序时用 `symbol/orderBookId`；`trade_id` 会把相邻成交随机打散 |
| `order.lifecycle` | order-svc | 审计、推送 | `user_id` |
| `chain.swap` | indexer | kline, rebate | `pool_address` |
| `chain.deposit` | indexer/wallet | ledger | 按链上地址/账户或已归属的 account_id；归属前未必有 user_id |
| `withdraw.status` | wallet | ledger, notify | `withdraw_id` |

**分区键原则**：先写出业务需要的顺序域，再选 key。单分区只提供该 topic/partition
中的顺序，并要求生产者正确处理重试与 epoch；它不提供跨 topic、跨数据库的“全局顺序”。

### 同步 vs 异步决策表

| 操作 | 通道 | 原因 |
|------|------|------|
| 下单风控 | gRPC | 立即反馈 |
| 成交后入账 | MQ | 削峰、可重放 |
| 链上 Swap 入库 | MQ | Indexer 与 API 解耦 |
| 推送 WS 行情 | MQ → market → WS | fan-out |
| 资金费率结算 | 定时任务 + MQ | 批量 |

### 可靠性

| 机制 | 说明 |
|------|------|
| Outbox | 每个服务在自己的本地事务中写业务表 + outbox，relay/CDC 发 Kafka |
| 幂等 | `uk(event_id)` 或业务键 `trade_id` |
| DLQ | 对可跳过的独立事件可死信；资金/有序流应暂停或隔离相关 key/partition，保留原 offset、原因和可控重放 |
| 重放 | 需要 immutable 原始事件、schema/规则版本和确定性处理；新 group 从 offset 开始只是机制，不自动保证能重建 |

参见 [S-KAFKA-02](../middleware/kafka/S-KAFKA-02-producer-reliability.md)、[S-RAB-01](../middleware/rabbitmq/S-RAB-01-exchange-async-pipeline.md)

### CEX vs DEX 事件差异

| 类型 | CEX 事件 payload | DEX 事件 payload |
|------|------------------|------------------|
| 成交 | price, qty, maker/taker | — |
| Swap | — | pool, amountIn/Out, sender, tx_hash |
| Reorg | — | `removed=true` 冲正 |

### Kafka vs RocketMQ（交易所口述）

| 维度 | Kafka | RocketMQ |
|------|-------|----------|
| 吞吐/生态 | 高吞吐日志与流处理生态成熟 | 高吞吐消息，事务/顺序/延时能力取决于使用模式与版本 |
| 顺序 | 分区内有序 | 单 Queue 有序 |
| 延迟消息 | 通常配合调度器或专用延时 topic | 内置延时/定时消息能力，精度与限制按版本确认 |
| 典型用途 | trade 总线 | 充提通知、关单 |

## 生产场景

- **lag 堆积**：ledger consumer 扩容；临时降级非关键消费者（数仓）
- **重复消息**：账务必须幂等；depth delta、K 线 volume 等行情写也可能被重复累计，
  应使用 event id/sequence/version，而不是假设“多写无害”
- **消息过大**：深度快照走对象存储，MQ 只传引用

## 追问链

1. **为何不用 RPC 广播成交？** → N 个订阅者耦合；MQ 可重放审计。
2. **Exactly-once？** → 端到端难；交易所 **至少一次 + 幂等** 务实。
3. **与 S-ARCH-10 关系？** → ARCH-10 讲语义；本题 **交易所 Topic 划分**。

## 反模式

- 下单成功靠 MQ 回调用户（应用 WebSocket/轮询订单状态）
- 一个 `exchange-events` 大 Topic 无 schema
- 消费者里跨库写 ledger+wallet

## 代码示例

```go
// Outbox relay（简化）
type OutboxEvent struct {
    ID        int64
    Topic     string
    Key       string
    Payload   []byte
    Published bool
}
// 定时或 CDC 扫描 unpublished → Kafka Publish → 标记 published
```

“发布成功但标记 published 前宕机”仍会重复发送，因此 Outbox 解决丢消息窗口，
不是 exactly-once；消费者和 relay 都要接受重复。

## 延伸阅读

- [S-KAFKA-03 交易事件总线](../middleware/kafka/S-KAFKA-03-trade-event-bus.md)
- [S-MSVC-04 数据一致性](./S-MSVC-04-database-per-service.md)
- [Transactional Outbox](https://microservices.io/patterns/data/transactional-outbox.html)
