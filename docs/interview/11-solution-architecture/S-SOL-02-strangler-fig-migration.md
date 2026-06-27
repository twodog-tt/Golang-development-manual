---
id: S-SOL-02
title: 绞杀者模式与遗留系统迁移
module: solution-architecture
level: architect
frequency: 5
go_version: "1.22+"
tags: [strangler-fig, migration, legacy, dual-write, architect]
status: published
code_refs: []
sources:
  - https://martinfowler.com/bliki/StranglerFigApplication.html
  - https://learn.microsoft.com/en-us/azure/architecture/patterns/strangler-fig
---

# 绞杀者模式与遗留系统迁移

## 30 秒版（开场）

> **绞杀者（Strangler Fig）**：新系统像藤蔓一样 **逐路由、逐能力** 替换旧系统，而不是 Big Bang 重写。架构师要讲清 **流量切换、双写/对账、回滚开关**。生产关键词：**Facade 路由、特性开关、影子流量**。

## 3 分钟版（一面深度）

1. **是什么**：在旧系统外围设 **Routing Facade**（网关/BFF），按 URL/用户/百分比把流量导向新旧实现。
2. **为什么**：架构师面试几乎必问「你怎么迁移 PHP/Java 老单体到 Go 微服务」；大爆炸风险不可接受。
3. **怎么做**：按 **垂直切片** 迁移（如先「查询订单」再「创建订单」）；每步可独立回滚；数据层双写 + 对账 job。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart TB
  Client[客户端] --> GW[Routing Facade / API Gateway]
  GW -->|90%| Legacy[遗留单体]
  GW -->|10% 灰度| NewGo[Go 新服务]
  NewGo --> NewDB[(新库)]
  Legacy --> OldDB[(旧库)]
  NewGo -.->|双写/CDC| OldDB
  Reconcile[对账 Job] --> NewDB
  Reconcile --> OldDB
```

**迁移阶段模板（面试叙事用）**

| 阶段 | 动作 | 回滚 |
|------|------|------|
| 0 | 只读 API 读新写旧 | 关开关回 100% 旧 |
| 1 | 双写 + 异步对账 | 停双写，读旧 |
| 2 | 读新写新，旧库只读 | 写回旧（需脚本） |
| 3 | 下线旧模块 | 保留只读备份 |

**Go 侧常见落地**

- 网关：Nginx/APISIX 按 path 路由；或 Go BFF 内 `if feature.NewOrder()` 转发
- 双写：应用层写两个 repo；或 Debezium/Canal **CDC** 单向同步
- 对账：定时任务比对新旧 `order_id` 数量与 checksum

## 生产场景

- **支付链路迁移**：先迁查询，最后迁扣款；支付 never 允许 silent failure
- **Session 粘滞**：旧系统有 session，迁移期 JWT 统一或 SSO 桥接
- **与 S-ARCH-19 关系**：19 讲单体→微服务概念；本题讲 **怎么安全落地**

## 排查与工具

- 特性开关：LaunchDarkly / 自研 config center
- 指标：新旧路径 **错误率、P99、业务成功率** 对比
- 影子流量：新系统接生产流量副本不写库

## 架构取舍

| 双写 | CDC |
|------|-----|
| 实现快 | 解耦应用 |
| 应用复杂 | 延迟、顺序需处理 |

**何时不用绞杀者**：系统极小、或旧系统可整体停机窗口重写（罕见）。

## 追问链

1. **双写不一致怎么办？** → 以旧为准 + 告警 + 人工/自动修复；定义 **source of truth**。
2. **如何选第一个切片？** → 边界清晰、流量可灰度、非资金核心路径优先。
3. **组织阻力？** → 联合 KPI、小步快跑、可视化迁移看板（架构师软技能）。
4. **Go 与旧 Java 共存事务？** → 避免跨库 2PC；Saga + 幂等（见 [S-DIST-05](../middleware/distributed/S-DIST-05-distributed-transaction.md)）。

## 反模式与事故

- **Big Bang 切换** → 大促前全量切，回滚不及
- **无对账** → 双写静默丢单
- **按层迁移**（先迁所有 DAO）→ 长期两套并行，成本失控

## 代码示例

```go
func (h *OrderHandler) Create(c *gin.Context) {
    if h.flags.UseNewOrderService(c.Request.Context(), userID) {
        h.newSvc.Create(c)
        return
    }
    h.legacyProxy.Create(c)
}
```

## 延伸阅读

- [Strangler Fig - Martin Fowler](https://martinfowler.com/bliki/StranglerFigApplication.html)
- [Azure Strangler Fig pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/strangler-fig)
