# 13 Solidity 与合约工程

8 题 | P1 扩展（**区块链架构师 / 全栈 Web3**） | [返回索引](../README.md)

> 面向 **Go 后端 + 区块链架构师**：能设计、Review、审计 Solidity，并与 [12 Web3 Go 层](../12-blockchain-web3/) 协作。偏 **合约工程与安全**，非纯理论研究。

| ID | 题目 | 频率 |
|----|------|------|
| [S-SOLID-01](./S-SOLID-01-language-storage.md) | Solidity 语言基础与 storage 布局 | ⭐⭐⭐⭐⭐ |
| [S-SOLID-02](./S-SOLID-02-security-reentrancy.md) | 合约安全：重入、权限与 OWASP | ⭐⭐⭐⭐⭐ |
| [S-SOLID-03](./S-SOLID-03-erc-standards.md) | ERC-20 / 721 / 1155 标准与实现 | ⭐⭐⭐⭐⭐ |
| [S-SOLID-04](./S-SOLID-04-upgradeable-proxy.md) | 可升级合约：Proxy / UUPS / 存储槽 | ⭐⭐⭐⭐⭐ |
| [S-SOLID-05](./S-SOLID-05-gas-optimization.md) | Gas 优化与设计模式 | ⭐⭐⭐⭐ |
| [S-SOLID-06](./S-SOLID-06-testing-audit.md) | Foundry 测试与审计清单 | ⭐⭐⭐⭐⭐ |
| [S-SOLID-07](./S-SOLID-07-defi-patterns.md) | DeFi 合约模式：AMM / Oracle / 闪电贷 | ⭐⭐⭐⭐ |
| [S-SOLID-08](./S-SOLID-08-contract-go-boundary.md) | 合约与 Go 后端架构边界 | ⭐⭐⭐⭐⭐ |

## 示例合约

| 题 ID | 路径 | 说明 |
|-------|------|------|
| S-SOLID-02 | `examples/solidity/ReentrancyGuard.sol` | 重入防护模式 |
| S-SOLID-03 | `examples/senior/erc20bind/contract/SimpleToken.sol` | 最小 ERC20 + abigen |

## 与 Go 模块关系

| Go（12） | Solidity（13） |
|----------|----------------|
| RPC、索引、签名 | 链上逻辑、资产规则 |
| [S-BC-09 abigen](../12-blockchain-web3/S-BC-09-abigen-contract-bindings.md) | ABI 由本模块合约产出 |
| [S-BC-05 索引器](../12-blockchain-web3/S-BC-05-indexer-reorg.md) | 监听本模块定义的事件 |

## 推荐刷题顺序

语言/存储 → 安全 → ERC 标准 → 升级 → Gas → 测试审计 → DeFi → 与 Go 分工
