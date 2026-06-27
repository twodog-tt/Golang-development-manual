---
id: S-SOLID-03
title: ERC-20 / 721 / 1155 标准与实现
module: solidity-contracts
level: architect
frequency: 5
go_version: "1.22+"
tags: [erc20, erc721, erc1155, token, solidity]
status: published
code_refs: [examples/senior/erc20bind/contract/SimpleToken.sol]
sources:
  - https://eips.ethereum.org/EIPS/eip-20
  - https://eips.ethereum.org/EIPS/eip-721
  - https://eips.ethereum.org/EIPS/eip-1155
  - https://docs.openzeppelin.com/contracts/
---

# ERC-20 / 721 / 1155 标准与实现

## 30 秒版（开场）

> **ERC-20** 同质化代币；**ERC-721** NFT 唯一 id；**ERC-1155** 半同质化批量。架构师选型 + Review **approve/transferFrom、回调、元数据**。Go 索引靠 **Transfer/TransferSingle 事件**（[S-BC-04](../12-blockchain-web3/S-BC-04-contract-abi-events.md)）。

## 3 分钟版（一面深度）

1. **是什么**：EIP 接口约定，非强制但生态互操作依赖。
2. **为什么**：交易所/钱包只认标准；非标 token 导致 Go 后端解析失败。
3. **怎么做**：生产用 OpenZeppelin；注意 `decimals`、mint/burn、权限。

## 10 分钟版（对比表）

| 标准 | 模型 | 核心方法 | 典型场景 |
|------|------|----------|----------|
| ERC-20 | 同质化 | transfer, approve, allowance | USDT, 平台币 |
| ERC-721 | 唯一 id | safeTransferFrom + onERC721Received | 艺术品 NFT |
| ERC-1155 | id + 数量 | safeBatchTransferFrom | 游戏道具 |

**ERC-20 面试要点**

```solidity
function transfer(address to, uint256 amount) external returns (bool);
function approve(address spender, uint256 amount) external returns (bool);
function transferFrom(address from, address to, uint256 amount) external returns (bool);
event Transfer(address indexed from, address indexed to, uint256 value);
```

- **approve  Front-running**：可先 approve(0) 再 approve(n)
- **fee-on-transfer**：实际到账 < amount，DEX 需特殊处理
- **USDT 无 bool 返回**：Go 解析要兼容

**ERC-721 safeTransfer**

- 必须检查接收合约实现 `onERC721Received`，防误转黑洞

**ERC-1155 批量**

- 一次 tx 多 id/数量；Gas 优于多次 ERC-721

## 生产场景

- 平台币 + NFT：20 作 Gas/积分，721 作凭证
- 元数据：`tokenURI` → IPFS JSON（[S-BC-06](../12-blockchain-web3/S-BC-06-defi-backend-patterns.md)）
- Go abigen 绑定： [S-BC-09](../12-blockchain-web3/S-BC-09-abigen-contract-bindings.md)

## 架构取舍

| 自研 token | OZ 继承 |
|------------|---------|
| 灵活 | 审计省心 |

## 追问链

1. **721 vs 1155 何时选？** → 唯一 vs 可堆叠同类资产。
2. **ERC-4626？** → 收益型 vault token，DeFi 标准化。
3. **Permit (EIP-2612)？** → 链上 approve 改签名，省 Gas。
4. **双代币模型？** → 治理 token + 质押 receipt token。

## 反模式与事故

- **无限 approve 给不可信合约**
- **721 用 transfer 非 safeTransfer** → 合约钱包收不到
- **decimals 假设 18** → USDC 6 位

## 代码示例

[SimpleToken.sol](../../../examples/senior/erc20bind/contract/SimpleToken.sol) — 教学用最小实现；生产用 OZ `ERC20`.

## 延伸阅读

- [EIP-20](https://eips.ethereum.org/EIPS/eip-20)
- [OpenZeppelin Contracts](https://docs.openzeppelin.com/contracts/)
