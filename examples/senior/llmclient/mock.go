package llmclient

import (
	"context"
	"errors"
	"strings"
)

// MockClient 按空格切分模拟流式输出，不发起真实 HTTP 请求。
type MockClient struct {
	// Reply 固定回复；为空时回显最后一条 user 消息。
	Reply string
	// ChunksPerToken 每个 chunk 包含几个「词」，默认 1。
	ChunksPerToken int
}

func (m *MockClient) Complete(ctx context.Context, messages []Message) (string, Usage, error) {
	var buf strings.Builder
	usage, err := m.StreamChat(ctx, messages, func(chunk string) error {
		buf.WriteString(chunk)
		return nil
	})
	return buf.String(), usage, err
}

func (m *MockClient) StreamChat(ctx context.Context, messages []Message, onChunk StreamHandler) (Usage, error) {
	if onChunk == nil {
		return Usage{}, errors.New("llmclient: onChunk is nil")
	}

	text := m.Reply
	if text == "" {
		text = lastUserContent(messages)
		if text == "" {
			text = "empty prompt"
		}
		text = "echo: " + text
	}

	promptTokens := estimateTokens(messages)
	words := strings.Fields(text)
	step := m.ChunksPerToken
	if step <= 0 {
		step = 1
	}

	for i := 0; i < len(words); i += step {
		if err := ctx.Err(); err != nil {
			return Usage{}, err
		}
		end := i + step
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		if end < len(words) {
			chunk += " "
		}
		if err := onChunk(chunk); err != nil {
			return Usage{}, err
		}
	}

	return Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: len(words),
	}, nil
}

func lastUserContent(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func estimateTokens(messages []Message) int {
	n := 0
	for _, msg := range messages {
		n += len(strings.Fields(msg.Content))
	}
	return n
}
