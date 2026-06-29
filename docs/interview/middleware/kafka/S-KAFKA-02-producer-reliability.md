---
id: S-KAFKA-02
title: Kafka Producer 可靠性：acks、幂等与分区键
module: kafka
level: senior
frequency: 5
go_version: "1.22+"
tags: [kafka, producer, acks, idempotent, partition-key]
status: published
code_refs: []
sources:
  - https://kafka.apache.org/documentation/#producerconfigs
  - https://kafka.apache.org/documentation/#semantics
  - https://github.com/segmentio/kafka-go
---

# Kafka Producer 可靠性：acks、幂等与分区键

## 30 秒版（开场）

> Producer 可靠性靠 **`acks` + `retries` + 幂等 Producer**；与 [S-ARCH-04 幂等](../../03-system-design/S-ARCH-04-idempotency.md) 配合才能端到端不重复。**Partition Key** 决定消息进哪个 partition，影响 **顺序与热点**。生产关键词：**acks=all、enable.idempotence、max.in.flight、key 设计**。

## 3 分钟版（一面深度）

1. **是什么**：`acks=0/1/all` 控制 broker 确认深度；幂等 Producer（PID + sequence）防 **单分区重复**；Key hash 绑定 partition。
2. **为什么**：网络抖动、broker 切换会导致重试重复；交易所 **orderId / symbol** 的 key 选错会破坏顺序或打爆单 partition。
3. **怎么做**：生产 `acks=all` + `min.insync.replicas≥2`；开启 `enable.idempotence=true`；`max.in.flight.requests.per.connection=1` 若需严格单分区顺序；消费端仍要业务幂等。

## 10 分钟版（原理 + 图示）

**acks 语义**

| acks | 行为 | 延迟 | 丢消息风险 |
|------|------|------|------------|
| 0 | 不等 broker 确认 | 最低 | 最高 |
| 1 | Leader 写入即返回 | 中 | Leader 宕机未同步 |
| all/-1 | ISR 全部确认 | 较高 | 最低（配合 minISR） |

**幂等 Producer**

```mermaid
sequenceDiagram
  participant P as Producer PID
  participant B as Broker
  P->>B: seq=1 msg A
  P->>B: seq=2 msg B
  Note over P,B: 重试 seq=1 被 broker 去重
```

- 开启后：`enable.idempotence=true` 自动设 `acks=all`、`retries>0`
- **只保证单连接单 partition 不重复**；跨 partition、跨会话仍可能重复
- 与 **事务 Producer** 区别：事务可原子写多 partition（流处理场景）

**Partition Key 策略**

| 场景 | Key | 效果 |
|------|-----|------|
| 同订单全生命周期 | `orderId` | 顺序 + 局部热点可接受 |
| 同交易对撮合 | `symbol` | 行情/成交顺序 |
| 均匀打散 | null / random | 最高并行，无顺序 |
| 用户维度 | `userId` | 注意大 V 热点 partition |

## 生产场景

- **成交上报**：key=`symbol`；value 含 `tradeId` 供消费幂等
- **充提状态变更**：key=`withdrawId`；配合 Outbox 发消息（[S-SOL-03](../../11-solution-architecture/S-SOL-03-event-driven-cqrs.md)）
- **批量 flush**：`linger.ms` + `batch.size` 换吞吐，监控 P99 延迟

## 排查与工具

| 现象 | 排查 |
|------|------|
| 消息丢失 | acks 配置、minISR、broker 日志 NOT_ENOUGH_REPLICAS |
| 重复消费 | 消费端幂等表；Producer 是否未开幂等 |
| 单 partition lag 高 | key 热点；考虑 salt `userId#shard` |
| 发送超时 | `delivery.timeout.ms`、broker 负载、网络 |

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| acks=all + 幂等 | 资金/订单 | 纯日志采集 |
| acks=1 | 可容忍极少丢失的 metrics | 核心账务 |
| 无 key 轮询 | 高并行埋点 | 顺序业务 |
| 事务 Producer | Kafka Streams EOS | 简单 Go 微服务（过重） |

## 追问链

1. **幂等 Producer 和 DB 唯一键？** → Producer 防 **重复发**；消费仍可能重复 **处理**，要 DB/Redis 去重。
2. **max.in.flight=5 为何影响顺序？** → 重试可能导致后发的先到；严格顺序设 1 或单线程发送。
3. **Go 客户端？** → `segmentio/kafka-go` Writer；`IBM/sarama` SyncProducer/AsyncProducer。
4. **和 [S-DIST-04](./S-DIST-04-kafka-semantics.md) 关系？** → 本题 Producer；DIST-04 消费 + rebalance。

## 反模式与事故

- `acks=1` 发资金事件 → Leader 宕机丢消息
- 全用 `userId` 作 key，头部用户打满单 partition
- 只开 Producer 幂等、消费无去重 → 仍重复入账
- 超大 message 未调 `max.message.bytes` → 发送失败静默丢

## 代码示例

```go
w := &kafka.Writer{
    Addr:                   kafka.TCP("kafka:9092"),
    Topic:                  "withdraw.status",
    Balancer:               &kafka.Hash{},
    RequiredAcks:           kafka.RequireAll,
    AllowAutoTopicCreation: false,
}
err := w.WriteMessages(ctx, kafka.Message{
    Key:   []byte(withdrawID),
    Value: payload,
    // kafka-go 通过 Transport 配置幂等；sarama 用 Config.Producer.Idempotent = true
})
```

## 延伸阅读

- [Kafka Producer Configs](https://kafka.apache.org/documentation/#producerconfigs)
- [S-KAFKA-01 架构与 ISR](./S-KAFKA-01-architecture-storage.md)
- [S-ARCH-04 幂等设计](../../03-system-design/S-ARCH-04-idempotency.md)
