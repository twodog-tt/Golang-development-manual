package llmclient

import (
	"context"
	"strings"
	"testing"
)

func TestMockClient_StreamChat(t *testing.T) {
	c := &MockClient{Reply: "hello world from mock"}
	var parts []string
	usage, err := c.StreamChat(context.Background(), []Message{
		{Role: "user", Content: "hi"},
	}, func(chunk string) error {
		parts = append(parts, chunk)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(parts, "")
	if got != "hello world from mock" {
		t.Fatalf("got %q", got)
	}
	if usage.PromptTokens != 1 {
		t.Fatalf("prompt tokens: %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 4 {
		t.Fatalf("completion tokens: %d", usage.CompletionTokens)
	}
}

func TestMockClient_StreamCancel(t *testing.T) {
	c := &MockClient{Reply: "one two three four"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.StreamChat(ctx, nil, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected cancel error")
	}
}

func TestMockClient_EchoDefault(t *testing.T) {
	c := &MockClient{}
	out, usage, err := c.Complete(context.Background(), []Message{
		{Role: "user", Content: "ping"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "echo: ping" {
		t.Fatalf("got %q", out)
	}
	if usage.CompletionTokens != 2 {
		t.Fatalf("completion tokens: %d", usage.CompletionTokens)
	}
}
