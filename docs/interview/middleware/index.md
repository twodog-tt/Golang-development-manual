# 中间件与数据库（按类型浏览）

按 **MySQL / Redis / Kafka / RocketMQ / RabbitMQ / Elasticsearch** 分类，便于按 JD 或技术栈刷题。

| 类型 | 题数 | 入口 |
|------|------|------|
| [MySQL + GORM](./mysql/index.md) | 5 | 索引、MVCC、慢查询、分库分表、GORM |
| [Redis](./redis/index.md) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](./kafka/index.md) | 4 | 架构、Producer、消费语义、交易总线 |
| [RocketMQ](./rocketmq/index.md) | 4 | 架构、事务/顺序/延迟、选型、排障 |
| [RabbitMQ](./rabbitmq/index.md) | 1 | 链上监听与业务异步拆分 |
| [Elasticsearch](./elasticsearch/index.md) | 3 | 倒排索引、DSL、同步运维 |
| [分布式事务](./distributed/index.md) | 1 | TCC / Saga |

**关联系统设计**：缓存见 [03-system-design](../03-system-design/S-ARCH-06-cache-failure-modes.md)；MQ 通用语义见 [S-ARCH-10](../03-system-design/S-ARCH-10-mq-semantics.md)。

学习路径仍可按模块 [04 分布式](../04-distributed-middleware/index.md) / [05 数据库](../05-database-storage/index.md) 顺序阅读（内容与本文目录一致）。
