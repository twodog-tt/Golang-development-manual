---
id: S-RMQ-03
title: RocketMQ 与 Kafka 选型对比
module: rocketmq
level: senior
frequency: 4
go_version: "1.22+"
tags: [rocketmq, kafka, comparison, middleware]
status: published
code_refs: []
sources:
  - https://kafka.apache.org/documentation/
  - https://rocketmq.apache.org/docs/introduction/02advantage/
  - https://developer.cloud.tencent.com/article/2647941
---

# RocketMQ 与 Kafka 选型对比

## 30 秒版（开场）

> **Kafka** 强项：**超高吞吐、日志流、生态（Flink/Connect）**；分区有序、副本 ISR。**RocketMQ** 强项：**事务消息、延迟级别、Tag 过滤、国内运维经验**。Go 后端选 MQ 看 **语义需求** 而非单纯 QPS。面试答：**场景 + 一致性 + 运维 + 团队熟悉度**。

## 3 分钟版（一面深度）

| 维度 | Kafka | RocketMQ |
|------|-------|----------|
| 定位 | 分布式日志 / 流平台 | 业务型 MQ |
| 吞吐 | 极高 | 高（万级 TPS 足够多数业务） |
| 顺序 | 分区内有序 | Queue 内有序 |
| 延迟消息 | 需自建 / 时间轮 | 内置 delay level |
| 事务消息 | 幂等 + 事务 API | 半消息 + 回查 |
| 消费模型 | Consumer Group + partition 绑定 | Consumer Group + Queue 负载均衡 |
| 运维 | KRaft/ZK，分区再均衡 | NameServer + Broker 主从 |
| 生态 | 大数据主流 | 国内云、电商 |

## 10 分钟版（架构取舍）

**选 Kafka**

- 日志采集、埋点、CDC 入湖
- 百万级吞吐、多消费者独立读同流
- 团队已有 Kafka Connect / Flink 流水线

**选 RocketMQ**

- 订单、支付、通知等业务 MQ
- 需要 **事务消息、固定延迟关单**
- 国内部署文档、云厂商托管成熟

**Go 服务注意**

- 均需 **消费幂等**（at-least-once）
- Rebalance 期间可能重复/短暂不可用
- 大消息：Kafka `max.message.bytes`；RocketMQ 默认 4MB 可调

```mermaid
quadrantChart
    title MQ 选型（示意）
    x-axis 低业务语义 --> 高业务语义
    y-axis 低吞吐 --> 高吞吐
    quadrant-1 RocketMQ
    quadrant-2 Kafka
    quadrant-3 Redis Stream
    quadrant-4 自研队列
```

## 生产场景

- 字节/阿里系 JD：常问 Kafka 消费语义 + 若做过电商会问 RocketMQ 事务
- 双 MQ 并存：日志走 Kafka，核心业务走 RocketMQ（成本与复杂度上升）

## 追问链

1. **两者都是拉消费？** → Kafka poll；RocketMQ Push 实为长轮询拉。
2. **谁更适合顺序？** → 都靠 key 绑定固定分区/Queue；Kafka 分区数决定并行度。
3. **Exactly-once？** → 两端多为 at-least-once + 业务幂等；Kafka EOS 限流式场景。
4. **迁移成本？** → Topic 模型、客户端、运维工具全换；用 **双写 + 对账** 渐进。

## 反模式与事故

- **日志型 Kafka 任务用超大消息** → broker 压力
- **为统一而强行只留一种 MQ** → 场景不匹配
- **不看消费组 Rebalance 策略** → 重复消费打爆下游

## 延伸阅读

- [Kafka 文档](https://kafka.apache.org/documentation/)
- [Kafka 消费语义](../kafka/S-DIST-04-kafka-semantics.md)
- [MQ 通用语义](../../03-system-design/S-ARCH-10-mq-semantics.md)
