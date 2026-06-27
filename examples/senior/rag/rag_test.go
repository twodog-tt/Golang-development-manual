package rag

import (
	"context"
	"strings"
	"testing"

	"td-homework/examples/senior/llmclient"
)

func TestStore_SearchOrdersRefund(t *testing.T) {
	store := NewStore()
	store.Add(
		Chunk{ID: "1", Source: "policy.md", Text: "订单退款需在 7 天内申请"},
		Chunk{ID: "2", Source: "shipping.md", Text: "默认快递 3 到 5 天送达"},
		Chunk{ID: "3", Source: "policy.md", Text: "退款审核通过后 1 到 3 个工作日到账"},
	)

	hits := store.Search("怎么退款", 2)
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d", len(hits))
	}
	if !strings.Contains(hits[0].Chunk.Text, "退款") {
		t.Fatalf("top hit should mention refund: %q", hits[0].Chunk.Text)
	}
}

func TestService_Answer(t *testing.T) {
	store := NewStore()
	store.Add(Chunk{ID: "1", Source: "faq", Text: "VIP 用户享受免运费"})

	svc := &Service{
		Store: store,
		LLM:   &llmclient.MockClient{},
		TopK:  1,
	}
	answer, hits, err := svc.Answer(context.Background(), "VIP 运费")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits: %d", len(hits))
	}
	if !strings.Contains(answer, "echo:") {
		t.Fatalf("unexpected answer: %q", answer)
	}
}
