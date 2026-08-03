# 12 区块链与 Web3

13 篇 | P1 扩展（Web3 / 链上后端 JD） | [返回专题索引](../../topic-catalog.md) · [重点专题](../../web3-exchange-wallet-focus.md)

> 面向 **Go 后端** 做链上数据索引、钱包、DApp 中台、交易所/ NFT 业务；偏 **工程落地**，非密码学研究员方向。

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

EVM → **Gas/Fee 与多链费用** → RPC → 签名 → ABI 理论 → **abigen 实战** → 索引器 → L2 概览 →
**Rollup 安全边界** → **跨链消息安全** → 4337 → DeFi 架构
