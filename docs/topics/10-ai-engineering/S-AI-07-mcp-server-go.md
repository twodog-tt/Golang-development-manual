---
id: S-AI-07
title: Go 实现 MCP Server：工具暴露与 stdio/HTTP 部署
module: ai-engineering
level: senior
frequency: 5
go_version: "1.24.2+"
tags: [mcp, model-context-protocol, tools, stdio, go-sdk]
status: published
code_refs: [examples/senior/mcp]
sources:
  - https://modelcontextprotocol.io/specification/2025-11-25/basic/transports
  - https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization
  - https://github.com/modelcontextprotocol/go-sdk
  - https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp
---

# Go 实现 MCP Server：工具暴露与 stdio/HTTP 部署

<a id="oral-card"></a>

## 要点卡

[返回 P0 知识图谱](../_meta/p0-knowledge-graph.md)

!!! abstract "30 秒回答"

    MCP 用 JSON-RPC 生命周期和 capability negotiation 标准化宿主与工具、资源、提示等服务的连接。
    Go 可用官方 SDK；stdio 适合宿主拉起的本地进程，stdout 必须保持纯协议。远程 Streamable HTTP
    使用单一 endpoint 的 POST/GET 与可选 SSE，还要做协议版本协商、认证、token audience、
    Origin 校验、会话/超时和逐工具授权，不能把 JSON Schema 当权限边界。

**3 分钟展开**

1. 客户端先 `initialize` 协商协议版本与 capabilities，再进入正常请求；服务端不能假定所有客户端支持同一特性。
2. Tool schema 只描述输入输出；执行时仍从可信身份解析 tenant/resource scope，校验业务状态、预算、幂等和高风险审批。
3. stdio 从宿主/环境获取凭证并只向 stderr 记录日志；远程 HTTP 遵循 OAuth 资源服务器语义，校验 access token 是否发给本服务，禁止 token passthrough。
4. 限制返回体、并发、执行时间和取消传播；能力或协议升级要做兼容测试。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 先协商再调用；schema 不授权；远程 token 必须绑定本资源且不得透传下游 |
| 手画图 | `host/client ⇄ MCP transport ⇄ Go server → policy → internal API` |
| 项目落点 | 将 OctoAgentFlow 的最小工具注册表、执行超时、审计和本仓库 Go MCP 示例串起来讲 |
| 一个取舍 | MCP 提高跨宿主复用性，但增加协议兼容、远程身份与工具治理成本 |

**错误表达**

- ❌ “MCP 就是把 Function Calling 包一层 HTTP；有 schema 就能安全执行。”
- ✅ “MCP 是宿主到服务的有状态协议边界；协议连通、输入结构和业务授权是三件事。”

**自测追问**：stdio 与 Streamable HTTP 的凭证、日志、Origin 和会话边界分别有什么不同？

## 10 分钟版（原理 + 图示）

```mermaid
flowchart LR
  Host[Agent Host] -->|stdio / Streamable HTTP| MCP[Go MCP Server]
  MCP --> Tools[Tool: get_order / grep_logs]
  Tools --> API[内部 HTTP / DB]
```

**官方 SDK Tool handler（与本仓库 `greet` 返回契约一致）**

```go
type greetInput struct {
    Name string `json:"name" jsonschema:"要问候的人名"`
}
type greetOutput struct {
    Greeting string `json:"greeting"`
}

func greet(_ context.Context, _ *mcp.CallToolRequest, in greetInput) (
    *mcp.CallToolResult, greetOutput, error,
) {
    name := in.Name
    if name == "" {
        name = "friend"
    }
    return nil, greetOutput{Greeting: fmt.Sprintf("Hello, %s (from Go MCP)", name)}, nil
}
```

这只是 handler 片段，不能直接当完整可执行程序。本例通过 `server.New(Config{Authority: a, LocalToken: token})`
注册带鉴权包装的工具；`greet` 要求 `greet` scope，`get_order` 要求 `orders:read`。
stdio 的 token 由可信启动器配置，HTTP 的 token 来自每次请求的 Authorization header，均在工具执行前重新验证；
身份不能从模型参数或 `_meta` 读取。完整程序及运行命令见下方。

**传输方式选型**

| 传输 | 场景 |
|------|------|
| Stdio | 宿主拉起的本地子进程；stdout 只承载协议 |
| Streamable HTTP | 远程多客户端；单一 MCP endpoint 使用 POST/GET，可选 SSE |
| InMemory | 单测（`mcp.NewInMemoryTransports()`） |

**Cursor 挂载示例（stdio）**

```json
{
  "mcpServers": {
    "golang-manual": {
      "command": "go",
      "args": ["run", "./examples/senior/mcp/"],
      "cwd": "/path/to/Golang-development-manual",
      "env": {"MCP_DEMO_TOKEN": "local-demo-secret"}
    }
  }
}
```

## 生产场景

- **内部运维 Copilot**：`query_metrics`、`search_logs` Tool 接 Prometheus/Loki
- **业务 Agent**：`create_ticket` 只读/写分离；写操作 Tool 内二次鉴权
- **多 Tool 聚合**：一个 MCP Server 注册 10+ 小粒度 Tool，优于一个大而全 SQL Tool

