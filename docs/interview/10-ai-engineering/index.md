# 10 AI 工程与编程

14 题 | Agent Platform / Crypto Agent 生态岗位 P0 | [返回索引](../../interview-catalog.md) · [角色优先级](../_meta/role-priority-matrix.md)

> 面向 **AI Agent Platform / Crypto Agent Ecosystem 与 Go 后端** 的工程面试：既覆盖模型接入，
> 也覆盖可恢复工作流、HITL、Memory、MCP/A2A、Agent 身份与商业协议、开放平台和 Web3
> 安全执行面，非算法研究员方向。

| ID | 题目 | 频率 |
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

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-AI-01 | `examples/senior/llmclient/` | `go test ./examples/senior/llmclient/...` |
| S-AI-02 | `examples/senior/rag/` | `go test ./examples/senior/rag/...` |
| S-AI-07 | `examples/senior/mcp/` | `go test ./examples/senior/mcp/...` · `go run ./examples/senior/mcp/` |

## 适用场景

- JD 含 **AI Agent Platform / Crypto Agent / Agent Economy / MCP / A2A / Agent SDK**
- 二面问「Agent 如何暂停恢复」「审批后如何防重复执行」「Memory 如何隔离与学习」
- 架构面问「Agent 身份、支付、Marketplace/Launchpad、钱包执行如何分层」
- 与 [S-ARCH-16](../03-system-design/S-ARCH-16-observability.md)、[S-CLOUD-03](../09-cloud-native/S-CLOUD-03-opentelemetry.md)、[S-ES 系列](../middleware/elasticsearch/index.md) 交叉复习

## 推荐刷题顺序

API 接入 → Agent/Tool → 工作流与 HITL → Persona/Memory → 安全与成本 → MCP →
MCP/A2A 互操作 → ERC-8004 身份 → x402/ERC-8183 Commerce → 开放平台/Launchpad →
RAG → 多模态
