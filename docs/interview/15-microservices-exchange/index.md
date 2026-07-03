# 15 微服务（交易所场景）

6 题 | **CEX / DEX 背景的微服务架构** | [返回索引](../../interview-catalog.md) · [重点准备题单](../../resume-focus-web3.md)

> 以 **中心化交易（CEX）** 与 **链上交易（DEX/Web3）** 为业务背景，讲清微服务拆分、通信、数据、网关与事件边界。与 [14 DEX/CEX](../14-dex-cex-engineering/index.md) 业务专题互补：14 讲 **领域逻辑**，15 讲 **服务治理与架构落地**。

## 模块地图

| ID | 题目 | 频率 |
|----|------|------|
| [S-MSVC-01](./S-MSVC-01-exchange-microservices-whiteboard.md) | **交易所微服务全链路白板（CEX+DEX）** | ⭐⭐⭐⭐⭐ |
| [S-MSVC-02](./S-MSVC-02-domain-decomposition.md) | 交易域服务拆分与限界上下文 | ⭐⭐⭐⭐⭐ |
| [S-MSVC-03](./S-MSVC-03-discovery-grpc-governance.md) | 服务发现与 gRPC 通信治理 | ⭐⭐⭐⭐⭐ |
| [S-MSVC-04](./S-MSVC-04-database-per-service.md) | Database per Service 与跨服务一致性 | ⭐⭐⭐⭐⭐ |
| [S-MSVC-05](./S-MSVC-05-gateway-bff-traffic.md) | API 网关、BFF 与交易流量治理 | ⭐⭐⭐⭐⭐ |
| [S-MSVC-06](./S-MSVC-06-event-bus-async-boundary.md) | 事件总线与异步服务边界 | ⭐⭐⭐⭐ |

## 与现有模块关系

| 模块 | 分工 |
|------|------|
| [14 DEX/CEX](../14-dex-cex-engineering/index.md) | 撮合、账务、充提、K 线、永续等业务 |
| **15 本模块** | 上述能力如何 **拆服务、怎么通信、数据怎么隔离** |
| [06 网络](../06-network-governance/index.md) | gRPC/JWT/WS 通用考点 |
| [11 解决方案架构](../11-solution-architecture/index.md) | DDD、绞杀者、Mesh 通用架构师题 |
| [03 系统设计](../03-system-design/index.md) | 限流、熔断、MQ 语义、幂等 |

## 推荐刷题顺序

**CEX 微服务岗**：MSVC-01 → MSVC-02 → MSVC-03 → MSVC-04 → 下钻 [EXCH-13](../14-dex-cex-engineering/S-EXCH-13-cex-end-to-end-architecture.md)

**Web3/DEX 微服务岗**：MSVC-01 → MSVC-02 → MSVC-06 → MSVC-04 → 下钻 [EXCH-14](../14-dex-cex-engineering/S-EXCH-14-web3-exchange-fullstack-architecture.md)

**平台/治理岗**：MSVC-03 → MSVC-05 → MSVC-06 → [S-SOL-04](../11-solution-architecture/S-SOL-04-bff-gateway-mesh.md)

## 自测标准

- [ ] 能在白板上画出 **CEX 与 DEX 两条业务链** 对应的微服务边界
- [ ] 能说清 **哪些必须同步 gRPC、哪些必须异步 MQ**
- [ ] 能解释 **账务库与订单库分离** 后如何保证最终一致
- [ ] 能设计 **交易 API 网关** 的限流维度（用户/IP/symbol/接口）