## 排查与工具

- MCP Inspector / `mcp` CLI 调试 stdio
- 日志：**不要**写 stdout（破坏 JSON-RPC），用 stderr 或 slog
- 单测：`NewInMemoryTransports` + `client.Connect` + `CallTool`

## 架构取舍

| 方案 | 适用 |
|------|------|
| 官方 go-sdk | 跟 spec 最快，推荐新项目 |
| 每个能力一个 MCP Server | 权限隔离清晰 |
| MCP + 传统 REST 并存 | MCP 给 Agent，REST 给前端 |

**何时不用 MCP**：固定 UI 工作流、强表单校验场景 — 直接 API 更简单。

## 深挖问答

1. **和 Function Calling 区别？** → FC 在模型 API 内；MCP 是 **宿主↔服务** 标准协议，可跨进程/跨语言。
2. **Tool 返回什么？** → 结构化 JSON（`AddTool` Out 类型）或 `TextContent`；工具执行失败用 `IsError=true`；协议/参数错误与业务结果分开处理。本例订单不可见和不存在统一返回 `order unavailable`。
3. **如何做鉴权？** → HTTP 的 OAuth profile 按 MCP authorization 实现资源服务器；mTLS 可作为额外部署保护，不能替代 OAuth 发现/授权语义。校验 token audience/resource 与 Origin 防 DNS rebinding；本地 HTTP 只绑定 loopback。stdio 从环境/宿主取得凭证，但 Tool 内仍按最终用户与资源做授权。
4. **资源 Resources 与 Tool？** → Resource 只读暴露文件/URI；Tool 可执行副作用。

## 反模式与事故

- **stdout 打日志** → 协议损坏，IDE 连不上
- **一个 Tool 执行任意 SQL** → 注入与越权
- **无超时** → Tool 内 HTTP 挂死拖垮 Agent
- **返回巨型 JSON** → 撑爆 Agent context
- Streamable HTTP 对任意 Origin 开放或本地监听 `0.0.0.0` 且无认证 → DNS rebinding/越权调用风险
- 把客户端给 MCP Server 的 access token 原样透传给下游 API → 混淆资源受众并扩大凭证泄露半径

## 代码示例

需要 **Go 1.24.2+**，依赖固定为官方 `go-sdk v1.4.0`。在仓库根目录运行：

```bash
go test -race ./examples/senior/mcp/...
MCP_DEMO_TOKEN=local-demo-secret go run ./examples/senior/mcp/
MCP_DEMO_TOKEN=local-demo-secret go run ./examples/senior/mcp/ -transport http
# 包括真实 stdio 子进程、A2A CLI 和已有 Agent 恢复门禁
bash scripts/verify_reliability_examples.sh --offline
```

HTTP 默认仅监听 `127.0.0.1:8088`，拒绝非 loopback 地址。每个请求校验 audience、有效期与当前凭据，
存在 Origin 时执行精确允许列表；会话绑定 tenant+subject，不能凭 session ID 跳过认证。空闲会话过期返回 404，
客户端重新 initialize，不能因此自动重放写工具。处理器给后端传播超时/取消，后端必须协作处理 context。

实现见 [完整运行与故障矩阵](https://github.com/twodog-tt/Golang-development-manual/tree/master/examples/senior/mcp)。两个演示工具为 `greet`、`get_order`；
订单按受信租户隔离，`10001` 属于 tenant-a，`10002` 属于 tenant-b。返回金额为整数最小单位
`amount_minor=9900`，不使用浮点金额。读不到与不存在都返回相同的工具执行错误，避免订单存在性泄露。

| 故障验证 | 期望 |
|---|---|
| 输入缺失/类型错误、模型附加 tenant_id、未知工具 | 拒绝，不进入查询 |
| 跨租户、scope 降权、过期/错 audience/撤销 token | 拒绝，已有 session 同样检查 |
| 输出 Schema、structuredContent 与兼容 TextContent | Schema 存在，返回值一致；精确检查 greet 默认名字 |
| 后端超时、客户端取消 | context 传到后端，无无限等待 |
| 缺认证、恶意 Origin、会话盗用、会话过期/重建 | 401/403/404；重新握手成功 |
| 真实 stdio 进程 | 明确版本握手、纯协议 stdout、EOF 正常退出 |

**证据边界**：bearer 身份表和订单为本地夹具；没有 OAuth 登录/签发/JWKS/introspection 服务。
资源元数据中的 `issuer.example.invalid` 是占位符，不代表授权服务器可用。内存测试使用 SDK 默认 `2025-06-18`，
HTTP 和真实 stdio 检查显式请求 `2025-11-25`；没有将其扩大为所有版本、sampling/elicitation 或真实宿主的兼容验收。
带副作用调用的 UNKNOWN 与 receipt 恢复见 [MCP/A2A 实战](./S-AI-11-mcp-a2a-vendor-neutral-interoperability.md)。

## 延伸阅读

- [Model Context Protocol](https://modelcontextprotocol.io/)
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)
