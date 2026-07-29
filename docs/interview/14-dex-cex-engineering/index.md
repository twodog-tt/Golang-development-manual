# 14 DEX / CEX 交易所工程

28 题 | P0/P1 专题（**预测市场 / 交易所 / 做市 / 合约后端 / 架构师** JD） | [返回索引](../../interview-catalog.md) · [重点专题](../../web3-exchange-wallet-focus.md)

> 面向 **CEX 撮合与资金系统（Go）** + **DEX/预测市场链上协议（Solidity）** 工程师，
> 及 **交易所/预测市场技术负责人** 全栈面试。

## 完整架构白板（架构师必练）

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-13](./S-EXCH-13-cex-end-to-end-architecture.md) | **CEX 端到端交易系统架构（45min）** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-14](./S-EXCH-14-web3-exchange-fullstack-architecture.md) | **Web3 交易所全栈（链上+链下）** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-15](./S-EXCH-15-settlement-ha-disaster-recovery.md) | **清结算、对账与高可用** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-16](./S-EXCH-16-perpetual-matching-position.md) | **永续合约撮合与仓位引擎** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-17](./S-EXCH-17-runnable-deterministic-matching-engine.md) | **Go 可运行确定性撮合引擎** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-18](./S-EXCH-18-wal-snapshot-replay.md) | **撮合 WAL、快照与回放** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-19](./S-EXCH-19-market-data-sequence-gap-recovery.md) | **行情序号与 Gap Recovery** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-20](./S-EXCH-20-fix-session-sequence-recovery.md) | **FIX Session 与断线恢复** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-21](./S-EXCH-21-self-trade-prevention-surveillance.md) | **STP 与监控合规边界** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-22](./S-EXCH-22-call-auction-performance-validation.md) | **集合竞价与性能验证** | ⭐⭐⭐⭐⭐ |

> 架构白板 + **永续撮合**（EXCH-16）与专题题（01/04）配合使用。

## 预测市场（CLOB-first + 链上结算）

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-23](./S-EXCH-23-prediction-market-ctf-lifecycle.md#oral-card) | **CTF、Outcome Token 与市场生命周期** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-24](./S-EXCH-24-prediction-market-clob-eip712-settlement.md#oral-card) | **CLOB、EIP-712 订单与链上结算** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-25](./S-EXCH-25-prediction-market-oracle-dispute-resolution.md#oral-card) | **预言机、体育/电竞数据源与争议仲裁** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-26](./S-EXCH-26-prediction-market-security-testing-mainnet.md#oral-card) | **安全不变量、测试矩阵与主网上线** | ⭐⭐⭐⭐⭐ |

> 四篇按 `资产与生命周期 → 交易与结算 → 现实事实与仲裁 → 安全与上线` 形成完整闭环。
> 协议字段和时间参数存在版本差异，正文会先讲通用不变量，再单独标注具体实现。

## CEX（中心化，Go 后端为主）

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-01](./S-EXCH-01-cex-matching-engine.md) | CEX 撮合引擎与订单簿架构 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-02](./S-EXCH-02-deposit-withdraw-wallet.md) | 充值、提现与链上钱包体系 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-03](./S-EXCH-03-account-ledger.md) | 账户体系与资金账务（复式记账） | ⭐⭐⭐⭐⭐ |
| [S-EXCH-04](./S-EXCH-04-futures-margin-liquidation.md) | 合约：保证金、强平、资金费率 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-16](./S-EXCH-16-perpetual-matching-position.md) | **永续合约撮合与仓位引擎** | ⭐⭐⭐⭐⭐ |
| [S-EXCH-05](./S-EXCH-05-risk-reconciliation.md) | 风控、对账与合规审计 | ⭐⭐⭐⭐⭐ |

## 可运行撮合与恢复

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-EXCH-17 | `examples/senior/matchingengine/` | `go test -race ./examples/senior/matchingengine/...` |
| S-EXCH-18 | `examples/senior/walreplay/` | `go test -race ./examples/senior/walreplay/...` |
| S-EXCH-19 | `examples/senior/marketdatarecovery/` | `go test -race ./examples/senior/marketdatarecovery/...` |
| S-EXCH-20 | `examples/senior/fixsession/` | `go test -race ./examples/senior/fixsession/...` |
| S-EXCH-21 | `examples/senior/matchingengine/` | `go test -race ./examples/senior/matchingengine/...` |
| S-EXCH-22 | `examples/senior/callauction/` | `go test ./examples/senior/callauction/...` |

## DEX（去中心化，合约 + 索引）

| ID | 题目 | 频率 |
|----|------|------|
| [S-EXCH-06](./S-EXCH-06-dex-amm-liquidity.md) | DEX AMM 与流动性池设计 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-27](./S-EXCH-27-pancakeswap-v2-v3-differences.md#oral-card) | PancakeSwap V2/V3：池、LP 与后端集成差异 | ⭐⭐⭐⭐⭐ |
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
| [S-EXCH-19](./S-EXCH-19-market-data-sequence-gap-recovery.md) | 行情序号、快照桥接与 Gap Recovery | ⭐⭐⭐⭐⭐ |
| [S-EXCH-20](./S-EXCH-20-fix-session-sequence-recovery.md) | FIX Session 序号与断线恢复 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-12](./S-EXCH-12-token-launch-rebate.md) | Token 发行、分账与返佣提现 | ⭐⭐⭐⭐⭐ |
| [S-EXCH-28](./S-EXCH-28-affiliate-tiered-rate-rebate.md#oral-card) | **多级代理极差费率、计佣账本与后台隔离** | ⭐⭐⭐⭐⭐ |

## 关联模块

| 已有题目 | 关系 |
|----------|------|
| [S-SOLID-07 DeFi 模式](../13-solidity-contracts/S-SOLID-07-defi-patterns.md) | AMM/Oracle 原理 |
| [S-BC-05 索引器](../12-blockchain-web3/S-BC-05-indexer-reorg.md) | DEX 成交/充值监听 |
| [S-ARCH-04 幂等](../03-system-design/S-ARCH-04-idempotency.md) | 充提、撮合幂等 |
| [S-ARCH-08 限流](../03-system-design/S-ARCH-08-rate-limiting.md) | 交易 API 防刷 |
| [S-SOL-08 白板模板](../11-solution-architecture/S-SOL-08-evolution-whiteboard.md) | 45min 答题结构 |

## 推荐刷题顺序

**预测市场技术负责人**：**EXCH-23 → 24 → 25 → 26** → EXCH-17/18/19 → EXCH-15 → SEC-01/04 → SOLID-01/04/08

**架构师**：EXCH-13 → **EXCH-23~26** → **EXCH-17~22** → EXCH-16 → EXCH-04 → EXCH-15 → 按需下钻 01/03/05

**交易系统后端**：**EXCH-17~22** → EXCH-16 → EXCH-04 → EXCH-01 → EXCH-03

**Web3 后端**：EXCH-14 → **27** → **23~26** → 10/11/12 → BC-05 → SOLID-04/08

## 岗位自测

- **CEX 后端**：能讲清撮合、账务、充提、强平四条链路
- **DEX 协议**：能画 AMM、讲清 LP 风险与 Oracle
- **预测市场**：能从规则冻结、CTF、签名订单、CLOB、链上结算讲到争议兑付与主网门禁
- **架构师**：能在 **45 分钟**内画完 CEX 或 Web3 全栈，并讲清对账与 HA（EXCH-13/14/15）
