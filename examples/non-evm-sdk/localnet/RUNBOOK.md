# 四链 localnet 故障与升级兼容门禁

本目录把两类测试明确分开：

1. **默认离线门禁**：只使用仓库内 N/N-1 schema fixture、`httptest` 和故障 `RoundTripper`。它不访问公网、不启动节点、不广播交易。
2. **真实 localnet 门禁**：显式构建并运行固定 commit 的节点二进制，通过固定 digest 的 Toxiproxy 注入网络故障。只有操作者主动执行对应 Make target 才会运行。

这套门禁验证的是 adapter 的 endpoint/schema/状态语义，以及相邻版本启动后的 API 兼容性。它不是共识、状态数据库迁移或主网升级的完整认证。

## 当前实跑证据

2026-07-17 在本机使用 manifest 固定、provenance 校验通过的 CometBFT 二进制完成：

- `0.38.22 → 0.38.23` 状态目录复用与 chain identity 保持；
- 节点停止期间 transport error、同版本重启与恢复；
- Toxiproxy latency、timeout、`reset_peer` 以及每次故障后的 identity/状态语义恢复。

Solana、Aptos、Sui 已通过 manifest、N/N-1 fixture、脚本语法、Compose 和四个 Go module
的 race/vet 门禁，但本机未构建这三套大型 Rust 节点二进制，因而尚无可声称的真实节点
启动、故障或升级证据。后续执行者必须保存 binary provenance 和 gate 输出，不能把
offline fixture/harness 通过写成“四链 localnet 实跑通过”。

## 版本锁

版本快照固定于 2026-07-17。每条链的 tag 和完整源码 commit 位于 `manifests/*.env`；annotated tag 记录的是 peeled commit，而不是 tag-object SHA。

| 链 | N-1 | N | 升级状态目录 |
| --- | --- | --- | --- |
| Solana / Agave | 3.1.14 | 4.0.3 | fresh |
| Cosmos / CometBFT | 0.38.22 | 0.38.23 | reuse |
| Aptos Node | 1.47.1 | 1.48.2 | reuse |
| Sui | 1.74.1 | 1.75.2 | reuse |

Cosmos 固定在当前 SDK 适配的 CometBFT 0.38 patch 线，不把 0.39/1.0 冒充成已支持版本。Solana 跨稳定线时使用新 ledger；官方本地验证器文档也建议切换版本后重置 ledger。Aptos、Sui、CometBFT 的 `reuse` 门禁会复用本地状态目录，启动失败即阻断升级。

CometBFT v0.38.23 tag 的源码 `TMCoreSemVer` 仍自报 `0.38.22`。manifest 因此同时记录 source version 与 reported version，节点真实性以 peeled commit 和二进制 SHA-256 为准；门禁不会把失真的自报字符串当作供应链身份。

Toxiproxy 固定为：

```text
ghcr.io/shopify/toxiproxy:2.12.0@sha256:9378ed52a28bc50edc1350f936f518f31fa95f0d15917d6eb40b8e376d1a214e
```

## 默认离线门禁

```bash
cd examples/non-evm-sdk/localnet
make manifests
make offline
```

`make offline` 强制使用 `GOPROXY=off GOSUMDB=off`。依赖未缓存时会失败，不会下载依赖。四个模块里的真实 localnet 测试只有在 `NON_EVM_LOCALNET=1` 时才会执行。

如需重新核验上游 tag 是否仍指向锁定 commit，显式执行网络检查：

```bash
make upstream-tags
```

每条链均检查：

- N-1 与 N 的数字字符串/JSON 数字及新增字段兼容；
- 破坏性字段类型或未知终态枚举被明确拒绝；
- timeout、TCP reset、超时 latency 返回 error，不能被分类成 `UNKNOWN`、`FAILED` 等链上状态；
- 错误 genesis hash、chain ID 或 chain identifier 必须 fail closed；
- 未观察到的合法交易 ID 只有在节点成功响应时才能得到 `UNKNOWN`。

fixtures 是按官方响应契约裁剪的最小 fixture，不宣称是本机实跑抓包。真实节点响应由下面的 opt-in 门禁独立验证。

## 构建固定节点二进制

构建会 clone 官方仓库并编译，必须显式允许网络：

```bash
LOCALNET_ALLOW_NETWORK=1 make build CHAIN=solana LANE=n-1
LOCALNET_ALLOW_NETWORK=1 make build CHAIN=solana LANE=n
```

