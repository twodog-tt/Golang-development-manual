---
id: S-ARCH-10
title: 消息队列：至少一次、恰好一次、顺序性
module: system-design
level: senior
frequency: 5
go_version: "1.22+"
tags: [mq, kafka, rabbitmq, at-least-once, ordering]
status: published
code_refs: []
sources:
  - https://kafka.apache.org/documentation/#semantics
---

# 消息队列：至少一次、恰好一次、顺序性

## 30 秒版（开场）

> 分布式 MQ **最常见的是至少一次（At-Least-Once）**。Kafka EOS 能覆盖 Kafka topic 之间的 read-process-write 事务，但不会自动把外部数据库、支付 API 等副作用纳入同一事务；业务上的“效果恰好一次”仍需本地事务、幂等键、outbox/inbox 与状态机。

## 3 分钟版（一面深度）

1. **是什么**：At-Most-Once 可能丢；At-Least-Once 可能重复；Exactly-Once 语义复杂，多为「效果上的恰好一次」。
2. **为什么**：网络、 rebalance、进程 crash 都会导致重复或丢失；顺序与吞吐矛盾。
3. **怎么做**：默认 At-Least-Once + 业务幂等；Kafka 幂等 producer + 事务；顺序消息用相同 partition key；失败进 DLQ 人工/自动重试。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart LR
  Prod[Producer] --> Broker[(Kafka/RocketMQ)]
  Broker --> Cons1[Consumer Group]
  Cons1 --> Dedup[幂等去重]
  Dedup --> Biz[业务处理]
  Biz -->|失败| DLQ[死信队列]
  Biz -->|成功| Commit[Offset Commit]
```

**语义对照**

| 语义 | 行为 | 实现成本 | 典型 |
|------|------|----------|------|
| At-Most-Once | 可能丢 | 低 | 日志采集可丢 |
| At-Least-Once | 可能重复 | 中 | 默认，需幂等 |
| Exactly-Once / Effectively-Once | 在明确边界内不重复产生可见效果 | 高 | Kafka EOS（Kafka 内）或业务事务 + 幂等 |
| 顺序 | 分区内有序 | 降并行 | 订单状态变更 |

**Kafka 要点**

- **幂等 Producer**：`enable.idempotence=true`，PID+序列号防 broker 重试重复。
- **事务**：可原子提交多个 Kafka 分区写入及消费 offset，适合 Kafka 内 read-process-write；外部 DB 写不在该事务中。
- **Consumer**：先处理再 commit offset；crash 后重复消费 → **必须幂等**。

**容量估算**

- 订单 1 万 TPS、消息 1KB 只给出约 10 MB/s 原始载荷；还需计算副本、保留期、压缩、峰值、磁盘 IOPS、故障冗余与分区并行度，不能据此断言“三节点足够”。
- Consumer lag 目标 < 1000 条或 < 1s；lag 10 万需 **扩 consumer 或优化处理**（注意分区数上限）。

## 生产场景

- **订单创建 → 库存/积分/通知**：At-Least-Once + `order_id` 幂等。
- **Binlog CDC**：源端通常按日志位置读取；重新按主键分区后只保同一 key 的顺序，可能失去跨 key/跨表事务顺序。
- **延迟消息**：RocketMQ 5.x 支持定时/延时消息；Kafka 没有通用的原生延时队列，通常用 delay topics、应用调度器或流处理状态实现。

## 排查与工具

| 工具 | 用途 |
|------|------|
| `kafka-consumer-groups.sh --describe` | lag 监控 |
| DLQ 深度 | 失败堆积 |
| 幂等表重复率 | 消费重复是否正常 |
| trace 关联 message_id | 端到端 |

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| At-Least-Once + 幂等 | 绝大多数业务 | 无法去重的副作用 |
| Kafka 事务 | 流处理 exactly-once | 简单业务过重 |
| 单分区顺序 | 强顺序单实体 | 高吞吐全局顺序 |
| 内存队列 | 单进程 | 持久化、跨服务 |

## 追问链

1. **先 commit 还是先处理？** → 先处理再 commit（至少一次）；先 commit 可能丢（至多一次）。
2. **如何保证全局顺序？** → 单分区，吞吐受限；或业务层按 version 丢弃乱序。
3. **Rebalance 时重复消费？** → 正常，幂等兜底； cooperative sticky 减少 rebalance。
4. **Go 消费 Kafka 用什么？** → `segmentio/kafka-go`、`IBM/sarama`；注意 consumer group。
5. **DLQ 之后怎么办？** → 告警 + 人工修复 + 回放工具；根因分类（ poison message 跳过）。

## 反模式与事故

- 认为 MQ 保证 exactly-once，不做幂等，重复扣款。
- 分区数=1 追求全局顺序，吞吐瓶颈。
- 无 DLQ，失败消息无限重试阻塞队列。
- Consumer 处理 30s 未 ack，session timeout 反复 rebalance。

## 代码示例

```go
// 幂等消费骨架
func (h *Handler) Handle(ctx context.Context, msg *kafka.Message) error {
    bizID := extractBizID(msg)
    return h.repo.WithTx(ctx, func(tx Tx) error {
        inserted, err := tx.TryInsertProcessed(bizID)
        if err != nil {
            return err
        }
        if !inserted {
            return nil
        }
        return h.processInTx(ctx, tx, msg)
    })
}
```

只有当去重记录和业务变更在同一本地事务中提交，才能关闭“标记成功但业务未做”与“业务已做但标记未写”的崩溃窗口。外部 HTTP/链上交易等副作用需继续传递业务幂等键并记录状态，不能由这张表单独兜底。

## 延伸阅读

- [Kafka Semantics](https://kafka.apache.org/documentation/#semantics)
- [RocketMQ 领域模型](https://rocketmq.apache.org/docs/domainModel/01main/)
