# 10 AI 工程与编程

15 篇 | Agent 控制面核心阅读 | [返回专题索引](../../topic-catalog.md) · [领域能力优先级](../_meta/role-priority-matrix.md)

> 面向 **AI / Crypto Agent 控制面与 Go 后端** 的工程专题：模型接入、可恢复工作流、HITL、
> Memory、MCP/A2A、身份/Commerce、开放平台与 Web3 执行边界（非算法研究向）。
> 先读概念地图：[Agent 控制面](../../maps/agent-control-plane.md)。

| ID | 标题 | 频率 |
|----|------|------|
| [S-AI-01](./S-AI-01-llm-api-integration.md) | Go 接入大模型 API：流式、重试、超时 | ⭐⭐⭐⭐⭐ |
| [S-AI-02](./S-AI-02-rag-architecture.md) | RAG 架构：分块、向量检索与 Go 落地 | ⭐⭐⭐⭐⭐ |
| [S-AI-03](./S-AI-03-agent-tool-calling.md) | AI Agent 与 Function Calling | ⭐⭐⭐⭐⭐ |
| [S-AI-04](./S-AI-04-prompt-context.md) | Prompt 工程与 Context 窗口管理 | ⭐⭐⭐⭐ |
| [S-AI-05](./S-AI-05-llm-security.md) | LLM 应用安全：注入、PII、护栏 | ⭐⭐⭐⭐⭐ |
| [S-AI-06](./S-AI-06-llm-observability-cost.md) | LLM 可观测性、成本与延迟优化 | ⭐⭐⭐⭐ |
| [S-AI-07](./S-AI-07-mcp-server-go.md) | Go 实现 MCP Server | ⭐⭐⭐⭐⭐ |
| [S-AI-08](./S-AI-08-multimodal-voice.md) | 多模态与语音接入 | ⭐⭐⭐⭐ |
| [S-AI-09](./S-AI-09-agent-workflow-hitl-publishing.md) | Agent 工作流、HITL 与可靠发布控制面 | ⭐⭐⭐⭐⭐ |
| [S-AI-10](./S-AI-10-persona-memory-feedback-governance.md) | Persona、分层 Memory 与反馈学习治理 | ⭐⭐⭐⭐⭐ |
| [S-AI-11](./S-AI-11-mcp-a2a-vendor-neutral-interoperability.md) | MCP 与 A2A：跨框架互操作 | ⭐⭐⭐⭐⭐ |
| [S-AI-12](./S-AI-12-erc8004-agent-identity-reputation-validation.md) | ERC-8004：Agent 身份、信誉与验证 | ⭐⭐⭐⭐⭐ |
| [S-AI-13](./S-AI-13-x402-erc8183-agent-commerce.md) | x402/x402b/ERC-8183：Agent Commerce | ⭐⭐⭐⭐⭐ |
| [S-AI-14](./S-AI-14-crypto-agent-open-platform-marketplace-launchpad.md) | Crypto Agent 开放平台与 Launchpad | ⭐⭐⭐⭐⭐ |
| [S-AI-15](./S-AI-15-agent-evaluation-release-gates.md) | Agent 评测、任务轨迹与发布回归门禁 | ⭐⭐⭐⭐⭐ |

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-AI-01 | `examples/senior/llmclient/` | `go test -race ./examples/senior/llmclient/...` |
| S-AI-02 | `examples/senior/rag/` | `go test -race ./examples/senior/rag/...` |
| S-AI-03 / 05 / 09 | `examples/senior/agentworkflow/` | `go test -race ./examples/senior/agentworkflow/...` · `go run ./examples/senior/agentworkflow/cmd/demo -dir "$(mktemp -d)"` |
| S-AI-04 | `examples/senior/promptcontext/` | `go test -race ./examples/senior/promptcontext/...` · `go run ./examples/senior/promptcontext/cmd/demo` |
| S-AI-06 | `examples/senior/llmops/` | `go test -race ./examples/senior/llmops/...` |
| S-AI-07 | `examples/senior/mcp/` | `go test -race ./examples/senior/mcp/...` · `MCP_DEMO_TOKEN=local-demo-secret go run ./examples/senior/mcp/` |
| S-AI-08 | `examples/senior/mediaio/` | `go test -race -timeout 30s ./examples/senior/mediaio/...` · `go run ./examples/senior/mediaio/cmd/demo` |
| S-AI-10 | `examples/senior/agentmemory/` | `go test -race ./examples/senior/agentmemory/...` |
| S-AI-11 | `examples/senior/a2a/` | `go test -race ./examples/senior/a2a/...` · `go run ./examples/senior/a2a/cmd/demo -dir "$(mktemp -d)" -step demo` |
| S-AI-15 | `examples/senior/agenteval/` | `go test -race ./examples/senior/agenteval/...` · `go run ./examples/senior/agenteval/cmd/eval` |

S-AI-01/02/06/10 的综合演示：`go run ./examples/senior/knowledge-demo`；全部离线故障门禁：
`bash scripts/verify_reliability_examples.sh --offline`。要求 Go 1.24.2+，持久化示例使用 Linux/macOS 文件锁；
所有 token、模型、审批者和发布效果均为本地夹具，MCP stdio 命令会等待客户端输入。

## 适用场景

- 场景涉及 **Agent 工作流 / MCP / A2A / Agent SDK / Agent Commerce**
- 需要讲清「如何暂停恢复」「审批后如何防重复执行」「Memory 如何隔离与学习」
- 需要分层「Agent 身份、支付、Marketplace/Launchpad、钱包执行」
- 与 [S-ARCH-16](../03-system-design/S-ARCH-16-observability.md)、[S-CLOUD-03](../09-cloud-native/S-CLOUD-03-opentelemetry.md)、[S-ES 系列](../middleware/elasticsearch/index.md) 交叉复习

## 推荐阅读顺序

API 接入 → Agent/Tool → 工作流与 HITL → Persona/Memory → 安全与成本 → MCP →
MCP/A2A 互操作 → ERC-8004 身份 → x402/ERC-8183 Commerce → 开放平台/Launchpad →
RAG → 多模态
