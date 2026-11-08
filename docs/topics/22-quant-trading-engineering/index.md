# 22 量化交易工程

8 篇 | 从数据与研究到回测、执行和生产控制 | [返回专题索引](../../topic-catalog.md)

本模块面向有后端基础、希望进入量化开发的工程师。先掌握通用交易系统，再按股票、期货或数字资产的市场规则深入。
与现有交易所模块的连接点是行情、订单、撮合和账本；新增重点是**数据在当时是否可见、回测如何模拟成交、策略意图如何安全执行**。

## 专题与阅读顺序

| 顺序 | 专题 | 能解决的问题 |
|------|------|--------------|
| 1 | [S-QUANT-01 量化开发全景与系统架构](S-QUANT-01-quant-development-architecture.md) | 区分研究、研究工程、交易系统和低延迟开发；确定能力缺口 |
| 2 | [S-QUANT-02 行情数据与市场微观结构](S-QUANT-02-market-data-microstructure.md) | 理解盘口、时间戳、点时数据与数据质量 |
| 3 | [S-QUANT-03 因子研究与统计验证](S-QUANT-03-factor-research-statistical-validation.md) | 理解收益、风险、IC、样本外验证和过拟合 |
| 4 | [S-QUANT-04 事件驱动回测](S-QUANT-04-event-driven-backtesting.md) | 模拟延迟、部分成交、手续费与滑点，避免未来数据泄漏 |
| 5 | [S-QUANT-05 OMS / EMS 与执行算法](S-QUANT-05-oms-ems-execution-algorithms.md) | 处理订单状态、撤单竞态、未知结果与 TWAP / VWAP / POV |
| 6 | [S-QUANT-06 组合风控、PnL 与对账](S-QUANT-06-portfolio-risk-pnl-reconciliation.md) | 管理挂单敞口、资金预占、损益和停机恢复 |
| 7 | [S-QUANT-07 做市与跨市场套利](S-QUANT-07-market-making-arbitrage.md) | 理解库存、逆向选择、腿风险、基差和资金费 |
| 8 | [S-QUANT-08 Go 交易系统生产工程](S-QUANT-08-go-trading-systems-production.md) | 设计有界队列、恢复、尾延迟测量和上线门禁 |

## 三类开发路径

| 方向 | 优先阅读 | 需要补齐 |
|------|----------|----------|
| 研究平台 / Quant Developer | 01 → 02 → 03 → 04 | Python、SQL、概率统计、数据质量、可复现实验 |
| 交易系统 / 执行与风控 | 01 → 02 → 05 → 06 → 08 | 交易协议、订单恢复、账户级并发控制与运维 |
| 做市 / 低延迟系统 | 02 → 04 → 05 → 07 → 08 | 微观结构、排队模型、C++/系统性能；按实际延迟预算选择实现 |

Go 生产经验能迁移到数据服务、交易网关、风控和控制面；研究计算与极低延迟执行需要分别评估 Python/C++ 及其他工具。
数学基础至少包括条件概率、期望/方差、协方差、线性回归、假设检验和时间序列相关性；03 给出这些概念在研究流程中的落点。

## 四周起步路线

| 周 | 阅读与练习设计 | 完成标准 |
|----|----------------|----------|
| W1 | 01 / 02；定义合成行情与证券主数据 schema | 能解释 event time、接收时间、可用时间、tick/lot 与缺失数据 |
| W2 | 03 / 04；手工推演一个滞后信号、下一时点成交的回测 | 能展示样本外结果口径与成本敏感性，指出仍未实现的成交假设 |
| W3 | 05 / 06；画订单、资金预占与停机状态机 | 能推演超时、重复回报、撤单途中成交、重启对账 |
| W4 | 07 / 08；设计双腿失败和行情陈旧的故障矩阵 | 能给出停止新增风险、恢复与验收条件，而非只展示收益曲线 |

这是学习顺序建议，不代表四周可以完成生产策略、低延迟系统或交易接入验收。

## 复用现有知识

- 行情恢复：[S-EXCH-19 序号与 Gap Recovery](../14-dex-cex-engineering/S-EXCH-19-market-data-sequence-gap-recovery.md)。
- 交易会话：[S-EXCH-20 FIX 会话与恢复](../14-dex-cex-engineering/S-EXCH-20-fix-session-sequence-recovery.md)。
- 撮合语义：[S-EXCH-17 确定性撮合](../14-dex-cex-engineering/S-EXCH-17-runnable-deterministic-matching-engine.md)；交易方的回测仍需单独模拟延迟和排队。
- 资金事实：[S-PAY-07 双分录账本](../18-web3-payments-stablecoin/S-PAY-07-postgresql-double-entry-ledger-lab.md)。
- 数字资产衍生品：[S-EXCH-04 保证金与强平](../14-dex-cex-engineering/S-EXCH-04-futures-margin-liquidation.md)。

## 证据边界与自测

本模块 8 篇均为 `explanation_only`：包含架构、公式、推演与练习设计，**尚无本模块独立的回测器、券商接入或策略收益验证**。
相邻示例的确定性测试只证明其各自覆盖的组件行为。

- 数据库事后修正的记录，如何避免影响历史信号？
- 用收盘价生成信号，为什么不能默认在同一个收盘价成交？
- 撤单请求已发出，但成交回报随后到达，仓位与资金如何处理？
- 两个策略同时通过风控，怎样避免合计敞口超限？
- 回测表现与仿真交易分歧，如何区分数据、逻辑、成交和成本问题？

来源按各篇末尾链接回查；规则与接口复核日期为 **2026-11-08**。
