# 17 多链钱包与托管

12 篇 | 钱包/托管岗位 P0 | [返回专题索引](../../topic-catalog.md) · [角色优先级](../_meta/role-priority-matrix.md)

> 目标不是背更多链名，而是建立一套不会错误泛化的 **Chain Adapter、交易并发、归集、签名和恢复模型**。

| ID | 标题 | 频率 |
|----|------|------|
| [S-WALLET-01](./S-WALLET-01-chain-adapter-capability-matrix.md) | 多链钱包 Chain Adapter 与能力矩阵 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-02](./S-WALLET-02-bitcoin-utxo-psbt-fee-bump.md) | Bitcoin UTXO、Coin Selection、PSBT 与手续费替换 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-03](./S-WALLET-03-solana-account-pda-transaction.md) | Solana 账户模型、PDA 与交易生命周期 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-04](./S-WALLET-04-cosmos-cometbft-ibc-sequence.md) | Cosmos SDK、CometBFT、IBC 与账户 Sequence | ⭐⭐⭐⭐ |
| [S-WALLET-05](./S-WALLET-05-sui-aptos-state-model.md) | Sui Object 与 Aptos Resource 模型对比 | ⭐⭐⭐⭐ |
| [S-WALLET-06](./S-WALLET-06-deposit-sweep-reservation-recovery.md) | 充值地址、归集、Nonce/UTXO 预占与恢复 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-07](./S-WALLET-07-mpc-dkg-reshare-recovery.md) | MPC/TSS 的 DKG、Reshare 与故障恢复 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-08](./S-WALLET-08-solana-go-sdk-transaction.md) | Solana Go SDK 离线构建、签名与确认 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-09](./S-WALLET-09-cosmos-go-sdk-sign-mode-direct.md) | Cosmos SDK TxBuilder 与 DIRECT 签名 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-10](./S-WALLET-10-aptos-go-sdk-bcs-transaction.md) | Aptos Go SDK、BCS 与执行跟踪 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-11](./S-WALLET-11-sui-go-capability-adapter.md) | Sui Object、Address Balance 与能力适配 | ⭐⭐⭐⭐⭐ |
| [S-WALLET-12](./S-WALLET-12-tron-trc20-resource-transaction.md) | TRON/TRC20 资源、权限与交易生命周期 | ⭐⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-WALLET-02 | `examples/senior/coinselect/` | `go test ./examples/senior/coinselect/...` |
| S-WALLET-08 | `examples/non-evm-sdk/solana/` | `cd examples/non-evm-sdk/solana && go test ./...` |
| S-WALLET-09 | `examples/non-evm-sdk/cosmos/` | `cd examples/non-evm-sdk/cosmos && go test -p=1 ./...` |
| S-WALLET-10 | `examples/non-evm-sdk/aptos/` | `cd examples/non-evm-sdk/aptos && go test ./...` |
| S-WALLET-11 | `examples/non-evm-sdk/sui/` | `cd examples/non-evm-sdk/sui && go test ./...` |

四个 module 的默认测试同时覆盖离线 transaction/capability 逻辑与真实 endpoint 的本地
contract test；testnet smoke 只有设置 endpoint 环境变量才运行，广播还需另行提供已签交易。

## 推荐顺序

能力矩阵 → Bitcoin → TRON/TRC20 → Solana/Cosmos/Sui/Aptos 原理 → 地址与归集 →
MPC 故障恢复 → 四条非 EVM Go 实战。
