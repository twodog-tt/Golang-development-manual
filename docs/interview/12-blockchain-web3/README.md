# 12 区块链与 Web3

6 题 | P1 扩展（Web3 / 链上后端 JD） | [返回索引](../README.md)

> 面向 **Go 后端** 做链上数据索引、钱包、DApp 中台、交易所/ NFT 业务；偏 **工程落地**，非密码学研究员方向。

| ID | 题目 | 频率 |
|----|------|------|
| [S-BC-01](./S-BC-01-blockchain-evm-basics.md) | 区块链基础与 EVM 账户模型 | ⭐⭐⭐⭐⭐ |
| [S-BC-02](./S-BC-02-go-ethereum-rpc.md) | Go 连接节点：JSON-RPC 与 ethclient | ⭐⭐⭐⭐⭐ |
| [S-BC-03](./S-BC-03-tx-signing-key-mgmt.md) | 交易签名与密钥管理 | ⭐⭐⭐⭐⭐ |
| [S-BC-04](./S-BC-04-contract-abi-events.md) | 智能合约交互：ABI 与事件监听 | ⭐⭐⭐⭐⭐ |
| [S-BC-05](./S-BC-05-indexer-reorg.md) | 链上索引器：扫块、重组与幂等 | ⭐⭐⭐⭐⭐ |
| [S-BC-06](./S-BC-06-defi-backend-patterns.md) | DeFi / NFT 后端架构模式 | ⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-BC-02 | `examples/senior/ethrpc/` | `go test ./examples/senior/ethrpc/...` |

## 适用场景

- JD 含 **Web3 / 区块链 / 钱包 / 链上数据 / DeFi**
- 二面问「怎么扫块」「重组怎么处理」「Go 怎么发交易」
- 与 [S-SOL-03 事件驱动](../11-solution-architecture/S-SOL-03-event-driven-cqrs.md)、[S-ARCH-04 幂等](../03-system-design/S-ARCH-04-idempotency.md) 交叉

## 推荐刷题顺序

EVM 基础 → RPC 连接 → 签名 → 合约/事件 → 索引器 → 业务架构
