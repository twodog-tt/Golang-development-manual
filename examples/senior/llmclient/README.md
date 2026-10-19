# 本地 HTTP / SSE 模型接入夹具

对应 S-AI-01。Go 1.24.2+，标准库，真实 loopback HTTP，不调用外部模型、不需要密钥。
仓库根目录运行：

```bash
go test -race ./examples/senior/llmclient/...
go run ./examples/senior/knowledge-demo
```

`Client.Complete(ctx, []Message)` 返回 `(string, Usage, error)`；`StreamChat(ctx, []Message, StreamHandler)` 返回 `(Usage, error)`。
`MockClient` 按空格分词，仅验证调用编排，`Usage{Known:false, Source:"mock-word-count"}` 不可作账单。
`NewLocalHTTP(url, attempts, observer)` 只允许 HTTP + 字面量 loopback IP，禁用代理、重定向，调用后 `Close` 释放空闲连接。

## 固定文本协议

本地 POST JSON 为 `{"contract":"local-text-sse-v1","messages":[{"Role":"user","Content":"hi"}],"stream":true}`。
响应 `Content-Type: text/event-stream`，支持 `response.created`、`response.output_text.delta` 和 `response.completed`：

```text
data: {"type":"response.output_text.delta","delta":"hello"}

data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":10,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}

```

每帧必须以空行结束。无显式 completed 的 EOF 为错误；failed/incomplete/error 也不成功。
借用 Responses 文本事件语义，但请求格式、可接受事件集和错误恢复是本地契约，不能把 URL 改为真实 provider 就称已接通完整 API。
工具事件直接拒绝，不触发任何工具。需要真实厂商时另接锁定版本的官方 SDK 和契约测试。

| 测试 | 验证 |
|---|---|
| `TestSSEFramingAndUsage` | LF/CRLF/CR、注释/BOM、多行 data、cached input 是 input 子集 |
| `TestExplicitCompletionAndFailuresNeverRetry` | 断流、失败、错误终止状态/JSON/usage、工具、过大帧、事件名不一致，不重试 |
| `TestSafeRejectionRetriesAndAmbiguousErrorsDoNot` | 仅本地 429 + `X-Fixture-Rejected: before-generation` 可重试；未知429/500/重定向不跟随 |
| `TestDeadlineCallbackAndCancel` | Retry-After 等待受 deadline 限制，下游失败/取消真实关闭上游连接 |
| `TestInputAndStreamBudgets` | 64 KiB/64条输入、nil callback和1 MiB总流边界 |
| `TestUnacceptedConnectionAndInvalidResponsesDoNotRetry` | 真实断连、错误Content-Type、无空行终止、负/缺用量、非法Retry-After |
| `TestUnknownUsageAndEndpointLimits` | 缺 usage 保持未知，非 loopback/凭据 URL 拒绝；Retry-After 秒数/日期 |

输入最多 64 KiB、64 条消息；单行/帧最多 64 KiB、总流最多 1 MiB；总 deadline 最多 5 秒、等待响应头最多 2 秒。
最多 1～5 attempts；省略 Retry-After 时有限指数退避+jitter。网络故障/5xx/已接受后的断流默认不重试，保留 request ID 与 partial 状态。
Complete 出错不给出部分答案；StreamChat 已消费的 delta 无法撤回，调用者须以 error 明确标记未完成。
每个 HTTP attempt 各产生一次 observer 事件（同步回调，调用者负责并发安全），费用 unknown 不能当成零。
