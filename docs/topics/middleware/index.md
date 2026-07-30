# 中间件与数据库（按类型浏览）

按 **MySQL / PostgreSQL / Redis / Kafka / RocketMQ / RabbitMQ / Elasticsearch** 分类，便于按岗位场景或技术栈浏览。

| 类型 | 篇数 | 入口 |
|------|------|------|
| [MySQL + GORM](./mysql/index.md) | 7 | 索引、MVCC、复杂 SQL、资金表、锁与 GORM |
| [PostgreSQL](./postgresql/index.md) | 3 | MVCC/VACUUM、SSI/锁、WAL/复制与 pgx |
| [Redis](./redis/index.md) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](./kafka/index.md) | 4 | 架构、Producer、消费语义、交易总线 |
| [RocketMQ](./rocketmq/index.md) | 4 | 架构、事务/顺序/延迟、选型、排障 |
| [RabbitMQ](./rabbitmq/index.md) | 1 | 链上监听与业务异步拆分 |
| [Elasticsearch](./elasticsearch/index.md) | 3 | 倒排索引、DSL、同步运维 |
| [分布式事务](./distributed/index.md) | 1 | TCC / Saga |

**关联系统设计**：缓存见 [03-system-design](../03-system-design/S-ARCH-06-cache-failure-modes.md)；MQ 通用语义见 [S-ARCH-10](../03-system-design/S-ARCH-10-mq-semantics.md)。

岗位优先级以 [角色化矩阵](../_meta/role-priority-matrix.md) 为准；MySQL 与 PostgreSQL 应对照
实现语义，不要把 undo/heap、隔离级别和清理机制混用。
