---
id: S-MSVC-02
title: 交易域服务拆分与限界上下文
module: microservices-exchange
level: architect
frequency: 5
go_version: "1.22+"
tags: [ddd, bounded-context, microservices, cex, dex, decomposition]
status: published
resume_focus: true
code_refs: []
sources:
  - https://martinfowler.com/bliki/BoundedContext.html
  - https://www.domainlanguage.com/ddd/
---

# 交易域服务拆分与限界上下文

## 30 秒版（开场）

> 交易所拆微服务应 **按限界上下文** 而非按表或按团队人头。CEX 核心域：**订单、撮合、账务、钱包、风控、行情**；DEX 核心域：**索引、行情读模型、Launchpad、返佣**。上下文映射关系决定 **gRPC 同步 vs MQ 异步**。关键词：**聚合根不跨库事务、防腐层 ACL**。

## 3 分钟版（一面深度）

1. **是什么**：用 DDD 战略设计划分交易所服务边界与集成模式。
2. **为什么**：拆错边界会导致分布式事务地狱、对账对不上、发布耦合。
3. **怎么做**：每个上下文独立数据库；跨上下文用 ID 引用 + 事件；禁止跨库 JOIN。

## 10 分钟版

### 限界上下文地图（CEX + DEX）

```mermaid
flowchart LR
  subgraph Trading[交易上下文]
    OMS[订单 OMS]
    ME[撮合 ME]
  end
  subgraph Accounting[账务上下文]
    Ledger[复式记账]
  end
  subgraph Treasury[资金上下文]
    Wallet[充提/热钱包]
  end
  subgraph Market[行情上下文]
    MD[Market Data]
  end
  subgraph Chain[链上索引上下文]
    Idx[Indexer]
    Kline[K 线读模型]
  end
  subgraph Growth[增长上下文]
    Launch[Token 发行]
    Rebate[返佣]
  end
  OMS -->|OHS 开放主机| ME
  ME -->|发布语言 TradeEvent| Ledger
  ME -->|发布语言| MD
  Wallet -->|客户-供应商| Ledger
  Idx -->|发布语言 ChainEvent| Kline
  Idx -->|发布语言| Rebate
  Launch -->|ACL| Idx
```

### 上下文集成模式

| 关系 | 示例 | 集成方式 |
|------|------|----------|
| 发布-订阅 | ME → Ledger | Kafka `trade.matched` |
| 开放主机服务 OHS | OMS → ME | gRPC `SubmitOrder` |
| 防腐层 ACL | Launch → 链上 Factory | abigen + 版本适配 |
| 共享内核 | **避免** | 共用 user 表是大坑 |

### 服务清单与职责（面试白板表）

| 服务 | 所属上下文 | 核心聚合 | 禁止做的事 |
|------|------------|----------|------------|
| order-svc | 交易 | Order | 直接改余额 |
| matching-svc | 交易 | OrderBook, Trade | 写账务流水 |
| ledger-svc | 账务 | JournalEntry | 发起链上转账 |
| wallet-svc | 资金 | Deposit, Withdraw | 撮合订单 |
| market-svc | 行情 | Ticker, Depth | 资金结算 |
| indexer-svc | 链上 | ChainCursor, RawLog | 算 K 线 OHLC |
| kline-svc | 行情读模型 | KlineBar | 扫链 |
| risk-svc | 风控 | Rule, Blacklist | 持久化成交 |

### 何时不该再拆

| 信号 | 建议 |
|------|------|
| 团队 < 15 人 | 模块化单体 + 2～3 关键服务独立 |
| 强一致下单+扣款在同一请求 | 勿拆成跨服务 2PC |
| 链上索引 < 3 人维护 | indexer + kline 可同部署，逻辑分包 |

参见 [S-ARCH-14](../03-system-design/S-ARCH-14-microservice-boundary.md)

### CEX vs DEX 拆分差异

| 维度 | CEX | DEX |
|------|-----|-----|
| 真相源 | 撮合 WAL + 账务流水 | 链上日志 + Indexer 游标 |
| 延迟敏感 | matching-svc | indexer-svc（块延迟） |
| 扩展维度 | per symbol | per chain_id |

## 生产场景

- **返佣要读成交**：rebate-svc 消费 `TradeEvent` 或 `SwapEvent`，**不调** ledger 内部接口
- **钱包入账**：wallet-svc 发 `DepositConfirmed` → ledger-svc 消费入账
- **合约升级**：Launch 上下文通过 ACL 适配新旧 ABI

## 追问链

1. **订单和撮合能否合并？** → 小所可以；量大后撮合独立扩缩。
2. **行情能否合并进撮合？** → 读多写少，合并增加耦合；建议 MQ 解耦。
3. **与 S-SOL-01 关系？** → SOL-01 讲 DDD 通用；本题 **交易所域实例化**。

## 反模式

- 按 `users` / `orders` 表拆服务
- ledger 暴露 `UPDATE balance` 给所有服务
- indexer 直接写返佣结算表（跨上下文事务）

## 延伸阅读

- [S-SOL-01 限界上下文](../11-solution-architecture/S-SOL-01-bounded-context-ddd.md)
- [S-ARCH-14 微服务边界](../03-system-design/S-ARCH-14-microservice-boundary.md)
- [S-EXCH-03 账务](../14-dex-cex-engineering/S-EXCH-03-account-ledger.md)
