# 非 EVM Go SDK：offline signing + endpoint adapters

每个目录都是独立 Go module，用于隔离链 SDK 的依赖图与版本约束。原有离线构建、签名与
reservation 示例均保留；新增 adapter 只负责把已经签名的 bytes 发送到真实 endpoint，并把
查询证据归一化。根目录执行 `go test ./...` 不会自动进入这些嵌套 module。

统一故障状态机位于 `examples/senior/txlifecycle/`。adapter 不负责自动重签、换 sequence、换
recent blockhash、替换 Sui object ref，也不会把 provider 未命中误判成链上结果。

| 链 | 离线实现 | 在线接口 |
|----|----------|----------|
| Solana | 社区 `gagliardetto/solana-go` system transfer | HTTP JSON-RPC：`getHealth`、`getGenesisHash`、`sendTransaction`、`getSignatureStatuses`、`isBlockhashValid` |
| Cosmos | 官方 Cosmos SDK v0.53.7、`TxBuilder`、`SIGN_MODE_DIRECT` | CometBFT JSON-RPC：`status`、`broadcast_tx_sync`、`tx` |
| Aptos | Aptos Labs 官方 SDK v1.13.0、BCS/签名 | Fullnode REST：`GET /v1`、`POST /v1/transactions`、`GET /v1/transactions/by_hash/{hash}` |
| Sui | capability-aware object/balance reservation；不虚构 Go transaction builder | 当前 GraphQL：`Query.transaction`、`Mutation.executeTransaction`；无 deprecated JSON-RPC fallback |

## 统一状态语义

| 状态 | 所需证据 |
|------|----------|
| `UNKNOWN` | 查询未命中且没有 admission 或 expiration 证据。Solana signature 为 `null` 且未提供 recent blockhash；Cosmos `tx` 明确 not found；Aptos `404 + transaction_not_found`；Sui `transaction: null`。 |
| `PENDING` | 已 accepted/mempool，或有仍有效的待执行证据。它不是成功。 |
| `REJECTED` | 预执行/准入检查明确拒绝，例如 Cosmos `CheckTx code!=0`。它不声称交易已 committed execution，因此不同于 `FAILED`。 |
| `SUCCEEDED` | 有成功执行证据，并达到调用方要求的 finality/commitment。 |
| `FAILED` | 有失败执行证据，并达到调用方要求的 finality/commitment。成功与失败必须对称等待相同 finality。 |
| `EXPIRED` | adapter 有资源过期的肯定证据；当前示例仅在 Solana signature 未命中且显式传入的 recent blockhash 已无效时返回。not found 本身绝不等于 expired。 |

链特有边界：

- Solana `sendTransaction` 返回 signature 仅表示 RPC 接收。`processed + err` 在要求
  `confirmed` 时仍是 `PENDING`；达到 required commitment 后才按 `err` 分成
  `SUCCEEDED`/`FAILED`。recent blockhash 有效性使用独立 commitment 查询，不与交易确认
  commitment 混为一谈。发送前会解析并验证本地签名，RPC 返回值还必须等于 wire bytes
  内嵌的 first signature/tx id。
- Cosmos `broadcast_tx_sync(code=0)` 只表示 `CheckTx` 通过并进入该节点 mempool，归一化为
  `PENDING`；`CheckTx code!=0` 是 `REJECTED`，两者都不能称为 committed execution。只有后续
  `tx` 返回的 committed `tx_result.code` 才归一化为 `SUCCEEDED`/`FAILED`。广播响应 hash
  必须等于 exact TxRaw bytes 的 SHA-256。
- Aptos `pending_transaction` 是 `PENDING`；已执行交易严格读取 `success` 和 `vm_status`。
  仅明确的 `404 + error_code=transaction_not_found` 映射为 `UNKNOWN`；其他 404、限流和
  provider 错误保留为结构化 `RESTError`。提交前会反序列化 BCS、验证 authenticator，并
  校验 endpoint 返回 hash 与本地 domain-separated transaction hash 一致。
- Sui `Query.transaction` 返回 `null` 是 `UNKNOWN`，可能是索引延迟或从未提交。
  `ExecutionStatus` 仅按 `SUCCESS`/`FAILURE` 处理，失败详情来自
  `effects.executionError.message`。`Mutation.executeTransaction` 的交易 ID **只取**
  `effects.transaction.digest`；`effects.digest` 是 effects digest，不能混用。GraphQL 顶层
  `errors` 作为请求/入口错误返回。

## 确定性 contract tests

默认测试全部使用 `httptest`，验证 HTTP method/path/media type、JSON-RPC/GraphQL body、
结构化错误以及 `UNKNOWN/PENDING/REJECTED/SUCCEEDED/FAILED/EXPIRED` 语义，不访问公网：

```bash
(cd solana && go test ./...)
(cd cosmos && GOMAXPROCS=2 go test -p=1 ./...)
(cd aptos && go test ./...)
(cd sui && go test ./...)
```

## Opt-in 只读 smoke tests

只设置 endpoint URL 只会执行 readiness/链身份读取，不会广播、不调用 faucet、不花 gas。
`*_EXPECTED_*` 可选，但 CI/生产探测强烈建议设置；否则 HTTP 健康的错误网络仍可能被接受。

### Solana

```bash
cd solana

# local validator
SOLANA_RPC_URL=http://127.0.0.1:8899 \
SOLANA_EXPECTED_GENESIS_HASH='<trusted-local-genesis-hash>' \
go test -run '^TestSmokeReadOnly$' -v

# public devnet（公共 endpoint 可能限流）
SOLANA_RPC_URL=https://api.devnet.solana.com \
SOLANA_EXPECTED_GENESIS_HASH=EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG \
go test -run '^TestSmokeReadOnly$' -v
```

