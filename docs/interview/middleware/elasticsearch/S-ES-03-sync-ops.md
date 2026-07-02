---
id: S-ES-03
title: Elasticsearch 数据同步与运维
module: elasticsearch
level: senior
frequency: 4
go_version: "1.22+"
tags: [elasticsearch, sync, canal, cluster, ops]
status: published
code_refs: []
sources:
  - https://www.elastic.co/docs/solutions/search/ingest-for-search
  - https://www.elastic.co/guide/en/elasticsearch/reference/current/modules-cluster.html
---

# Elasticsearch 数据同步与运维

## 30 秒版（开场）

> ES 数据通常 **从 MySQL 同步**（Canal/Debezium → MQ → ES Consumer，或 Logstash/Flink）。原则：**MySQL 为源、ES 可重建**；同步延迟可接受则 **近实时搜索**。运维关注 **分片均衡、磁盘 watermark、集群颜色、版本升级**。

## 3 分钟版（一面深度）

1. **同步路径**：全量（DB scan + bulk）+ 增量（binlog CDC）；幂等写 ES（同 doc id upsert）。
2. **一致性**：延迟秒级；失败进 DLQ 重试；定期 **对账** MySQL count vs ES count。
3. **运维**：黄/红集群处理；节点磁盘 >85% 只读；滚动重启；快照 snapshot 备份。

## 10 分钟版（同步架构）

```mermaid
flowchart LR
  MySQL[(MySQL)] -->|binlog| Canal[Canal / Debezium]
  Canal --> MQ[Kafka/RocketMQ]
  MQ --> Worker[Go ES Writer]
  Worker --> ES[(Elasticsearch)]
```

| 方式 | 优点 | 缺点 |
|------|------|------|
| 双写 | 简单 | 不一致、难回滚 |
| CDC + MQ | 解耦、可重放 | 链路长 |
| Logstash JDBC | 配置快 | 增量能力弱 |

**Go Consumer 要点**

- Bulk 批量（500～5000 条/批）+ `refresh=false`
- 文档 `_id` = 业务主键，天然 upsert
- 消费失败不 commit offset； poison 进 DLQ

**与分库分表**（见 [S-DB-04](../mysql/S-DB-04-sharding.md)）

- 跨片列表：ES 聚合检索
- 或 ES 存宽表冗余字段，避免 join

## 生产场景

- 商品库 MySQL，搜索走 ES；大促前 **全量 reindex** 预热
- 索引别名 `products_v2` 切换零停机

## 排查与工具

- `_cat/shards` 未分配分片
- `_cluster/allocation/explain`
- 同步延迟：MQ lag + ES 写入 TPS

## 架构取舍

| 方案 | 适用 |
|------|------|
| 可重建 ES | 接受丢索引从 MySQL 全量恢复 |
| 跨集群复制 CCR | 异地灾备 |
| 冷热架构 ILM | 日志历史降冷节点 |

## 追问链

1. **同步丢消息？** → MQ 持久化 + 至少一次 + ES 幂等 id。
2. **删除怎么同步？** → binlog DELETE 事件删 ES doc。
3. **mapping 变更？** → 新 index + reindex + alias 切换。
4. **集群红了？** → 未分配副本、磁盘满、版本不兼容；先扩盘/删旧索引。

## 反模式与事故

- **双写无对账** → 搜索有货 DB 无货
- **bulk 无背压** → ES 拒绝写入 429
- **单 giant shard** → 无法恢复、迁移慢

## 延伸阅读

- [Elasticsearch 数据写入](https://www.elastic.co/docs/solutions/search/ingest-for-search)
- 关联：[S-DB-04 分库分表](../mysql/S-DB-04-sharding.md)
