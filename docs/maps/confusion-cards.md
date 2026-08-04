# 易混概念专卡

> 四条最容易讲错的工程边界。每条给 **一句话结论 → 对照表 → 深读链接**。  
> 返回：[概念地图总览](./index.md)

---

<a id="custody-vs-mpc"></a>

## 1. 托管信任模型 ≠ MPC 签名实现

**一句话**：托管回答「谁控制用户资产」；MPC 回答「平台热签如何降低单点私钥风险」。

| | 托管（Custody） | MPC / TSS |
|--|----------------|-----------|
| 讨论层面 | 信任模型 / 产品形态 | 密码学与出签实现 |
| CEX 零售用户 | 通常是托管账户（账本债权） | 用户无感；MPC 在平台热签侧 |
| 能否单方面转走 | 平台按规则可以（受审批/风控约束） | 不改变「平台能否转」本身 |
| 常见误读 | 「上了 MPC 就变成去中心化钱包」 | 「普通用户手里有一把 MPC 钱包」 |

去中心化/共管钱包：签名权在用户设备或共管份额，**厂商不能单方面转走**才算名副其实——那是另一类产品，不要和 CEX 托管热签混称。

**深读**：[MPC/TSS 与 CEX 托管签名](../topics/12-blockchain-web3/S-BC-10-mpc-tss-custody.md) ·
[MPC DKG/Reshare](../topics/17-multichain-wallet/S-WALLET-07-mpc-dkg-reshare-recovery.md) ·
[钱包概念地图](./wallet-custody.md)

---

<a id="indexer-vs-canonical"></a>

## 2. Indexer 投影 ≠ 链上最终事实

**一句话**：Canonical chain / 合约状态是事实；索引库是可丢、可重建的投影。

| | Canonical / 合约 | Indexer DB |
|--|------------------|------------|
| 权威性 | 权威 | 派生缓存 |
| reorg | 链自身分叉选择 | 必须回退投影再重放 |
| 唯一键 | 不证明「仍在规范链」 | 只防重复写入 |
| 故障恢复 | 以链为准 | 可从某高度重建 |

**深读**：[链上索引器 reorg](../topics/12-blockchain-web3/S-BC-05-indexer-reorg.md) ·
[Indexer 概念地图](./indexer-node-data.md)

---

<a id="mq-vs-idempotency"></a>

## 3. MQ at-least-once ≠ 业务 exactly-once

**一句话**：消息系统常见至少一次；业务「效果只发生一次」靠幂等键、状态机与本地事务/outbox。

| | 传输层（MQ） | 业务层 |
|--|--------------|--------|
| 常见保证 | at-least-once（可能重复） | effect-once（幂等收敛） |
| Kafka EOS | 多在 Kafka 读写边界内 | 不含外部 DB/链上/支付 |
| 该做什么 | 重试、DLQ、分区顺序 | `event_id` 唯一约束、合法状态迁移 |

**深读**：[消息队列语义](../topics/03-system-design/S-ARCH-10-mq-semantics.md) ·
[幂等设计](../topics/03-system-design/S-ARCH-04-idempotency.md) ·
[Indexer 概念地图](./indexer-node-data.md)

---

<a id="confirmation-vs-commit"></a>

## 4. 业务确认水位 ≠ 中间件 / 链上「提交」

**一句话**：Raft 提交、PoW 的 N 确认、Ethereum finalized、CometBFT height commit 不是同一语义；入账状态机必须写明用的是哪一种。

| | Raft / etcd | PoW N 确认 | Ethereum finalized | CometBFT commit |
|--|-------------|------------|--------------------|-----------------|
| 解决什么 | 封闭集群复制 | 概率最终的风险预算 | 经济最终 checkpoint | 高度级 BFT 提交 |
| 成员 | 已知节点 | 开放算力 | 开放质押 | 已知 validator set |
| 能否当「绝对不可逆」 | 在 CFT 假设内谈已提交日志 | 否，只是成本阈值 | 冲突属安全事件，非日常回滚 | 冲突破坏 BFT 假设 |
| 常见误读 | 「写进 etcd = 链上到账」 | 「6 块 = BFT commit」 | 「head 就是最终」 | 「prevote = commit」 |

**深读**：[经典共识 vs 链上共识](../topics/20-protocol-consensus-security/S-PROTO-05-classic-vs-onchain-consensus.md) ·
[Ethereum PoS / Finality](../topics/20-protocol-consensus-security/S-PROTO-01-ethereum-pos-fork-choice-finality.md) ·
[CometBFT 轮次与锁](../topics/20-protocol-consensus-security/S-PROTO-02-bft-cometbft-round-lock-safety-liveness.md)
