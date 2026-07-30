---
id: S-DB-06
title: 复杂 SQL：JOIN、CTE、窗口函数与执行计划
module: database-storage
level: senior
frequency: 5
go_version: "1.22+"
tags: [mysql, sql, join, cte, window-function, explain]
status: published
code_refs: []
sources:
  - https://dev.mysql.com/doc/refman/8.4/en/join.html
  - https://dev.mysql.com/doc/refman/8.4/en/with.html
  - https://dev.mysql.com/doc/refman/8.4/en/window-functions.html
  - https://dev.mysql.com/doc/refman/8.4/en/explain.html
---

# 复杂 SQL：JOIN、CTE、窗口函数与执行计划

<a id="oral-card"></a>

## 要点卡

[返回 P0 知识图谱](../../_meta/p0-knowledge-graph.md)

!!! abstract "30 秒回答"

    复杂 SQL 先保证集合语义正确，再谈“少写一层”。JOIN 要明确基数和去重，否则一对多叠加会把金额重复；CTE 是命名和组织查询的工具，不保证一定物化或更快；窗口函数在不压缩行数的前提下计算排名、累计值等。优化必须看 `EXPLAIN ANALYZE` 的实际 rows、loops、排序和临时表，不能只凭 SQL 长短。

**3 分钟展开**

1. **JOIN 基数**：先写每张表的主键/唯一键和预期 1:1、1:N、N:M。
2. **聚合位置**：一对多表先按业务键聚合再 join，避免笛卡尔放大。
3. **CTE**：提升可读性、支持递归；优化器可能 merge 或 materialize，需看执行计划。
4. **窗口函数**：`PARTITION BY` 定分组，`ORDER BY` 定顺序，frame 决定参与计算的行。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 先证明集合语义和基数；CTE 不保证物化或加速；窗口函数不折叠行；优化看实际计划和结果对账 |
| 手画图 | `keys/cardinality → JOIN/aggregate/window → EXPLAIN ANALYZE → index/rewrite → reconciliation` |
| 项目落点 | 用实际账务对账、返佣或交易流水说明一对多 join 如何导致金额重复；只引用本人实际参与部分和可解释指标 |
| 一个取舍 | 单条 SQL 保持快照与集合优化机会；拆到应用更易读但增加往返和一致性边界 |

**错误表达**

- ❌ “SQL 行数越少越快；CTE 一定物化；JOIN 多出重复行后再 DISTINCT 就正确。”
- ✅ “性能由计划和数据分布决定；DISTINCT 可能掩盖错误基数，必须从业务键证明结果。”

**自测追问**：一对多表为什么应先聚合再 join？窗口 frame 与 `PARTITION BY` 分别控制什么？

## 10 分钟版（原理 + 图示）

```mermaid
flowchart LR
  Keys["主键 / 唯一键 / 基数"] --> Semantics["验证集合语义"]
  Semantics --> Query["JOIN / CTE / Window"]
  Query --> Plan["EXPLAIN ANALYZE"]
  Plan --> Index["索引与改写"]
  Index --> Verify["结果对账 + 性能回归"]
```

**每个账户最新一笔交易**

```sql
WITH ranked AS (
  SELECT
    account_id,
    tx_id,
    amount,
    created_at,
    ROW_NUMBER() OVER (
      PARTITION BY account_id
      ORDER BY created_at DESC, tx_id DESC
    ) AS rn
  FROM ledger_transactions
)
SELECT account_id, tx_id, amount, created_at
FROM ranked
WHERE rn = 1;
```

排序必须有稳定 tie-breaker；只按 `created_at`，同一时间戳的结果可能不确定。

**运行余额**

```sql
SELECT
  account_id,
  entry_id,
  amount,
  SUM(amount) OVER (
    PARTITION BY account_id
    ORDER BY created_at, entry_id
    ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
  ) AS running_balance
FROM ledger_entries;
```

显式写 `ROWS`，避免默认 frame 在重复排序值下产生意外 peer 行语义。

**JOIN 放大例子**

一个 order 有 3 条 payment attempt、2 条 refund，直接同时 left join 后可能得到 6 行。若再 `SUM(order.amount)` 就重复计数。应分别在 CTE/derived table 中聚合 payment 和 refund，再按 `order_id` join。

## 生产场景

- 对账：按日/资产/链聚合账本、链上 observation 与 provider statement，再 full-diff。
- 排行榜：窗口函数适合离线/后台查询；高 QPS 在线榜单可能需要预计算。
- 层级组织：递归 CTE 可查树，但要限制深度并防循环数据。

## 排查与工具

```sql
EXPLAIN FORMAT=TREE
SELECT ...;

EXPLAIN ANALYZE
SELECT ...;
```

`EXPLAIN ANALYZE` 会实际执行查询；对写语句或重查询要在安全环境谨慎使用。重点看估算与实际偏差、每个 iterator loops、扫描行、排序/临时表和总耗时。

## 架构取舍

把所有逻辑塞入一条 SQL 可能减少往返，但也可能难审计、锁更久、优化器选择复杂。资金查询优先正确和可解释；必要时用临时结果/离线数仓，而不是为了“单 SQL”牺牲可验证性。

## 深挖问答

1. **CTE 一定物化吗？** → 不一定；MySQL 优化器可 merge 或 materialize，依查询和版本计划而定。
2. **窗口函数和 GROUP BY？** → GROUP BY 压缩为每组一行；窗口函数通常保留输入行。
3. **LEFT JOIN 条件放 WHERE 会怎样？** → 对右表列的过滤可能把无匹配 NULL 行去掉，语义退化成 inner join；需要判断应放 `ON` 还是 `WHERE`。
4. **为什么排名要第二排序键？** → 保证重复时间/分数时结果确定。
5. **复杂 SQL 怎么测？** → 小型反例数据验证基数，再用生产分布规模压测和结果对账。

## 反模式与事故

- 先 join 两个一对多表再聚合，金额翻倍。
- 认为 CTE 天生更快或一定只执行一次。
- `SELECT *` 让覆盖索引失效并增加网络/解码。
- 只看 EXPLAIN 估算，不看真实数据倾斜和 loops。

## 延伸阅读

- [JOIN Clause](https://dev.mysql.com/doc/refman/8.4/en/join.html)
- [Common Table Expressions](https://dev.mysql.com/doc/refman/8.4/en/with.html)
- [Window Functions](https://dev.mysql.com/doc/refman/8.4/en/window-functions.html)
- [EXPLAIN](https://dev.mysql.com/doc/refman/8.4/en/explain.html)

