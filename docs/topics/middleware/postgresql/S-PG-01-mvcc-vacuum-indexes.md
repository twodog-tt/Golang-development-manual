---
id: S-PG-01
title: PostgreSQL MVCC、VACUUM、可见性与索引设计
module: postgresql
level: senior
frequency: 5
go_version: "1.24+"
tags: [postgresql, mvcc, vacuum, hot, visibility-map, btree, brin, index-only-scan]
status: published
resume_focus: true
code_refs: []
sources:
  - https://www.postgresql.org/docs/current/mvcc-intro.html
  - https://www.postgresql.org/docs/current/routine-vacuuming.html
  - https://www.postgresql.org/docs/current/storage-vm.html
  - https://www.postgresql.org/docs/current/storage-hot.html
  - https://www.postgresql.org/docs/current/indexes.html
  - https://www.postgresql.org/docs/current/indexes-multicolumn.html
  - https://www.postgresql.org/docs/current/indexes-index-only-scans.html
  - https://www.postgresql.org/docs/current/brin.html
---

# PostgreSQL MVCC、VACUUM、可见性与索引设计

## 30 秒版（开场）

> PostgreSQL 的 `UPDATE` 通常写出新 tuple version，旧版本等到不再被任何快照需要后，才由 `VACUUM` 标记为可复用；普通 `VACUUM` 通常不把文件空间归还操作系统，`VACUUM FULL` 会重写表并持有强锁。索引保存的是指向 heap tuple 的位置，index-only scan 还依赖 visibility map；因此“建了覆盖索引就一定不回表”是错的。排查慢表要把长事务、dead tuple、autovacuum、索引顺序、heap fetch 和查询工作负载放在一起看。

## 3 分钟版（精讲深度）

1. **MVCC 是可见性协议，不是“读写都不加锁”**：每条 tuple 带事务可见性信息；读通常不阻塞普通写，但 DDL、显式锁、唯一性检查和写写冲突仍会等待或失败。
2. **更新不是原地覆盖的通用模型**：新版本进入 heap，旧版本在没有活跃快照需要后成为 dead tuple。若未修改普通索引所引用的列（BRIN 等 summarizing index 是例外）且同一 heap page 有空间，HOT update 可避免新增普通索引项，但 HOT 不是业务可依赖的保证。
3. **VACUUM 与 ANALYZE 要分开说**：普通 `VACUUM` 回收可复用空间、推进 visibility
   map，并冻结旧事务 ID 以防 wraparound；planner statistics 由 `ANALYZE` 更新，可以单独运行，
   也可以通过 `VACUUM (ANALYZE)` 组合执行。autovacuum daemon 也会按各自阈值触发 vacuum
   和 analyze。长事务、`idle in transaction`、长期 replication slot 等可能阻止清理或保留
   WAL，但具体影响路径不同。
4. **索引按查询形状设计**：B-tree 适合等值/范围/排序；GIN 常用于多值与全文类操作；GiST/SP-GiST 面向特定空间或搜索算子；BRIN 适合与物理顺序高度相关的超大表。

## 10 分钟版（原理 + 图示）

### tuple 生命周期

```mermaid
flowchart LR
  A["INSERT / UPDATE<br/>写入新 tuple"] --> B["事务提交"]
  B --> C["旧 tuple 对某些快照仍可见"]
  C --> D["全局不再需要旧版本"]
  D --> E["VACUUM 标记空间可复用"]
  E --> F["后续写入复用页内空间"]
  E --> G["文件尾部恰好可截断时<br/>才可能缩小文件"]
```

这里最容易说错两点：

- dead tuple 不是事务一提交就能立刻物理删除；仍要尊重其他事务快照。
- 普通 `VACUUM` 主要让空间在表内复用。需要重写并归还更多磁盘时可能考虑
  `VACUUM FULL`、`CLUSTER` 或在线重组工具，但它们有锁、额外空间和变更风险。

### visibility map 为什么影响 index-only scan

PostgreSQL 的普通索引项不携带完整的 tuple 可见性。即使查询列都在索引里，执行器仍可能访问
heap 判断该版本对当前快照是否可见；只有对应 heap page 被 visibility map 标为
all-visible 时，才可跳过这次 heap fetch。

因此判断覆盖索引是否有效，要看：

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT account_id, amount
FROM ledger_entry
WHERE tenant_id = $1
  AND created_at >= $2
ORDER BY created_at DESC
LIMIT 100;
```

重点观察实际行数估计偏差、`Heap Fetches`、shared hit/read、排序和扫描范围。`ANALYZE`
会真正执行语句；对写语句应先放在可回滚事务或安全环境中。

### 多列、INCLUDE、部分索引与 BRIN

```sql
CREATE INDEX CONCURRENTLY idx_ledger_tenant_time
ON ledger_entry (tenant_id, created_at DESC)
INCLUDE (account_id, amount);

