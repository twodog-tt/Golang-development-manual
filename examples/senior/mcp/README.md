# MCP：身份、工具授权与传输故障夹具

在仓库根目录运行；需要 **Go 1.24.2+**。依赖为仓库固定的官方 `go-sdk v1.4.0`，无需模型 API、数据库或外部账号。

```bash
go test -race ./examples/senior/mcp/...
# stdout 只输出 MCP 协议；凭据由可信启动器配置，不来自模型参数。
MCP_DEMO_TOKEN=local-demo-secret go run ./examples/senior/mcp/
# 另一个终端可启动 HTTP 入口；默认仅监听 127.0.0.1:8088。
MCP_DEMO_TOKEN=local-demo-secret go run ./examples/senior/mcp/ -transport http
```

这是本地预置 bearer 凭据的资源服务器夹具，没有 OAuth 登录、令牌签发、JWKS 或 introspection 服务。
`/.well-known/oauth-protected-resource` 元数据 URL 位于 `/mcp` 下，声明的 `issuer.example.invalid` 是显式占位符；真实部署必须接入可信 issuer 和可访问的发现端点。
不能将本例称为 OAuth 端到端验收。示例只绑定 loopback，拒绝 `0.0.0.0`；无需更改线上部署。

stdio 将 `MCP_DEMO_TOKEN` 绑定为 `tenant-a/demo-user`；HTTP 每个请求都重新验证 bearer 的 audience、有效期和角色，并将会话绑定到 tenant+subject。`get_order` 再次核对工具 scope，按可信租户查询：

- `10001` 属于 tenant-a，返回 `amount_minor=9900`，表示 99.00 元；金额使用整数最小单位。
- `10002` 属于 tenant-b，与不存在的订单均返回 `IsError=true / order unavailable`。
- `tenant_id` 不是工具参数；模型附加这个字段会被 Schema 拒绝。
- `greet` 也需要 `greet` scope。`name=""` 输出 `Hello, friend (from Go MCP)`。

HTTP 最小手工握手：

```bash
curl -i http://127.0.0.1:8088/mcp \
  -H 'Authorization: Bearer local-demo-secret' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"manual","version":"1"}}}'
```

保存响应的 `Mcp-Session-Id`，随后发送 `notifications/initialized`，再调用 `tools/list`、`tools/call`；每个后续请求同时带 bearer、session ID 和 `MCP-Protocol-Version: 2025-11-25`。完整交互由测试自动执行。无 Origin 的非浏览器客户端可正常使用；存在 Origin 时必须精确匹配允许列表，非法值返回 403。

SDK 的默认客户端协议是 `2025-06-18`；内存测试使用该默认值，HTTP 和真实 stdio 检查显式请求 `2025-11-25`。这只证明本例的两个工具与所列传输路径，没有宣称 URL elicitation、sampling 或所有协议版本兼容。

| 验证 | 期望 |
|---|---|
| schemas / structuredContent / TextContent | Schema 存在，结构化值与兼容文本一致 |
| 缺失参数、错误类型、伪造租户、未知工具 | 协议错误，不执行查询 |
| 跨租户、scope 降权、错 audience、过期/撤销 | 拒绝，且不透露外部订单是否存在 |
| 后端阻塞 / 调用取消 | deadline 或 cancellation 到达后端；后端必须协作处理 context |
| 缺 bearer、恶意 Origin、盗用别人的 session | 分别返回 401/403；session ID 不能替代凭据 |
| session 空闲过期 / DELETE / 重建 | 旧 session 返回 404；重新 initialize 得到新 session |
| 真实 stdio 子进程 | 正确握手、输入拒绝、输出可解析、EOF 正常退出 |

HTTP 的会话恢复不等于业务重试。带副作用的工具在回包丢失后仍需独立 intent/receipt 对账；参见 [A2A 恢复夹具](../a2a/README.md)。

统一验证（含真实 stdio 与 A2A CLI）：

```bash
bash scripts/verify_reliability_examples.sh --offline
```
