# RAG / Memory / HTTP SSE / 缓存成本综合演示

要求 Go 1.24.2+ 与 Linux/macOS；仓库根目录：

```bash
go run ./examples/senior/knowledge-demo
go test -race ./examples/senior/knowledge-demo/...
bash scripts/verify_reliability_examples.sh --offline
```

所有步骤自动执行并检查不变量，失败退出非零；临时目录在退出时清理，不修改用户数据、不调用真实模型、不需要密钥/Docker。
同一可信授权夹具串起：

1. 中文退款资料正分召回；火星问题无证据拒答，跨租户拒绝。
2. 写作者创建candidate，独立reviewer批准，operator启用；索引/缓存同步。
3. 删除主记录后重开Store，旧历史Ref立即失效；投影ACK丢失，再次同步恢复。
4. loopback HTTP首次429明确拒绝，第二次发送文本SSE及终止用量；下一次FAQ缓存命中绕过模型。
5. 按attempt聚合已知成本22微美元（仅教学价格），HTTPAttempts=2，缓存命中不增加请求数。

预期JSON布尔检查全部为true，`http_attempts=2`、`costs.Attempts=2`、`costs.Failed=1`、`costs.Unknown=0`、`costs.KnownMicroUSD=22`。
Failed=1是已证明未生成的429；无usage断流的Unknown故障在llmclient/llmops单测中独立验证。

[RAG](../rag/README.md) · [Memory](../agentmemory/README.md) · [HTTP/SSE](../llmclient/README.md) · [缓存/成本](../llmops/README.md)

综合演示使用夹具数据与固定答案，四篇正文补的是限定行为证据，不代表生产IAM、向量库、分布式索引、真实模型质量或商业API验收。
