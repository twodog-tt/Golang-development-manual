# 18 Web3 支付与稳定币

6 篇 | 支付/稳定币岗位 P0 | [返回专题索引](../../topic-catalog.md) · [角色优先级](../_meta/role-priority-matrix.md)

> 把“链上转账”提升为可运营的支付系统：状态机、稳定币风险、Treasury、双分录、清结算、对账与合规控制。

| ID | 标题 | 频率 |
|----|------|------|
| [S-PAY-01](./S-PAY-01-payment-state-idempotency-reversal.md) | Web3 支付状态机、幂等、Webhook 与冲正 | ⭐⭐⭐⭐⭐ |
| [S-PAY-02](./S-PAY-02-stablecoin-issuer-crosschain-risk.md) | 稳定币发行人控制、跨链转移与结算风险 | ⭐⭐⭐⭐⭐ |
| [S-PAY-03](./S-PAY-03-treasury-liquidity-rebalancing.md) | Treasury、流动性与多链资金再平衡 | ⭐⭐⭐⭐ |
| [S-PAY-04](./S-PAY-04-ledger-clearing-settlement-reconciliation.md) | 支付账本、清结算与三方对账 | ⭐⭐⭐⭐⭐ |
| [S-PAY-05](./S-PAY-05-compliance-travel-rule-sanctions.md) | KYC/KYB、Travel Rule 与制裁筛查架构 | ⭐⭐⭐⭐⭐ |
| [S-PAY-06](./S-PAY-06-institutional-custody-rwa-iso20022.md) | 机构托管、DvP 清算、RWA 生命周期与 ISO 20022 | ⭐⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-PAY-01 | `examples/senior/paymentstate/` | `go test ./examples/senior/paymentstate/...` |

## 推荐顺序

支付状态机 → 双分录与对账 → 稳定币风险 → Treasury → 合规控制 → 机构托管/DvP/RWA/ISO 20022。
