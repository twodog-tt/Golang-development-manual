---
id: S-EXCH-09
title: CEX 与 DEX 混合架构（CeDeFi）
module: dex-cex-engineering
level: architect
frequency: 4
go_version: "1.22+"
tags: [cedefi, hybrid, wallet, onboarding, architecture]
status: published
code_refs: []
sources:
  - https://ethereum.org/en/developers/docs/accounts/
---

# CEX 与 DEX 混合架构（CeDeFi）

## 30 秒版（开场）

> “CeDeFi/混合交易所”不是严格技术标准，讲解时先明确产品的 **托管、订单排序、
> 签名和结算信任模型**。同一 App 可以同时提供 CEX 账户与自托管钱包/DEX，但必须
> 分清 **平台负债账本、平台受控链上资产、用户自托管资产**，避免重复计入余额或对账。

## 3 分钟版（精讲深度）

1. **是什么**：同一 App 内现货 CEX + Web3 钱包/DEX 入口。
2. **为什么**：交易所转型 Web3 常见工程场景。
3. **怎么做**：账户体系隔离或打通；充提桥接；统一风控视图。

## 10 分钟版

```mermaid
flowchart TB
  subgraph CEX[CEX 域]
    API[交易 API]
    ME[撮合]
    Ledger[账务]
  end
  subgraph Web3[Web3 域]
    Wallet[钱包/Signer]
    Indexer[链索引]
    DEXAgg[DEX 聚合 API]
  end
  User[用户] --> API
  User --> Wallet
  Ledger <-->|站内划转/出金| Wallet
  Indexer --> Ledger
```

**典型产品形态**

| 形态 | 说明 |
|------|------|
| 托管交易 + 链上提 | 经典 CEX |
| 嵌入式 DEX | App 内 swap，平台收路由费 |
| 全自托管 | 无 CEX 客户负债账本，但仍可能有 relayer、indexer、quote/API 和合约依赖 |
| 子账户 / MPC 钱包 | 链上地址平台协管 |

**打通难点**

| 难点 | 方案 |
|------|------|
| 同一用户 CEX/DEX 身份 | 统一 UID；地址绑定用带 domain、chain、nonce、expiry 的签名挑战，并区分 EOA 与 ERC-1271 智能账户 |
| 资产显示 | CEX 余额 + 链上余额聚合 |
| 合规 | 链上交互 KYT；地域限制 |
| 对账 | 平台受控链上资产与 CEX 客户负债/平台权益分资产核对；用户自托管地址余额只作展示，不能算作平台储备 |

**Go 统一后端**

- **BFF 层**：聚合 CEX REST + Web3 服务
- **事件总线**：链上充值确认 → 触发 CEX 入账或仅展示
- **特性开关**：按地区开/关 DEX 模块

## 生产场景

- **用户混淆托管与链上** → 充错地址；强 UI 区分
- **站内「闪兑」** → 后台走 CEX 流动性或链上 Router，两套报价
- **Launchpad** → 链上认购 + CEX 分销 KYC

## 深挖问答

1. **为什么大厂做 DEX 模块？** → 留存、上币、手续费、生态。
2. **链上失败 CEX 已扣？** → Saga：先做 reservation，再签名/广播；明确失败后追加
   解冻，成功后结算。RPC 超时先进入 unknown 并查 tx/nonce，不能立即“回滚”后重发。
3. **与纯 DEX 聚合器区别？** → 可能有托管流动性、法币入口。
4. **架构师汇报怎么讲？** → 两域隔离、事件驱动、统一风控与对账。

## 反模式

- **CEX 账本与链上钱包混一张表** → 审计噩梦
- **无绑定点就链上入账** → 无法归属用户
- **把 feature flag 当作合规控制的全部** → 地域、主体、资产和链上直接访问还需要
  独立政策、审计与执行边界

## 延伸阅读

- [S-EXCH-02 充提钱包](./S-EXCH-02-deposit-withdraw-wallet.md)
- [S-EXCH-03 账务](./S-EXCH-03-account-ledger.md)
- [S-BC-01 EVM 基础](../12-blockchain-web3/S-BC-01-blockchain-evm-basics.md)
