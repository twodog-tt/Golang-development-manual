---
id: S-AI-04
title: Prompt 工程与 Context 窗口管理
module: ai-engineering
level: senior
frequency: 4
go_version: "1.22+"
tags: [prompt-engineering, context-window, token]
status: published
code_refs: []
sources:
  - https://platform.openai.com/docs/guides/prompt-engineering
  - https://docs.anthropic.com/en/docs/build-with-claude/prompt-engineering
  - https://github.com/dair-ai/Prompt-Engineering-Guide
---

# Prompt 工程与 Context 窗口管理

## 30 秒版（开场）

> **Prompt 工程**在后端侧体现为：指令模板化、few-shot 可配置、结构化输出和回归评估；**Context 管理**是控制 token 预算与不可信上下文边界。tokenizer、context window 和结构化输出能力都与具体模型/API 版本相关，不能把某个厂商参数背成通用标准。

## 3 分钟版（一面深度）

1. **是什么**：System/User/Assistant 等内容组成模型上下文；模型有 context window 和最大输出限制，二者关系按具体 API 定义。
2. **为什么**：后端要把 prompt 当 **配置/代码** 管理；超长对话会截断、变贵、变慢。
3. **怎么做**：模板引擎 + 安全变量边界；历史消息摘要压缩；RAG 片段按相关性、权限和预算选择；优先使用 provider 支持的 JSON Schema/Structured Outputs。普通 JSON mode 通常只约束“有效 JSON”，不等于符合业务 schema。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart TD
  Sys[System: 角色+规则+输出格式]
  Few[Few-shot 示例]
  RAG[RAG 检索片段]
  Hist[对话历史 摘要/最近N轮]
  User[当前用户输入]
  Sys --> Assemble[Prompt 组装]
  Few --> Assemble
  RAG --> Assemble
  Hist --> Assemble
  User --> Assemble
  Assemble -->|token 预算检查| LLM
```

**Context 裁剪策略**

| 策略 | 说明 |
|------|------|
| 最近 N 轮 | 简单；远历史丢失 |
| 滚动摘要 | 旧对话用小模型 summarize |
| 优先级 | 保留系统/开发者约束与当前用户请求；RAG 是不可信数据，按相关性裁剪，不能当更高优先级指令 |
| 硬截断 | 按 token 从低优先级删 |

**结构化输出（Go 解析）**

```go
type OrderIntent struct {
    Action string `json:"action"` // query | refund
    OrderID string `json:"order_id"`
}

// prompt 中明确 JSON schema + 示例
resp, err := llm.Complete(ctx, ChatRequest{
    Messages: msgs,
    ResponseFormat: &JSONSchema{Name: "order_intent", Schema: schema},
})
if err != nil {
    return err
}
var intent OrderIntent
if err := json.Unmarshal([]byte(resp.Content), &intent); err != nil {
    return fmt.Errorf("decode order intent: %w", err)
}
if err := validateIntent(intent); err != nil {
    return err
}
```

## 生产场景

- **多租户 SaaS**：每租户自定义 system prompt（存 DB，版本号 + 灰度）
- **长会话客服**：每 10 轮触发摘要写入 `conversation_summary` 字段
- **代码审查 Bot**：固定 checklist few-shot，减少漏项

## 排查与工具

- Token 计数：`tiktoken` 或 provider `usage` 回传
- Prompt 回归：golden dataset + 自动评分（LLM-as-judge 慎用）
- 配置：Git 管理 prompt 模板；敏感词过滤前置

## 架构取舍

| 方案 | 适用 |
|------|------|
| 模板 + 配置中心 | 运营可调、需审计 |
| 硬编码 prompt | 强合规、变更少 |
| 动态示例检索 | 复杂分类，类似 dynamic few-shot |

**何时少做 Prompt 花活**：有明确 API 契约时用 **Function Calling** 替代自然语言解析。

## 追问链

1. **System prompt 泄露怎么办？** → 不把 system prompt 当秘密存储或授权边界；其中不得放密钥。按它可能被部分推断/泄露设计，真正权限在代码层。
2. **中英文 token 差异？** → tokenizer 和模型不同，不能用固定“字/token”换算；用目标模型 tokenizer 或 provider usage 实测，并预留输出预算。
3. **怎么版本化？** → `prompt_id` + `version` 打日志，便于回放 bad case。
4. **Temperature 怎么设？** → 不同模型支持范围和语义不同，有些推理模型不开放该参数。通过任务评估集调参，不背固定区间；低温也不能保证事实正确或完全可复现。

## 反模式与事故

- **把整个知识库塞进 prompt** → 超窗、贵、注意力稀释 → 用 RAG
- **无输出 schema** → 正则扒 JSON 脆弱
- **prompt 与代码不同步发版** → 线上行为突变
- **在 prompt 里写真实密钥** → 立即轮换

## 代码示例

```go
const systemTmpl = `你是订单助手。仅回答订单相关问题。
输出必须是 JSON：{"answer":"","citations":[]}
禁止编造订单号。无数据时 answer 为 "不知道"。`
```

## 延伸阅读

- [OpenAI Prompt Engineering](https://platform.openai.com/docs/guides/prompt-engineering)
- [Anthropic Prompt Engineering](https://docs.anthropic.com/en/docs/build-with-claude/prompt-engineering)
