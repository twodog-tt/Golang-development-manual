# 中间件与数据库（按类型浏览）

按 **MySQL / Redis / Kafka / RocketMQ / Elasticsearch** 分类，便于按 JD 或技术栈刷题。

| 类型 | 题数 | 入口 |
|------|------|------|
| [MySQL + GORM](./mysql/) | 5 | 索引、MVCC、慢查询、分库分表、GORM |
| [Redis](./redis/) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](./kafka/) | 1 | 消费语义、Rebalance |
| [RocketMQ](./rocketmq/) | 3 | 架构、事务/顺序/延迟、与 Kafka 对比 |
| [Elasticsearch](./elasticsearch/) | 3 | 倒排索引、DSL、同步运维 |
| [分布式事务](./distributed/) | 1 | TCC / Saga |

**关联系统设计**：缓存见 [03-system-design](../03-system-design/S-ARCH-06-cache-failure-modes.md)；MQ 通用语义见 [S-ARCH-10](../03-system-design/S-ARCH-10-mq-semantics.md)。

学习路径仍可按模块 [04 分布式](../04-distributed-middleware/) / [05 数据库](../05-database-storage/) 顺序阅读（内容与本文目录一致）。
