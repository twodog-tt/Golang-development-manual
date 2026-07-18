# Senior 面试手写题示例

对应 `docs/interview/08-coding-senior/` 与 `docs/interview/10-ai-engineering/`。

## 手写题（S-CODE）

| 题 ID | 目录 | 说明 | 测试 |
|-------|------|------|------|
| S-CODE-01 | [lru/](lru/) | 并发安全 LRU | `go test ./lru/...` |
| S-CODE-02 | [ratelimit/](ratelimit/) | 令牌桶限流 | `go test ./ratelimit/...` |
| S-CODE-03 | [graceful_shutdown/](graceful_shutdown/) | HTTP 优雅关闭 | `go run ./graceful_shutdown/` |
| S-CODE-04 | [errgroup/](errgroup/) | errgroup 语义 | `go test ./errgroup/...` |
| S-CODE-05 | [connpool/](connpool/) | channel 连接池 | `go test ./connpool/...` |
| S-CODE-06 | [singleflightcache/](singleflightcache/) | 同 key 请求合并与缓存击穿治理 | `go test -race ./singleflightcache/...` |
| S-CODE-07 | [batchexec/](batchexec/) | 有界并发、取消与保序 | `go test -race ./batchexec/...` |

## AI 工程（S-AI）

| 题 ID | 目录 | 说明 | 测试 |
|-------|------|------|------|
| S-AI-01 | [llmclient/](llmclient/) | 流式 LLM Client Mock | `go test ./llmclient/...` |
| S-AI-02 | [rag/](rag/) | 简易 RAG（分块 + 检索） | `go test ./rag/...` |
| S-AI-07 | [mcp/](mcp/) | MCP Server（stdio） | `go test ./mcp/...` · `go run ./mcp/` |
| S-BC-02 | [ethrpc/](ethrpc/) | 以太坊 JSON-RPC 客户端 | `go test ./ethrpc/...` |
| S-BC-09 | [erc20bind/](erc20bind/) | abigen + simulated 部署转账 | `go test ./erc20bind/...` |
| S-WALLET-02 | [coinselect/](coinselect/) | UTXO 选择、vbytes fee 与 dust | `go test ./coinselect/...` |
| S-PAY-01 | [paymentstate/](paymentstate/) | 支付状态机、重复事件与冲正 | `go test ./paymentstate/...` |
| S-NODE-02 | [rpcpool/](rpcpool/) | Hedged read 与取消 | `go test -race ./rpcpool/...` |

## 交易所与跨链安全

| 题 ID | 目录 | 说明 | 测试 |
|-------|------|------|------|
| S-EXCH-17 | [matchingengine/](matchingengine/) | 确定性价格时间优先撮合、FOK/Post-only/STP | `go test -race ./matchingengine/...` |
| S-EXCH-18 | [walreplay/](walreplay/) | Framed WAL、快照与崩溃回放 | `go test -race ./walreplay/...` |
| S-EXCH-19 | [marketdatarecovery/](marketdatarecovery/) | Snapshot + delta 桥接与 gap fail closed | `go test -race ./marketdatarecovery/...` |
| S-EXCH-20 | [fixsession/](fixsession/) | FIX sequence、重传与 Gap Fill | `go test -race ./fixsession/...` |
| S-EXCH-21 | [matchingengine/](matchingengine/) | STP cancel-maker/taker/both 与确定性事件 | `go test -race ./matchingengine/...` |
| S-EXCH-22 | [callauction/](callauction/) | 集合竞价清算价与确定性分配 | `go test ./callauction/...` |
| S-BC-12 | [bridgeguard/](bridgeguard/) | 跨链 route/payload 绑定、重放与敞口限制 | `go test -race ./bridgeguard/...` |

## 安全、链数据与非 EVM 在线可靠性

| 题 ID | 目录 | 说明 | 测试 |
|-------|------|------|------|
| S-SEC-02 | [signerfencing/](signerfencing/) | signer-side epoch、owner、policy/intent 与幂等 fencing | `go test -race ./signerfencing/...` |
| S-NODE-07 | [chainmerge/](chainmerge/) | hash lineage、overlap、reorg 与 finalized guard | `go test -race ./chainmerge/...` |
| S-NODE-09 | [txlifecycle/](txlifecycle/) | UNKNOWN、同 bytes 重播、链证据与 provider 分歧 | `go test -race ./txlifecycle/...` |

非 EVM SDK 示例为独立 Go module，见
[`examples/non-evm-sdk/`](../non-evm-sdk/)；根模块的 `go test ./...` 不会自动进入这些目录。

```bash
cd examples/senior
go test ./...
```
