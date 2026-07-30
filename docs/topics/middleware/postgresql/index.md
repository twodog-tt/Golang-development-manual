# PostgreSQL

3 篇 | Go 后端、支付与 Staff/架构岗位 P0～P1 | [返回中间件索引](../index.md)

> 重点不是把 MySQL 术语换成 PostgreSQL，而是讲清 **tuple version、VACUUM、SSI、WAL 提交证据、复制滞后和 pgx 连接生命周期**。

| ID | 标题 | 频率 |
|----|------|------|
| [S-PG-01](./S-PG-01-mvcc-vacuum-indexes.md) | MVCC、VACUUM、可见性与索引设计 | ⭐⭐⭐⭐⭐ |
| [S-PG-02](./S-PG-02-isolation-locking-ledger.md) | 隔离级别、锁与资金写入 | ⭐⭐⭐⭐⭐ |
| [S-PG-03](./S-PG-03-wal-replication-pgx-ha.md) | WAL、复制、故障切换与 pgx 连接治理 | ⭐⭐⭐⭐⭐ |

## 推荐顺序

MVCC/索引 → 并发正确性 → WAL/HA/连接池。之后联动：

- MySQL 对照：[S-DB-02 事务与 MVCC](../mysql/S-DB-02-transaction-mvcc.md)
- 资金建模：[S-DB-07 资金表、约束与锁](../mysql/S-DB-07-financial-schema-locking.md)
- 跨服务一致性：[S-DIST-05 分布式事务](../distributed/S-DIST-05-distributed-transaction.md)

