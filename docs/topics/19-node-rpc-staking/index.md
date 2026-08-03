# 19 节点、RPC 与 Staking

10 篇 | 本专题核心阅读 | [返回专题索引](../../topic-catalog.md) · [领域能力优先级](../_meta/role-priority-matrix.md)

> 面向节点平台、RPC 网关、Indexer、Relayer、Validator 与链上数据架构，重点是 **状态机、数据正确性和运维边界**。

| ID | 标题 | 频率 |
|----|------|------|
| [S-NODE-01](./S-NODE-01-ethereum-node-architecture-sync.md) | Ethereum EL/CL、Full/Archive Node 与同步模式 | ⭐⭐⭐⭐⭐ |
| [S-NODE-02](./S-NODE-02-rpc-ha-quorum-hedging-cache.md) | RPC 高可用：多 Provider、Quorum、Hedging 与缓存 | ⭐⭐⭐⭐⭐ |
| [S-NODE-03](./S-NODE-03-validator-staking-slashing-keys.md) | Validator、Staking、Slashing 与密钥生命周期 | ⭐⭐⭐⭐⭐ |
| [S-NODE-04](./S-NODE-04-chain-data-platform.md) | 链上数据平台：Backfill、实时流、Trace 与 Schema | ⭐⭐⭐⭐⭐ |
| [S-NODE-05](./S-NODE-05-relayer-transaction-manager.md) | Relayer 与交易管理器：Nonce、Fee、Replacement、Finality | ⭐⭐⭐⭐⭐ |
| [S-NODE-06](./S-NODE-06-node-operations-runbook.md) | 节点运维：升级、快照、Pruning、监控与 Runbook | ⭐⭐⭐⭐ |
| [S-NODE-07](./S-NODE-07-canonical-backfill-realtime-merge.md) | Canonical Backfill + Realtime Merge 与 Reorg 提交协议 | ⭐⭐⭐⭐⭐ |
| [S-NODE-08](./S-NODE-08-trace-state-diff-versioned-decoder-quality.md) | Trace、State Diff、版本化 Decoder 与链数据质量 | ⭐⭐⭐⭐⭐ |
| [S-NODE-09](./S-NODE-09-non-evm-online-sdk-fault-injection.md) | 非 EVM 在线 SDK：提交、确认、故障注入与升级兼容 | ⭐⭐⭐⭐⭐ |
| [S-NODE-10](./S-NODE-10-chain-data-clickhouse-lakehouse.md) | ClickHouse、Reorg 与 Lakehouse 分层 | ⭐⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-NODE-02 | `examples/senior/rpcpool/` | `go test -race ./examples/senior/rpcpool/...` |
| S-NODE-07 | `examples/senior/chainmerge/` | `go test -race ./examples/senior/chainmerge/...` |
| S-NODE-09 | `examples/senior/txlifecycle/` | `go test -race ./examples/senior/txlifecycle/...` |

## 推荐顺序

节点架构 → RPC HA → Validator 安全 → 数据平台 → Canonical Merge → Trace/Decoder/质量 →
ClickHouse/Lakehouse → Relayer → 非 EVM 在线生命周期 → 节点运维。
