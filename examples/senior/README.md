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

## AI 工程（S-AI）

| 题 ID | 目录 | 说明 | 测试 |
|-------|------|------|------|
| S-AI-01 | [llmclient/](llmclient/) | 流式 LLM Client Mock | `go test ./llmclient/...` |
| S-AI-02 | [rag/](rag/) | 简易 RAG（分块 + 检索） | `go test ./rag/...` |
| S-AI-07 | [mcp/](mcp/) | MCP Server（stdio） | `go test ./mcp/...` · `go run ./mcp/` |
| S-BC-02 | [ethrpc/](ethrpc/) | 以太坊 JSON-RPC 客户端 | `go test ./ethrpc/...` |
| S-BC-09 | [erc20bind/](erc20bind/) | abigen + simulated 部署转账 | `go test ./erc20bind/...` |

```bash
cd examples/senior
go test ./...
```
