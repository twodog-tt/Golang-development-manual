---
id: S-SOLID-05
title: Gas 优化与设计模式
module: solidity-contracts
level: architect
frequency: 4
go_version: "1.22+"
tags: [gas, optimization, patterns, solidity]
status: published
code_refs: []
sources:
  - https://docs.soliditylang.org/en/latest/internals/optimizer.html
  - https://www.evm.codes/
  - https://eips.ethereum.org/EIPS/eip-1014
---

# Gas 优化与设计模式

## 30 秒版（开场）

> Gas 是 EVM 执行和交易数据的计费单位。架构师在 **可读性、安全性与成本**
> 间权衡：按访问模式设计 storage、减少无谓拷贝、合理使用 `immutable`、批量操作。
> 事件只能替代“仅供链下查询”的数据，不能替代合约未来需要读取的状态。

## 3 分钟版（精讲深度）

1. **是什么**：交易费通常由执行 Gas、费用市场参数以及链/L2 的数据可用性费用共同决定；
   EIP-1559 的 base fee + priority fee 是费用定价机制，不会改变合约本身消耗的 gas units。
2. **为什么**：高频协议（DEX、mint）Gas 差 20% 即竞争力差。
3. **怎么做**：先 profile（`forge test --gas-report`）；热路径优化；冷路径可读优先。

## 10 分钟版（优化清单）

同一笔交易中，首次访问某个账户或 storage key 与后续 warm access 的成本不同；
`SSTORE` 成本还取决于原值、当前值、新值和退款规则。讲解时应讲相对关系与
目标 fork，避免死记一组会随 EIP 变化的固定数字。

| 技巧 | 说明 |
|------|------|
| 变量打包 | 可减少槽位，但高频单字段更新可能增加读-改-写；按访问模式 benchmark |
| calldata | 对只读外部参数可避免部分 memory 拷贝；同时考虑交易 calldata/L2 DA 成本 |
| immutable/constant | 不占普通 storage slot；immutable 是构造时赋值后嵌入代码 |
| 事件索引 | 仅对合约以后无需读取的数据，用日志服务链下索引；合约不能读取历史 logs |
| unchecked | 仅在溢出边界已证明、测试覆盖且对固定编译器配置 benchmark 后考虑；现代 optimizer 可能已消除部分循环检查，不应作为机械模板 |
| 短路 | 便宜检查放前 |
| 自定义 error | 比 revert string 省 Gas |
| 克隆 minimal proxy | EIP-1167 批量部署 |

**设计模式**

| 模式 | 用途 |
|------|------|
| Pull Payment | 用户自提，防 push 失败 |
| Checks-Effects-Interactions | 安全序 |
| Diamond (EIP-2535) | 超大合约分 facet |
| Factory + CREATE2 | 地址由部署者、salt 与 init-code hash 决定；“salt 抢占”只在公开工厂、salt/初始化未绑定调用者或权限不足等设计下成立，还要防未原子初始化与恶意 init code |

**何时不优化**

- 安全关键路径 clarity > 省 2k Gas
- 过早 inline assembly

## 生产场景

- NFT 批量 mint：ERC-1155 + merkle allowlist
- 存储迁移：链下 Merkle root 上链验证

## 深挖问答

1. **SLOAD vs MLOAD 成本？** → storage 贵 orders of magnitude。
2. **cold vs warm access？** → EIP-2929 首次访问更贵。
3. **assembly 何时用？** → 库级优化；需双审计。
4. **L2 Gas 差异？** → 不同 Rollup 会分别计 L2 执行、L1 calldata/blob 或其他 DA
   成本；某些优化在 L1 和 L2 的收益排序不同，必须按目标链测量
   （[S-BC-07](../12-blockchain-web3/S-BC-07-l2-cross-chain-bridge.md)）。

## 反模式

- **链上存大字符串/JSON**
- **O(n) 遍历全用户** → 不可扩展
- **只看单次 gas snapshot** → 忽略编译器/optimizer/via-IR、目标 fork、典型状态和
  L2 数据费用变化

## 延伸阅读

- [evm.codes](https://www.evm.codes/)
