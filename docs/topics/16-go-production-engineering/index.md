# 16 Go 生产工程

6 篇 | 各岗位共享 P0 门槛 | [返回专题索引](../../topic-catalog.md) · [角色优先级](../_meta/role-priority-matrix.md)

> 这一模块不重复 GMP/GC，而是补齐资深 Go 岗更容易暴露短板的 **错误契约、包设计、测试、构建与供应链**。

| ID | 标题 | 频率 |
|----|------|------|
| [S-GOENG-01](./S-GOENG-01-errors-contract-panic-boundary.md) | 错误契约、Wrapping 与 Panic 边界 | ⭐⭐⭐⭐⭐ |
| [S-GOENG-02](./S-GOENG-02-package-interface-di.md) | 包边界、接口设计与依赖注入 | ⭐⭐⭐⭐⭐ |
| [S-GOENG-03](./S-GOENG-03-testing-table-fake.md) | Go 单元测试：表驱动、子测试与 Test Double | ⭐⭐⭐⭐⭐ |
| [S-GOENG-04](./S-GOENG-04-fuzz-benchmark-race.md) | Fuzz、Benchmark、Race 与回归门禁 | ⭐⭐⭐⭐⭐ |
| [S-GOENG-05](./S-GOENG-05-modules-toolchain-reproducible.md) | Go Modules、Workspace、Toolchain 与可复现构建 | ⭐⭐⭐⭐⭐ |
| [S-GOENG-06](./S-GOENG-06-static-analysis-supply-chain.md) | 静态分析、govulncheck 与依赖供应链 | ⭐⭐⭐⭐ |

## 推荐顺序

错误契约 → 包边界与 DI → 单元测试 → Fuzz/Race/Benchmark → Modules/Toolchain → CI 与供应链。

## 交叉题目

- [S-CODE-06 Singleflight](../08-coding-senior/S-CODE-06-singleflight-cache.md)
- [S-CODE-07 有界批处理执行器](../08-coding-senior/S-CODE-07-bounded-batch-executor.md)
- [S-NET-06 Linux FD、epoll 与 netpoll](../06-network-governance/S-NET-06-linux-fd-epoll-netpoll.md)
- [S-DB-07 资金类表设计](../middleware/mysql/S-DB-07-financial-schema-locking.md)
