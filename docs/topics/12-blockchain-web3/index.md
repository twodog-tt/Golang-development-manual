# 12 区块链与 Web3

17 篇 | Web3 链上工程 | [返回专题索引](../../topic-catalog.md) · [Web3 场景地图](../../web3-exchange-wallet-focus.md)

> 面向 **Go 后端** 做链上数据索引、钱包、DApp 中台、交易所业务；偏 **工程落地**，非密码学研究员方向。
> 概念地图：[Indexer](../../maps/indexer-node-data.md) · [钱包/MPC](../../maps/wallet-custody.md)。

| ID | 标题 | 频率 |
|----|------|------|
| [S-BC-01](./S-BC-01-blockchain-evm-basics.md) | 区块链基础与 EVM 账户模型 | ⭐⭐⭐⭐⭐ |
| [S-BC-02](./S-BC-02-go-ethereum-rpc.md) | Go 连接节点：JSON-RPC 与 ethclient | ⭐⭐⭐⭐⭐ |
| [S-BC-03](./S-BC-03-tx-signing-key-mgmt.md) | 交易签名与密钥管理 | ⭐⭐⭐⭐⭐ |
| [S-BC-04](./S-BC-04-contract-abi-events.md) | 智能合约交互：ABI 与事件监听 | ⭐⭐⭐⭐⭐ |
| [S-BC-05](./S-BC-05-indexer-reorg.md) | 链上索引器：扫块、重组与幂等 | ⭐⭐⭐⭐⭐ |
| [S-BC-06](./S-BC-06-defi-backend-patterns.md) | DeFi / NFT 后端架构模式 | ⭐⭐⭐⭐ |
| [S-BC-07](./S-BC-07-l2-cross-chain-bridge.md) | L2 扩容与跨链桥架构 | ⭐⭐⭐⭐⭐ |
| [S-BC-08](./S-BC-08-erc4337-account-abstraction.md) | Account Abstraction ERC-4337 | ⭐⭐⭐⭐ |
| [S-BC-09](./S-BC-09-abigen-contract-bindings.md) | go-ethereum abigen 完整实战 | ⭐⭐⭐⭐⭐ |
| [S-BC-10](./S-BC-10-mpc-tss-custody.md) | MPC/TSS 与 CEX 托管签名 | ⭐⭐⭐⭐⭐ |
| [S-BC-11](./S-BC-11-rollup-finality-da-proof-security.md) | Rollup Finality、DA、证明与强制退出 | ⭐⭐⭐⭐⭐ |
| [S-BC-12](./S-BC-12-cross-chain-message-bridge-security.md) | 跨链消息认证、重放与限额 | ⭐⭐⭐⭐⭐ |
| [S-BC-13](./S-BC-13-gas-fee-multichain.md) | Gas / Fee 计费与多链费用差异 | ⭐⭐⭐⭐⭐ |
| [S-BC-14](./S-BC-14-evm-chains-landscape-integration.md) | EVM 公链全景速览：L1、侧链、Rollup 与接入差异 | ⭐⭐⭐⭐⭐ |
| [S-BC-15](./S-BC-15-evm-chain-identity-verification.md) | 如何验证一条 EVM 公链：身份、活性、共识与资产证据 | ⭐⭐⭐⭐⭐ |
| [S-BC-16](./S-BC-16-transaction-lifecycle-finality-reorg.md) | EVM 交易生命周期：Pending、Receipt、确认、最终性与重组 | ⭐⭐⭐⭐⭐ |
| [S-BC-17](./S-BC-17-rpc-node-explorer-ha-runbook.md) | RPC 节点与区块浏览器：生产架构、高可用与 502 恢复 | ⭐⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-BC-02 | `examples/senior/ethrpc/` | `go test ./examples/senior/ethrpc/...` |
| S-BC-09 | `examples/senior/erc20bind/` | `go test ./examples/senior/erc20bind/...` |
| S-BC-12 | `examples/senior/bridgeguard/` | `go test -race ./examples/senior/bridgeguard/...` |

## 适用场景

- 场景涉及 **Web3 / 钱包 / L2 / 跨链 / 智能钱包 / 链上数据**
- 二面问「L2 和 L1 索引区别」「4337 UserOp」「怎么用 abigen」
- 与 [S-SOL-03 事件驱动](../11-solution-architecture/S-SOL-03-event-driven-cqrs.md)、[S-ARCH-04 幂等](../03-system-design/S-ARCH-04-idempotency.md) 交叉

## 推荐阅读顺序

EVM → **EVM 公链全景** → **公链身份核验** → **交易生命周期与最终性** →
**RPC/浏览器高可用** → **Gas/Fee 与多链费用** → RPC 编程 → 签名 → ABI 理论 →
**abigen 实战** → 索引器 → L2 概览 → **Rollup 安全边界** → **跨链消息安全** → 4337 → DeFi 架构
