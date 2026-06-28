---
id: S-SOLID-07
title: DeFi 合约模式：AMM / Oracle / 闪电贷
module: solidity-contracts
level: architect
frequency: 4
go_version: "1.22+"
tags: [defi, amm, oracle, flash-loan, solidity]
status: published
code_refs: []
sources:
  - https://docs.uniswap.org/contracts/v2/concepts/core-concepts/pools
  - https://docs.chain.link/data-feeds
---

# DeFi 合约模式：AMM / Oracle / 闪电贷

## 30 秒版（开场）

> DeFi 架构师要懂 **AMM 恒定乘积、Oracle 喂价、闪电贷单 tx 原子性**。Solidity 层定 **价格与清算规则**；Go 层做 **聚合展示与风控**，不能替代链上数学（[S-SOLID-08](./S-SOLID-08-contract-go-boundary.md)）。

## 3 分钟版（一面深度）

1. **是什么**：Uniswap 类 x*y=k；Chainlink aggregator；Aave flashLoan callback。
2. **为什么**：面试常问「如何设计链上 swap / 如何避免 oracle 操纵」。
3. **怎么做**：TWAP 抗操纵；多源 Oracle；闪电贷回调内自检 invariant。

## 10 分钟版

**AMM（Uniswap V2 简）**

```
x * y = k
amountOut = (amountIn * 997 * y) / (x * 1000 + amountIn * 997)  // 0.3% fee
```

- ** impermanent loss**：LP 相对 HODL 的机会成本
- **V3 集中流动性**：架构更复杂，Gas 不同

**Oracle**

| 类型 | 优点 | 风险 |
|------|------|------|
| Chainlink | 去中心化喂价 | 滞后、单源 |
| TWAP | 抗瞬时操纵 | 延迟 |
| 现货池价 | 无依赖 | 易被闪电贷操纵 |

**闪电贷**

```solidity
function executeOperation(...) external returns (bool) {
    // 1. 借 2. 套利/清算 3. 还+fee
    return true;
}
```

- 单 tx 内必须 **归还**；否则整 tx revert

## 生产场景

- 清算 bot：Solidity 规则 + Go 监控 mempool/事件触发
- 价格：关键操作不用 spot，用 TWAP 或 Chainlink

## 追问链

1. **MEV 与 DeFi？** → sandwich 攻击；架构上 private RPC/滑点保护。
2. **ERC-4626 vault？** → 标准化收益凭证。
3. **跨链 DeFi？** → 桥 + 双链流动性（[S-BC-07](../12-blockchain-web3/S-BC-07-l2-cross-chain-bridge.md)）。

## 反模式

- **单 DEX spot 作清算价**
- **Oracle 无 heartbeat 检查**

## 延伸阅读

- [Uniswap V2 Concepts](https://docs.uniswap.org/contracts/v2/concepts/core-concepts/pools)
- [Chainlink Data Feeds](https://docs.chain.link/data-feeds)
- [14 DEX/CEX：AMM 与 LP](../14-dex-cex-engineering/S-EXCH-06-dex-amm-liquidity.md)
