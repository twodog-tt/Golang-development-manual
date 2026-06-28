# 12 区块链与 Web3

9 题 | P1 扩展（Web3 / 链上后端 JD） | [返回索引](../index.md)

> 面向 **Go 后端** 做链上数据索引、钱包、DApp 中台、交易所/ NFT 业务；偏 **工程落地**，非密码学研究员方向。

| ID | 题目 | 频率 |
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

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-BC-02 | `examples/senior/ethrpc/` | `go test ./examples/senior/ethrpc/...` |
| S-BC-09 | `examples/senior/erc20bind/` | `go test ./examples/senior/erc20bind/...` |

## 适用场景

- JD 含 **Web3 / 钱包 / L2 / 跨链 / 智能钱包 / 链上数据**
- 二面问「L2 和 L1 索引区别」「4337 UserOp」「怎么用 abigen」
- 与 [S-SOL-03 事件驱动](../11-solution-architecture/S-SOL-03-event-driven-cqrs.md)、[S-ARCH-04 幂等](../03-system-design/S-ARCH-04-idempotency.md) 交叉

## 推荐刷题顺序

EVM → RPC → 签名 → ABI 理论 → **abigen 实战** → 索引器 → L2/桥 → 4337 → DeFi 架构
