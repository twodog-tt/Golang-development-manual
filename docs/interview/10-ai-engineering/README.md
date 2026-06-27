# 10 AI 工程与编程

8 题 | P1 扩展（2024+ 高频） | [返回索引](../README.md)

> 面向 **Go 后端** 在业务中接入大模型、RAG、Agent 的面试场景；偏工程落地，非算法研究员方向。

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

## 可运行代码

| 题 ID | 目录 | 命令 |
|-------|------|------|
| S-AI-01 | `examples/senior/llmclient/` | `go test ./examples/senior/llmclient/...` |
| S-AI-02 | `examples/senior/rag/` | `go test ./examples/senior/rag/...` |
| S-AI-07 | `examples/senior/mcp/` | `go test ./examples/senior/mcp/...` · `go run ./examples/senior/mcp/` |

## 适用场景

- JD 含 **AI 应用 / 大模型 / Copilot / MCP / 智能客服**
- 二面问「RAG 怎么做的」「能不能写 MCP Server」「语音怎么接」
- 与 [S-ARCH-16](../03-system-design/S-ARCH-16-observability.md)、[S-CLOUD-03](../09-cloud-native/S-CLOUD-03-opentelemetry.md)、[S-ES 系列](../middleware/elasticsearch/) 交叉复习

## 推荐刷题顺序

API 接入 → RAG → Agent → MCP → Prompt/Context → 安全 → 成本观测 → 多模态
