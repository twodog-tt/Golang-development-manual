---
id: S-PG-02
title: PostgreSQL 隔离级别、锁与资金写入
module: postgresql
level: senior
frequency: 5
go_version: "1.24+"
tags: [postgresql, transaction, isolation, ssi, lock, deadlock, ledger, idempotency]
status: published
resume_focus: true
code_refs: []
sources:
  - https://www.postgresql.org/docs/current/transaction-iso.html
  - https://www.postgresql.org/docs/current/explicit-locking.html
  - https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html
  - https://www.postgresql.org/docs/current/ddl-constraints.html
  - https://www.postgresql.org/docs/current/sql-select.html
---

# PostgreSQL 隔离级别、锁与资金写入

<a id="oral-card"></a>

## 要点卡

[返回 P0 知识图谱](../../_meta/p0-knowledge-graph.md)

!!! abstract "30 秒回答"

    PostgreSQL 默认 `READ COMMITTED` 是**每条语句重新取快照**，不是整个事务固定快照；`REPEATABLE READ` 使用事务级快照，但仍不能把所有跨行不变量都自动变成可串行化；`SERIALIZABLE` 通过 SSI 检测危险依赖，必要时以 `40001` 中止事务，所以应用必须重试整个事务。资金写入还要靠唯一幂等键、原子条件更新、稳定锁顺序、不可变流水和对账；隔离级别不能替代业务约束。

**3 分钟展开**

| 级别 | PostgreSQL 关键语义 | 应用责任 |
|------|---------------------|----------|
| Read Committed | 每条命令看见其开始前已提交的数据；同一事务两次查询可不同 | 用原子 DML、显式行锁或约束保护不变量 |
| Repeatable Read | 首次数据访问建立事务快照；PostgreSQL 实现不出现 phantom read | 仍可能有 serialization anomaly；处理并发更新失败 |
| Serializable | 结果等价于某个串行顺序，否则回滚一个事务 | 对 `serialization_failure` 做有界全事务重试 |

`READ UNCOMMITTED` 在 PostgreSQL 中按 `READ COMMITTED` 处理。不要按其他数据库的默认隔离级别
或实现细节回答。还要知道 Read Committed 下 `UPDATE`/`DELETE`/锁定读遇到并发更新时会等待，
随后对新版本重新检查 `WHERE`；因此一条更新命令可能看到目标行的并发新版本，却看不到对方对
其他行的改动。简单主键更新通常正需要这种语义，复杂跨行搜索条件则必须额外审查。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | RC 是语句级快照；PG RR 是事务级快照但仍可能有序列化异常；Serializable 失败要重试整个事务 |
| 手画图 | `tx snapshot → atomic DML/locks/constraints → commit | 40001/deadlock → whole-tx retry` |
| 项目落点 | 用实际账户扣款或账本写入说明条件更新、唯一幂等键、稳定锁顺序和 reconcile；只引用本人实际参与部分和可解释指标 |
| 一个取舍 | 原子 DML 在 RC 下简单高效；Serializable 更通用但有重试和谓词冲突成本 |

**错误表达**

- ❌ “Repeatable Read 会自动保护所有跨行不变量；收到 40001 只重试失败 SQL。”
- ✅ “PG RR 仍允许 serialization anomaly；重试必须从事务开始重算读取和业务判断。”

**自测追问**：为什么 `UPDATE ... WHERE balance >= amount` 比先查后改稳？Serializable 能否替代唯一约束？

## 10 分钟版（正确性设计）

### 单账户扣款：先把不变量写进一条原子 DML

```sql
UPDATE account_balance
SET available = available - $1,
    version = version + 1
WHERE tenant_id = $2
  AND account_id = $3
  AND currency = $4
  AND available >= $1
RETURNING available, version;
```

返回 0 行表示余额不足、对象不存在或并发条件已变化。它比“先 `SELECT` 余额，再 `UPDATE`”
更稳，因为判断和写入在同一条语句内完成。

表级约束仍应兜底：

```sql
CREATE TABLE account_balance (
    tenant_id  bigint        NOT NULL,
    account_id bigint        NOT NULL,
    currency   text          NOT NULL,
    available  numeric(38,0) NOT NULL CHECK (available >= 0),
    version    bigint        NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, account_id, currency)
);

CREATE UNIQUE INDEX uq_ledger_business_leg
ON ledger_entry (tenant_id, business_id, leg_no);
```

金额应使用最小单位整数或明确精度/舍入规则的 `numeric`。`CHECK` 适合当前行约束；跨行借贷守恒
不能靠一个普通 `CHECK` 表达，通常需要事务内写入双边流水、封装写入口、延迟约束/触发器的
谨慎设计，以及独立对账。

### 多账户过账：锁顺序与事实边界

1. 按稳定键排序，例如 `(tenant_id, account_id, currency)`。
2. 以相同顺序 `SELECT ... FOR UPDATE` 或执行条件更新。
3. 写入不可变 ledger legs，并由唯一业务键保证重放不重复。
4. 更新余额读模型；提交后再通过 outbox 发布事件。
5. 外部 RPC、链上广播或 Webhook 不放进可自动重试的数据库事务。