对 `cosmos`、`aptos`、`sui` 重复执行。构建脚本会：

1. checkout manifest 中的完整 commit；
2. 编译指定节点/CLI；
3. 将二进制放到 `bin/<chain>/<lane>/`；
4. 生成包含二进制 SHA-256 的 `.provenance`；
5. 启动前重新核对 chain、lane、tag、commit 和二进制 SHA-256。

没有 provenance 时默认拒绝启动。`LOCALNET_ALLOW_UNVERIFIED_BINARY=1` 仅供临时排障，不应进入 CI 门禁。

构建资源提示：

- Agave、Aptos、Sui 是大型 Rust workspace，建议预留足够磁盘和内存。
- Aptos 只启动 node REST，关闭 faucet 与 transaction stream。
- Sui 的 `--with-graphql` 会同时启动 indexer 和 consistent store，需要 PostgreSQL/libpq 环境。
  fresh lane 会先执行 `sui genesis --working-dir <state>/config`，再以
  `sui start --network.config <state>/config` 启动；不能把
  `--force-regenesis` 与 `--network.config` 同时使用，固定版本源码会明确拒绝该组合。
- CometBFT 使用内置 `kvstore` ABCI，仅用于 RPC 兼容测试。

## 启动和基线门禁

```bash
make toxiproxy-up
make start CHAIN=cosmos LANE=n-1
make gate CHAIN=cosmos
```

节点只绑定本机端口，Toxiproxy 代理如下：

| 链 | 直接 endpoint | 代理 endpoint |
| --- | --- | --- |
| Solana | `127.0.0.1:8899` | `127.0.0.1:18899` |
| Cosmos | `127.0.0.1:26657` | `127.0.0.1:16657` |
| Aptos | `127.0.0.1:8080/v1` | `127.0.0.1:18080/v1` |
| Sui GraphQL | `127.0.0.1:9125/graphql` | `127.0.0.1:19125/graphql` |

第一次直接 probe 会捕获本地 genesis/chain identity；之后直接和代理 endpoint 都必须与该值一致。Cosmos 固定为 `sdk-compat-localnet`，Aptos localnet 固定为 chain ID `4`。

## 故障、重启与升级门禁

```bash
make faults CHAIN=cosmos
make restart-gate CHAIN=cosmos
make upgrade-gate CHAIN=cosmos
```

`faults` 顺序执行：

1. 无故障 readiness + identity + `UNKNOWN` 基线；
2. 1500ms downstream latency，客户端 250ms deadline；
3. downstream timeout，丢弃数据直到 toxic 被移除；
4. downstream `reset_peer`；
5. 每个故障后的 identity 与状态语义恢复。

`restart-gate` 在节点停止期间要求 adapter 返回 transport error，然后以同一版本和数据目录重启，并验证 identity 不变。

`upgrade-gate` 先运行 N-1 的实际 endpoint 门禁，再切换到 N。`reuse` 链必须保持 identity；`fresh` 链重新捕获可信 identity。最后再次运行该链的离线 N/N-1 fixture 门禁。

所有门禁只读取 readiness 和一个确定不存在的交易 ID，不调用 `Submit`、`BroadcastSync`、faucet 或任何签名接口。

## Sui GraphQL/gRPC 边界

Sui localnet 使用：

```bash
sui start --with-graphql=127.0.0.1:9125
```

adapter 门禁只访问 GraphQL。不会探测、回退或构造 deprecated JSON-RPC 请求。当前工作包不广播交易，因此不验证执行通道；生产执行通道应按官方迁移边界接入 gRPC，索引查询继续使用 GraphQL。

参考：

- [Sui JSON-RPC migration](https://docs.sui.io/develop/accessing-data/json-rpc-migration)
- [Sui local network](https://docs.sui.io/guides/developer/getting-started/local-network)
- [CometBFT v0.38 local node](https://docs.cometbft.com/v0.38/core/using-cometbft)
- [Solana local validator](https://solana.com/docs/toolkit/local-validator)
- [Aptos Core releases](https://github.com/aptos-labs/aptos-core/releases)
- [Toxiproxy](https://github.com/Shopify/toxiproxy)

## 清理

```bash
make stop CHAIN=cosmos
make toxiproxy-down
make clean CHAIN=cosmos
```

运行时数据、源码缓存和本地二进制都在本目录的忽略路径中，不会污染四个 Go module。