### Cosmos / CometBFT

Cosmos testnet endpoint 必须按目标链选择；不要从 chain registry/provider 列表取到 URL 后只看
HTTP 200。测试会读取 `node_info.network`，设置预期 chain ID 可阻止连到返回 `provider` 等错误
身份的节点。

```bash
cd cosmos

COSMOS_RPC_URL=http://127.0.0.1:26657 \
COSMOS_EXPECTED_CHAIN_ID='<local-genesis-chain-id>' \
GOMAXPROCS=2 go test -p=1 -run '^TestSmokeReadOnly$' -v

COSMOS_RPC_URL=https://rpc.osmotest5.osmosis.zone \
COSMOS_EXPECTED_CHAIN_ID=osmo-test-5 \
GOMAXPROCS=2 go test -p=1 -run '^TestSmokeReadOnly$' -v
```

### Aptos

URL 必须包含 `/v1`。

```bash
cd aptos

APTOS_REST_URL=http://127.0.0.1:8080/v1 \
APTOS_EXPECTED_CHAIN_ID='<trusted-local-chain-id>' \
go test -run '^TestSmokeReadOnly$' -v

APTOS_REST_URL=https://api.testnet.aptoslabs.com/v1 \
APTOS_EXPECTED_CHAIN_ID=2 \
go test -run '^TestSmokeReadOnly$' -v
```

### Sui GraphQL

localnet 必须运行当前 GraphQL + indexer 服务；不要把旧 fullnode JSON-RPC URL 填进来。

```bash
cd sui

SUI_GRAPHQL_URL=http://127.0.0.1:<graphql-port>/graphql \
SUI_EXPECTED_CHAIN_IDENTIFIER='<trusted-local-chain-identifier>' \
go test -run '^TestSmokeReadOnly$' -v

SUI_GRAPHQL_URL=https://graphql.testnet.sui.io/graphql \
SUI_EXPECTED_CHAIN_IDENTIFIER=69WiPg3DAQiwdxfncX6wYQ2siKwAe6L9BZthQea3JNMD \
go test -run '^TestSmokeReadOnly$' -v
```

截至 2026-07，Sui JSON-RPC 已进入停用窗口；本示例只使用官方现行 GraphQL。官方公共
GraphQL endpoint 仅适合开发且有限流，生产应自建 GraphQL/indexer/fullnode 组合或使用提供
GraphQL SLA 的专用 provider。

## 显式广播 smoke tests

广播测试除 URL 外还要求单独的 signed transaction 环境变量，因此仅配置 URL 时永远不会
花 gas。它们不构建、不签名、不充值，也不依赖 faucet：

```bash
# Solana：完整 signed transaction wire bytes 的标准 base64
cd solana
SOLANA_RPC_URL='<rpc-url>' \
SOLANA_SIGNED_TX_BASE64='<base64-signed-transaction>' \
go test -run '^TestSmokeBroadcastSignedTransaction$' -v

# Cosmos：TxRaw bytes 的标准 base64
cd ../cosmos
COSMOS_RPC_URL='<comet-rpc-url>' \
COSMOS_SIGNED_TX_BASE64='<base64-signed-tx-bytes>' \
GOMAXPROCS=2 go test -p=1 -run '^TestSmokeBroadcastSignedTransaction$' -v

# Aptos：SignedTransaction BCS bytes 的标准 base64
cd ../aptos
APTOS_REST_URL='<rest-v1-url>' \
APTOS_SIGNED_TX_BCS_BASE64='<base64-signed-bcs>' \
go test -run '^TestSmokeBroadcastSignedTransaction$' -v

# Sui：TransactionData BCS 与每个 serialized signature 都是标准 base64
cd ../sui
SUI_GRAPHQL_URL='<graphql-url>' \
SUI_SIGNED_TRANSACTION_JSON='{"transactionDataBcs":"<base64>","signatures":["<base64>"]}' \
go test -run '^TestSmokeBroadcastSignedTransaction$' -v
```

安全边界：只在 disposable localnet/testnet 账户上使用最小余额；signed bytes 可能仍代表真实
资产转移，重复运行可能重播或产生第二次执行风险。不要把私钥、seed 或 mnemonic 放进这些
环境变量。广播返回后仍须按 tx id 查询并应用统一状态机；任何 `UNKNOWN` 都只允许查询或重播
完全相同的 signed bytes，不能据此立即重建另一笔交易。

## 版本与官方接口说明

- Solana Go SDK 是社区实现，生产需固定版本并与官方工具做 golden vector 交叉验证。在线
  adapter 使用 [Solana HTTP RPC](https://solana.com/docs/rpc/http)。
- CometBFT adapter 遵循 [v0.38 RPC spec](https://docs.cometbft.com/v0.38/spec/rpc/)。
- Aptos 官方仓库建议新项目评估 v2；当前 `v2` 要求的 Go 版本高于本示例 Go 1.24，因此离线
  示例固定 v1.13.0。在线 adapter 使用稳定 Fullnode REST BCS 提交接口。
- Sui 使用当前 [GraphQL RPC](https://docs.sui.io/develop/accessing-data/graphql/graphql-rpc)。
  官方迁移映射中，读取对应 `LedgerService.GetTransaction` / GraphQL `Query.transaction`，执行
  对应 `TransactionExecutionService.ExecuteTransaction` / GraphQL
  `Mutation.executeTransaction`；本 module 选择无需 protobuf 依赖的 GraphQL 实现。
