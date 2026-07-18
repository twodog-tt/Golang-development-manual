# 20 协议、共识与安全

4 题 | P1 深挖（**链基础设施 / 节点 / Protocol Engineer / 架构师** JD） | [返回索引](../../interview-catalog.md)

> 本模块用于把“会调用节点 RPC”提升到“能解释节点为何接受这条链、何时最终、
> 数据为何可用、升级为何不会分叉”。重点不是背术语，而是说清 **安全假设、状态机、
> 失败边界与恢复方式**。

## 题目

| ID | 题目 | 频率 |
|----|------|------|
| [S-PROTO-01](./S-PROTO-01-ethereum-pos-fork-choice-finality.md) | Ethereum PoS、Fork Choice、Finality 与弱主观性 | ⭐⭐⭐⭐⭐ |
| [S-PROTO-02](./S-PROTO-02-bft-cometbft-round-lock-safety-liveness.md) | BFT / CometBFT：轮次、锁、安全性与活性 | ⭐⭐⭐⭐⭐ |
| [S-PROTO-03](./S-PROTO-03-blob-da-peerdas-security.md) | Blob、DA 与 PeerDAS：从 EIP-4844 到 Fusaka | ⭐⭐⭐⭐⭐ |
| [S-PROTO-04](./S-PROTO-04-protocol-upgrade-state-migration.md) | 协议升级、状态迁移与不可回滚边界 | ⭐⭐⭐⭐⭐ |

## 推荐顺序

1. **S-PROTO-01**：先分清 head、justified、finalized 与 weak-subjectivity checkpoint。
2. **S-PROTO-02**：再比较链式 PoS fork choice 与按高度多轮 BFT。
3. **S-PROTO-03**：理解 Rollup 依赖的 DA 不只是“把数据哈希上链”。
4. **S-PROTO-04**：把协议规则、节点二进制、链上状态迁移和本地数据库迁移拆开。

## 与现有模块的关系

| 已有题目 | 本模块补充 |
|----------|------------|
| [S-NODE-01 EL/CL 与同步](../19-node-rpc-staking/S-NODE-01-ethereum-node-architecture-sync.md) | 深入 fork choice、finality 与 weak subjectivity |
| [S-WALLET-04 Cosmos / CometBFT / IBC](../17-multichain-wallet/S-WALLET-04-cosmos-cometbft-ibc-sequence.md) | 深入 round、prevote、precommit、lock |
| [S-BC-11 Rollup 安全边界](../12-blockchain-web3/S-BC-11-rollup-finality-da-proof-security.md) | 深入 blob、KZG、PeerDAS 与 DA 保留 |
| [S-NODE-06 节点运维](../19-node-rpc-staking/S-NODE-06-node-operations-runbook.md) | 深入 fork activation、迁移与回滚边界 |

## 岗位自测

- 能否解释“最新 head 已变化，但 finalized checkpoint 没变”为什么不是共识故障？
- 能否说明 CometBFT 中 `+2/3 prevote`、`+2/3 precommit` 和 commit 的差异？
- 能否说明 PeerDAS 已上线，并纠正“每个节点仍下载全部 blob”的过时表达？
- 能否解释为什么新规则已经 finalized 后，旧二进制不能作为普通回滚方案？