稳定锁顺序降低死锁概率，但不能证明永不死锁；数据库仍可能以 SQLSTATE `40P01` 终止一个事务。

### `FOR UPDATE`、`NOWAIT`、`SKIP LOCKED`

```sql
SELECT id
FROM payout_job
WHERE status = 'ready'
ORDER BY id
FOR UPDATE SKIP LOCKED
LIMIT 100;
```

这适合多 worker 领取任务，因为业务接受暂时跳过已被其他事务锁住的行。它给出的不是一致性
全貌，所以不应拿来计算总账余额、证明“没有待处理提现”或绕过资金冲突。

- `NOWAIT`：不能立刻拿锁就失败，适合快速反馈或上层重试。
- `SKIP LOCKED`：跳过冲突行，适合队列吞吐。
- advisory lock：由应用定义键空间；数据库不知道它保护哪条业务不变量，所有写入口必须遵守。

### Serializable 重试边界

正确重试单位是**整个事务函数**，不是只重放最后一条 SQL：

```text
begin
  read decision inputs
  validate invariant
  write ledger and balance
commit

if SQLSTATE == 40001 or approved deadlock retry:
  bounded backoff + jitter
  rerun from begin with the same business idempotency key
```

- `40001` 是预期的并发控制结果，不应直接报警为数据库损坏。
- `40P01` 常可重试，但还要修复锁顺序和长事务。
- unique violation 通常是业务冲突或幂等命中；只有能证明它来自可重试分配竞争时才按重试处理。
- 重试必须有截止时间、次数上限、指标，并避免在事务内产生邮件、支付或链上广播等不可回滚副作用。

## 生产排查

| 问题 | 证据 | 动作 |
|------|------|------|
| 锁等待 | `pg_stat_activity`、`pg_locks`、blocking PID | 找阻塞链与事务入口，不先粗暴提高 timeout |
| 死锁 | 数据库 deadlock 日志、SQLSTATE、各路径锁序 | 统一资源顺序，缩短事务，减少无关工作 |
| `40001` 激增 | 事务类型、读写集合、重试次数 | 缩小争用域、分片热点、审查隔离级别 |
| 连接长期 `idle in transaction` | `xact_start`、应用 trace | 修复遗漏 commit/rollback，设置合理超时 |
| 余额不一致 | ledger legs、幂等键、对账差异 | 冻结受影响路径，以流水重建，不手改余额掩盖 |

锁超时、语句超时和事务超时是故障边界，不是正确性方案。超时后客户端还要判断事务是否已经提交；
连接中断也不能简单等价为“数据库没有执行”。

## 架构取舍

- **原子条件更新**：单行余额/库存优先；简单且冲突域清楚。
- **显式行锁**：需要读取多列后作复杂决策时可用，但要稳定锁序并控制事务长度。
- **Serializable**：跨行谓词和复杂不变量值得使用时很强，但必须接受 abort/retry 成本。
- **单写者/账户分片**：高热点资金系统可在数据库外先按账户串行化，数据库约束仍作最后防线。
- **不可变账本 + 派生余额**：审计和重建能力更强；代价是写路径、对账与修复流程更复杂。

## 深挖问答

1. **Repeatable Read 是否等于 Serializable？**  
   不等于。PostgreSQL Repeatable Read 提供稳定快照，但仍可能出现无法对应串行执行的依赖；
   Serializable 才会检测并中止危险模式。
2. **Serializable 为什么还会失败？**  
   它通过 abort 保证结果可串行化；失败与重试正是协议的一部分。
3. **`SELECT FOR UPDATE` 能防 phantom 吗？**  
   它锁住返回的行，不自动锁住“尚不存在但符合谓词的未来行”；需唯一约束、可串行化或重构不变量。
4. **`SKIP LOCKED` 为什么不能用于账本查询？**  
   它故意返回不一致视图以换取队列吞吐，会漏掉正在被锁的行。
5. **发生 commit timeout 能直接重试吗？**  
   不能。提交结果可能 unknown；先用业务幂等键查询事实，再决定重放同一意图。

## 反模式与错误表达

- “MVCC 下读写没有锁冲突。”
- “Read Committed 的一条 DML 永远只处理命令开始快照里的原版本。”
- “Repeatable Read 就一定没有 write skew。”
- “Serializable 太慢，而且不会失败。”
- “死锁只要无限重试就行。”
- “`SKIP LOCKED` 能让所有并发查询更快。”
- “用了数据库事务，就能把 Kafka、Webhook 和链上广播一起原子提交。”

## 延伸阅读

- [Transaction Isolation](https://www.postgresql.org/docs/current/transaction-iso.html)
- [Explicit Locking](https://www.postgresql.org/docs/current/explicit-locking.html)
- [Serialization Failure Handling](https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html)
