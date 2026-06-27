// Package llmclient 提供可测试的流式 LLM 客户端抽象与 Mock 实现（S-AI-01 示例）。
package llmclient

import "context"

// Message 表示一条对话消息。
type Message struct {
	Role    string // system | user | assistant
	Content string
}

// Usage 记录 token 用量（Mock 按字符估算）。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// StreamHandler 消费流式文本片段；返回 error 可中止生成。
type StreamHandler func(chunk string) error

// Client 抽象大模型调用，便于单测与多厂商切换。
type Client interface {
	Complete(ctx context.Context, messages []Message) (string, Usage, error)
	StreamChat(ctx context.Context, messages []Message, onChunk StreamHandler) (Usage, error)
}
