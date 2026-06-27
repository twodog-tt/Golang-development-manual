# 11 解决方案架构（架构师岗）

8 题 | **P2+ 架构师 / Staff / 专家** | [返回索引](../README.md)

> 面向 **冲架构师岗** 的候选人：在 [03 系统设计](../03-system-design/)（单题 15min）之上，考察 **领域建模、演进治理、全局非功能、45min 白板叙事**。

| ID | 题目 | 频率 |
|----|------|------|
| [S-SOL-01](./S-SOL-01-bounded-context-ddd.md) | 限界上下文与 DDD 战略设计 | ⭐⭐⭐⭐⭐ |
| [S-SOL-02](./S-SOL-02-strangler-fig-migration.md) | 绞杀者模式与遗留系统迁移 | ⭐⭐⭐⭐⭐ |
| [S-SOL-03](./S-SOL-03-event-driven-cqrs.md) | 事件驱动、CQRS 与一致性边界 | ⭐⭐⭐⭐⭐ |
| [S-SOL-04](./S-SOL-04-bff-gateway-mesh.md) | BFF、API 网关与服务网格职责 | ⭐⭐⭐⭐ |
| [S-SOL-05](./S-SOL-05-multi-tenant-saas.md) | 多租户 SaaS 隔离与权限架构 | ⭐⭐⭐⭐⭐ |
| [S-SOL-06](./S-SOL-06-architecture-review.md) | 架构评审：流程、产出与博弈 | ⭐⭐⭐⭐⭐ |
| [S-SOL-07](./S-SOL-07-security-audit-architecture.md) | 安全与审计的全局架构 | ⭐⭐⭐⭐ |
| [S-SOL-08](./S-SOL-08-evolution-whiteboard.md) | 45 分钟架构演进白板模板 | ⭐⭐⭐⭐⭐ |

## 与现有模块的关系

| 模块 | 架构师面分工 |
|------|--------------|
| [03 系统设计](../03-system-design/) | 单点场景：秒杀、缓存、MQ、多活 |
| **11 本模块** | 跨系统：域划分、迁移、治理、租户、安全 |
| [07 领导力](../07-engineering-leadership/) | 人、流程、复盘、技术债 |
| [S-ARCH-20 ADR](../03-system-design/S-ARCH-20-tech-decision-doc.md) | 单次选型；本模块 S-SOL-06 讲评审机制 |

## 推荐刷题顺序（架构师 4 周）

| 周 | 内容 |
|----|------|
| W1 | S-SOL-01 域建模 + S-ARCH-14 微服务边界 + 03 系统设计 10 题 |
| W2 | S-SOL-02/03 演进与事件 + middleware 分布式事务 |
| W3 | S-SOL-04/05/07 平台与非功能 + 云原生/网络 |
| W4 | S-SOL-06/08 评审与白板 + 07 领导力 + 2 场 45min 模拟 |

## 自测标准（架构师岗）

- [ ] 能在白板上画 **上下文映射图**（至少 4 个域 + 集成关系）
- [ ] 能讲一个 **真实迁移/拆分** 项目：阶段、回滚、指标
- [ ] 45 分钟内完成 **需求澄清 → 估算 → 架构 → 风险 → 演进**
- [ ] 能处理评审中的 **反对意见**（成本、工期、风险）
