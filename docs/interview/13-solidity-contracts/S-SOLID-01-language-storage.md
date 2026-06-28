---
id: S-SOLID-01
title: Solidity 语言基础与 storage 布局
module: solidity-contracts
level: architect
frequency: 5
go_version: "1.22+"
tags: [solidity, storage, memory, evm, layout]
status: published
code_refs: [examples/senior/erc20bind/contract/SimpleToken.sol]
sources:
  - https://docs.soliditylang.org/en/latest/
  - https://docs.soliditylang.org/en/latest/internals/layout_in_storage.html
---

# Solidity 语言基础与 storage 布局

## 30 秒版（开场）

> Solidity 运行在 **EVM** 上：数据分 **storage**（持久、贵）、**memory**（临时）、**calldata**（只读入参）。架构师要懂 **storage 槽位、打包、继承线性化**，否则升级合约会 **存储冲突**。与 [S-BC-01 EVM 账户](../12-blockchain-web3/S-BC-01-blockchain-evm-basics.md) 衔接。

## 3 分钟版（一面深度）

1. **是什么**：静态类型、面向合约语言；0.8+ 默认溢出检查。
2. **为什么**：Gas 与安全问题多源于 **错误的数据位置** 和 **storage 布局变更**。
3. **怎么做**：状态变量顺序优化 packing；复杂结构用 `mapping`；对外接口 `external` + `calldata`。

## 10 分钟版（原理 + 图示）

**数据位置**

| 位置 | 生命周期 | Gas | 典型用途 |
|------|----------|-----|----------|
| storage | 链上永久 | 高 | 余额、配置 |
| memory | 函数内 | 中 | 临时数组 |
| calldata | 调用参数 | 低 | 外部函数入参 |

**storage 槽（slot）规则（面试高频）**

- 每个 slot 32 字节
- 小类型可 **打包**（如 `uint128 + uint128` 占 1 slot）
- `mapping` / 动态数组：单独槽公式，不连续 packing
- 继承：按 **C3 线性化** 顺序排列状态变量

```solidity
// 优化前：3 slots
uint128 a;
uint256 b;
uint128 c;

// 优化后：2 slots
uint128 a;
uint128 c;
uint256 b;
```

**可见性**

| 关键字 | 含义 |
|--------|------|
| public | 自动生成 getter |
| external | 仅外部调用，参数可用 calldata |
| internal | 合约+继承 |
| private | 仅本合约（仍占 storage） |

## 生产场景

- 升级合约前 **冻结 storage layout**（见 [S-SOLID-04](./S-SOLID-04-upgradeable-proxy.md)）
- 大数组循环 → Gas 超限，改 mapping 或分页
- 与 Go 交互：public mapping 无 key 枚举，需 **事件索引**（[S-BC-05](../12-blockchain-web3/S-BC-05-indexer-reorg.md)）

## 架构取舍

| 链上存全量 | 链下存哈希 |
|------------|------------|
| 贵、不可删 | 便宜、需信任链下 |

## 追问链

1. **memory vs storage 指针？** → storage 指针指向状态；memory 数组不能返回引用到 storage 除非明确。
2. **constant/immutable？** → 不占 storage slot（code 嵌入）。
3. **delete 语义？** → 清零可获 Gas refund（有限）。
4. **Solidity 0.8+ 溢出？** → 自动 revert；仍用 SafeCast 显式转换。

## 反模式与事故

- **升级时插入新状态变量到中间** → 存储错位，资产逻辑崩溃
- **unbounded loop** → DoS
- **public 数组** → 越界仅 off-chain 可见性

## 代码示例

本仓库 [SimpleToken.sol](https://github.com/twodog-tt/Golang-development-manual/blob/master/examples/senior/erc20bind/contract/SimpleToken.sol)：`mapping` 存余额。

## 延伸阅读

- [Solidity Docs](https://docs.soliditylang.org/)
- [Layout in Storage](https://docs.soliditylang.org/en/latest/internals/layout_in_storage.html)
