# 14 DEX / CEX 交易所工程

12 题 | P1 扩展（**交易所 / 做市 / 合约后端** JD） | [返回索引](../../interview-catalog.md) · [重点准备题单](../../resume-focus-web3.md)

> 面向 **CEX 撮合与资金系统（Go）** + **DEX 链上协议（Solidity）** 工程师，及 **交易所架构师** 全栈面试。

## CEX（中心化，Go 后端为主）

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-01](./S-EXCH-01-cex-matching-engine.md) | CEX 撮合引擎与订单簿架构 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-02](./S-EXCH-02-deposit-withdraw-wallet.md) | 充值、提现与链上钱包体系 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-03](./S-EXCH-03-account-ledger.md) | 账户体系与资金账务（复式记账） | ⭐⭐⭐⭐⭐ |
| [S-EXCH-04](./S-EXCH-04-futures-margin-liquidation.md) | 合约：保证金、强平、资金费率 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-05](./S-EXCH-05-risk-reconciliation.md) | 风控、对账与合规审计 | ⭐⭐⭐⭐⭐ |

## DEX（去中心化，合约 + 索引）

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-06](./S-EXCH-06-dex-amm-liquidity.md) | DEX AMM 与流动性池设计 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-07](./S-EXCH-07-aggregator-slippage.md) | DEX 聚合路由与滑点保护 | ⭐⭐⭐⭐ |
| [S-EXCH-08](./S-EXCH-08-mev-sandwich.md) | MEV、抢跑与链上交易防护 | ⭐⭐⭐⭐⭐ |

## 混合架构

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-09](./S-EXCH-09-hybrid-cex-dex.md) | CEX/DEX 混合与流动性整合 | ⭐⭐⭐⭐ |

## 行情与链上数据专题

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-10](./S-EXCH-10-kline-event-aggregation.md) | 链上事件驱动 K 线与行情聚合 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-11](./S-EXCH-11-websocket-market-hub.md) | WebSocket 行情 Hub 与连接治理 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-12](./S-EXCH-12-token-launch-rebate.md) | Token 发行、分账与返佣提现 | ⭐⭐⭐⭐⭐ |

## 关联模块

| 已有题目 | 关系 |
|----------|------|
| [S-SOLID-07 DeFi 模式](../13-solidity-contracts/S-SOLID-07-defi-patterns.md) | AMM/Oracle 原理 |
| [S-BC-05 索引器](../12-blockchain-web3/S-BC-05-indexer-reorg.md) | DEX 成交/充值监听 |
| [S-ARCH-04 幂等](../03-system-design/S-ARCH-04-idempotency.md) | 充提、撮合幂等 |
| [S-ARCH-08 限流](../03-system-design/S-ARCH-08-rate-limiting.md) | 交易 API 防刷 |

## 推荐刷题顺序

CEX 撮合 → 账务 → 充提 → 合约 → 风控 → DEX AMM → 聚合/MEV → 混合架构

## 岗位自测

- **CEX 后端**：能讲清撮合、账务、充提、强平四条链路
- **DEX 协议**：能画 AMM、讲清 LP 风险与 Oracle
- **架构师**：能对比 CEX/DEX 信任模型与一致性边界（[S-SOLID-08](../13-solidity-contracts/S-SOLID-08-contract-go-boundary.md)）
