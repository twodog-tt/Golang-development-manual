# 18 Web3 支付与稳定币

11 篇 | 本专题核心阅读 | [返回专题索引](../../topic-catalog.md) · [领域能力优先级](../_meta/role-priority-matrix.md)

> 把“链上转账”提升为可运营的支付系统：状态机、稳定币风险、Treasury、双分录、清结算、对账与合规控制。

| ID | 标题 | 频率 |
|----|------|------|
| [S-PAY-01](./S-PAY-01-payment-state-idempotency-reversal.md) | Web3 支付状态机、幂等、Webhook 与冲正 | ⭐⭐⭐⭐⭐ |
| [S-PAY-02](./S-PAY-02-stablecoin-issuer-crosschain-risk.md) | 稳定币发行人控制、跨链转移与结算风险 | ⭐⭐⭐⭐⭐ |
| [S-PAY-03](./S-PAY-03-treasury-liquidity-rebalancing.md) | Treasury、流动性与多链资金再平衡 | ⭐⭐⭐⭐ |
| [S-PAY-04](./S-PAY-04-ledger-clearing-settlement-reconciliation.md) | 支付账本、清结算与三方对账 | ⭐⭐⭐⭐⭐ |
| [S-PAY-05](./S-PAY-05-compliance-travel-rule-sanctions.md) | KYC/KYB、Travel Rule 与制裁筛查架构 | ⭐⭐⭐⭐⭐ |
| [S-PAY-06](./S-PAY-06-institutional-custody-rwa-iso20022.md) | 机构托管、DvP 清算、RWA 生命周期与 ISO 20022 | ⭐⭐⭐⭐⭐ |
| [S-PAY-07](./S-PAY-07-postgresql-double-entry-ledger-lab.md) | PostgreSQL 双分录账本：并发扣款、冻结冲正与故障验证 | ⭐⭐⭐⭐⭐ |

| [S-PAY-08](./S-PAY-08-temporal-money-workflows.md) | Temporal 资金长流程：审批、恢复、补偿与版本回放 | ⭐⭐⭐⭐⭐ |
| [S-PAY-09](./S-PAY-09-wallet-auth-siwe.md) | 钱包登录与账号绑定：SIWE、合约验签和会话撤销 | ⭐⭐⭐⭐⭐ |
| [S-PAY-10](./S-PAY-10-go-typescript-web3-integration.md) | Go 与 TypeScript Web3 联调：精度、消息签名和支付 API | ⭐⭐⭐⭐⭐ |
| [S-PAY-11](./S-PAY-11-stablecoin-channel-operations.md) | 稳定币渠道与资金运营：未知状态、Webhook、退款和对账闭环 | ⭐⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-PAY-01 | `examples/senior/paymentstate/` | `go test ./examples/senior/paymentstate/...` |
| S-PAY-08~11 | `examples/stablecoin-ops/` | `python3 examples/stablecoin-ops/verify.py` |

## 推荐顺序

支付状态机 → 双分录与对账 → 稳定币风险 → Treasury → 合规控制 → 机构托管/DvP/RWA/ISO 20022。

## 稳定币支付与资金运营项目

四篇共用 Go + TypeScript + PostgreSQL + Temporal 本地项目，按流程、身份、联调、运营顺序阅读：

- [Temporal 资金长流程：审批、恢复、补偿与版本回放](S-PAY-08-temporal-money-workflows.md)
- [钱包登录与账号绑定：SIWE、合约验签和会话撤销](S-PAY-09-wallet-auth-siwe.md)
- [Go 与 TypeScript Web3 联调：精度、消息签名和支付 API](S-PAY-10-go-typescript-web3-integration.md)
- [稳定币渠道与资金运营：未知状态、Webhook、退款和对账闭环](S-PAY-11-stablecoin-channel-operations.md)

[项目与一键验证](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/stablecoin-ops)：真实本地数据库和 Workflow Server，渠道为模拟夹具，不接真实资金。
