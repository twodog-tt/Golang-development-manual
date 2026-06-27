---
id: S-DB-05
title: GORM 预加载 N+1 与事务陷阱
module: database-storage
level: senior
frequency: 4
go_version: "1.22+"
tags: [gorm, n+1, preload, transaction, orm]
status: published
code_refs:
  - gorm/demo/main.go
sources:
  - https://gorm.io/docs/preload.html
  - https://gorm.io/docs/transactions.html
  - https://gorm.io/docs/hooks.html
---

# GORM 预加载 N+1 与事务陷阱

## 30 秒版（开场）

> **N+1**：查 N 条主记录后对每条再查关联，SQL 爆炸。GORM 用 **`Preload`/`Joins`** 预加载；`Find` 嵌套 struct 不会自动加载。**事务陷阱**：`Transaction` 回调里须用 `tx`；钩子 `AfterCreate` 里默认 DB 可能不在同一事务。生产关键词：**Session、Clauses、连接池、软删 Scope**。

## 3 分钟版（一面深度）

1. **是什么**：ORM 便利但易隐藏查询；N+1 是循环访问关联；事务中混用全局 `DB` 与 `tx` 导致部分提交或锁失效。
2. **为什么**：GORM 懒加载关联需显式 Preload；`Logger` 开 Info 才看见多条 SELECT；钩子用 `tx *gorm.DB` 参数才对。
3. **怎么做**：列表用 `Preload("Comments")` 或 `Joins`+`Select`；批量用 `Preload(clause.Associations)` 谨慎；写操作用 `db.Transaction(func(tx *gorm.DB) error { ... })`；调试开 `DryRun`/`Debug()`。

## 10 分钟版（原理 + 图示）

**N+1 对比**

| 写法 | SQL 数 | 说明 |
|------|--------|------|
| `Find(&posts)` 后循环 `post.Comments` | 1+N | 典型 N+1 |
| `Preload("Comments").Find(&posts)` | 2 | IN 查子表 |
| `Joins("Comments").Find(&posts)` | 1 | 行膨胀 duplicate |
| `Preload("Comments", func(db *gorm.DB) *gorm.DB { return db.Limit(5) })` | 2+ | 条件预加载 |

```mermaid
flowchart LR
  Bad[Find 100 posts] --> Loop[每 post 查 Comments]
  Loop --> N1[1 + 100 SQL]
  Good[Preload Comments] --> Two[2 SQL: posts + IN comments]
```

**事务陷阱**：`db.Create` 在 Transaction 外；钩子 `AfterCreate` 用 `DB` 全局实例——与外层 tx 脱节；`SavePoint` 嵌套错误；`PrepareStmt` 与连接池；`SkipDefaultTransaction` 单条 Create 默认无事务但多条需显式。

**其他坑**：`Updates(map)` 不触发 `Hook` 且不更新零值；`Delete` 软删 vs `Unscoped`；`First` 无记录 `ErrRecordNotFound` 需区分；大 `IN` 列表分批。

## 生产场景

- **博客列表带评论作者**：`Preload("Comments.User")` 三次变两次 SQL（见 demo）。
- **转账**：`db.Transaction` 内两笔 `tx.Model().Update` + 行锁 `clause.Locking{Strength: "UPDATE"}`。
- **钩子更新计数**：`AfterCreate` 必须用 `tx` 更新 `posts_count`，否则主记录 rollback 计数已加。

## 排查与工具

| 工具 | 用途 |
|------|------|
| GORM Logger Info | 打印每条 SQL |
| `db.DryRun` | 只看 SQL 不执行 |
| APM（Datadog/Jaeger） | DB span 数量 |
| go-sqlmock | 单测断言查询次数 |

路径：接口 RT 随列表长度线性涨 → 开 SQL log 数条数 → 加 Preload → 压测对比。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| Preload | 一对多、多对多 | 极大关联集 |
| Joins | 需过滤关联字段 | 多对多重复行 |
| 原生 SQL | 复杂报表 | 简单 CRUD |
| 禁用钩子 | 性能敏感 | 依赖计数一致性 |
| sqlx | 轻量可控 | 快速迭代 |

## 追问链

1. **Preload 和 Joins 区别？** → Preload 分条 SQL 无重复行；Joins 单 SQL 可能 duplicate 需 Distinct。
2. **钩子事务？** → 回调参数 `tx` 与创建操作同一事务。
3. **如何避免 N+1 无 Preload？** → 手动 `WHERE post_id IN (?)` 一次查评论再组装 map。
4. **GORM 连接池？** → 底层 `*sql.DB` 设 `SetMaxOpenConns` 等。
5. **软删影响 Preload？** → 默认带 `deleted_at IS NULL`；`Unscoped` 可查已删。

## 反模式与事故

- 列表 100 条未 Preload——101 次查询打满连接池。
- Transaction 内用 `DB.Create` 非 `tx.Create`——半成功脏数据。
- `Updates(struct)` 想更新零值字段——应用 `map` 或 `Select`。
- 钩子里再调 HTTP——事务长时间持锁。

## 代码示例

```go
// 正确：Preload 避免 N+1（摘自 gorm/demo）
func getUserPostsWithComments(userID uint) ([]Post, error) {
    var posts []Post
    err := DB.Where("user_id = ?", userID).
        Preload("Comments").
        Preload("Comments.User").
        Find(&posts).Error
    return posts, err
}

// 正确：事务 + 钩子使用 tx
func transfer(db *gorm.DB, from, to uint, amount int64) error {
    return db.Transaction(func(tx *gorm.DB) error {
        if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
            First(&Account{}, from).Error; err != nil {
            return err
        }
        // tx.Model(...).Update(...)
        return nil
    })
}
```

可运行示例见 [`gorm/demo/main.go`](../../../gorm/demo/main.go)（关联查询、钩子与 Preload SQL 日志）。

## 延伸阅读

- [GORM Preload](https://gorm.io/docs/preload.html)
- [GORM Transactions](https://gorm.io/docs/transactions.html)
- [GORM Hooks](https://gorm.io/docs/hooks.html)