CREATE INDEX CONCURRENTLY idx_withdrawal_pending
ON withdrawal (tenant_id, created_at)
WHERE status = 'pending';
```

- 多列 B-tree 最稳定的高收益形态仍是：前导列承接高频等值条件，随后放范围/排序列。
- 不要把规则背成“没用最左列就绝对不能用”。PostgreSQL 18 为多列 B-tree 引入了
  skip scan，是否获益由数据分布、统计信息和成本估算决定；目标版本若是 17 或更早版本，
  不能把这一能力当成既有事实。
- `INCLUDE` 列是 payload，不参与搜索键语义；它可能提高 index-only scan 机会，也会放大
  索引、写放大和缓存压力。
- 部分索引只有在优化器能证明查询条件蕴含其 predicate 时才可使用；参数化写法和条件形态
  可能影响证明。
- BRIN 保存 page range 摘要，适合按时间/高度近似顺序写入的大表；它会返回候选 page range
  再复查，不是精确行定位的 B-tree 替代品。

`CREATE INDEX CONCURRENTLY` 降低对写入的阻塞，但耗时更久、资源消耗更大，失败后还可能留下
invalid index；它也不能放在普通事务块里。生产变更应检查 `pg_index.indisvalid` 和构建进度。

## 生产排查

| 现象 | 优先证据 | 常见根因 |
|------|----------|----------|
| 表/索引持续膨胀 | `pg_stat_user_tables`、对象大小、长事务 | 高频更新、清理水位被旧快照卡住、autovacuum 跟不上 |
| autovacuum 很忙 | `pg_stat_progress_vacuum`、I/O、dead tuple 趋势 | 表级阈值不合适、突发写入、索引过多 |
| index-only 仍慢 | `EXPLAIN (ANALYZE, BUFFERS)` 的 heap fetch | 页面频繁修改、visibility map 尚未推进 |
| 计划突然改变 | 统计信息、参数、数据倾斜、版本 | 估算偏差，不是“优化器随机失效” |
| WAL/磁盘暴涨 | replication slot、归档、checkpoint 与写入量 | slot 消费者停滞、批量更新、索引写放大 |

`n_dead_tup` 是估算值，不应单独作为精确事实。还要检查 `pg_stat_activity` 中长时间
`xact_start`、`state = 'idle in transaction'` 和 `backend_xmin`，以及 replication slot
的保留水位。

## 架构取舍

- **高频更新表**：少建“可能有用”的索引，优先让最重要查询命中；每个索引都会增加写入和
  vacuum 成本。
- **链高度/时间序列归档**：数据与物理顺序相关且查询多为大范围过滤时可评估 BRIN；
  高频点查仍常需要 B-tree。
- **分区表**：分区首先解决生命周期、维护和裁剪问题，不自动让所有查询更快；分区键必须
  出现在主要过滤条件中，并控制分区数量。
- **重整空间**：先证明空间是否真的需要归还 OS；不要把 `VACUUM FULL` 当日常保养命令。

## 深挖问答

1. **为什么 PostgreSQL 需要 VACUUM，而 InnoDB 的表述不同？**  
   两者 MVCC 物理组织和清理机制不同。回答 PostgreSQL 的 heap tuple、visibility 与 vacuum，
   不要套用 undo log 话术。
2. **HOT update 一定发生吗？**  
   不一定；要求未修改普通索引引用列（summarizing index 可特殊处理）且目标 page 有空间等条件，
   fillfactor 只是影响机会。
3. **覆盖索引为什么仍回 heap？**  
   索引没有完整可见性；page 未标 all-visible 时必须验证 heap tuple。
4. **为什么索引越多写入越慢？**  
   每次写要维护更多索引项，还增加 WAL、缓存占用和 vacuum/重建成本。
5. **怎样证明一个索引该删？**  
   结合代表性周期使用统计、查询计划、重复/前缀关系、写成本与应急查询需求；统计归零和
   failover 也会重置观察窗口，不能只看一次计数。

## 反模式与错误表达

- “MVCC 所以 PostgreSQL 查询永远不加锁。”
- “`VACUUM` 会把所有空间立刻还给操作系统。”
- “`VACUUM FULL` 更彻底，所以应该定期跑。”
- “普通 `VACUUM` 会自动刷新 planner statistics。”
- “多列索引不带第一列就绝对无效。”
- “有 `INCLUDE` 就一定是 index-only scan。”
- “`n_dead_tup` 就是精确的垃圾行数。”

## 延伸阅读

- [PostgreSQL Concurrency Control](https://www.postgresql.org/docs/current/mvcc.html)
- [Routine Vacuuming](https://www.postgresql.org/docs/current/routine-vacuuming.html)
- [Indexes](https://www.postgresql.org/docs/current/indexes.html)
- [BRIN Indexes](https://www.postgresql.org/docs/current/brin.html)
