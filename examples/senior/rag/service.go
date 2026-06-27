package rag

import (
	"context"
	"fmt"
	"strings"

	"td-homework/examples/senior/llmclient"
)

// Service 编排检索 + LLM 生成（示例用 MockClient）。
type Service struct {
	Store  *Store
	LLM    llmclient.Client
	TopK   int
}

// Answer 检索相关片段并生成回答。
func (s *Service) Answer(ctx context.Context, question string) (string, []ScoredChunk, error) {
	if s.Store == nil || s.LLM == nil {
		return "", nil, fmt.Errorf("rag: store or llm is nil")
	}
	k := s.TopK
	if k <= 0 {
		k = 3
	}
	hits := s.Store.Search(question, k)

	var ctxBlock strings.Builder
	for i, h := range hits {
		ctxBlock.WriteString(fmt.Sprintf("[%d] (%s) %s\n", i+1, h.Chunk.Source, h.Chunk.Text))
	}

	prompt := fmt.Sprintf(`根据以下资料回答问题。若资料不足，回答「资料中未找到」。

资料：
%s

问题：%s`, ctxBlock.String(), question)

	reply, _, err := s.LLM.Complete(ctx, []llmclient.Message{
		{Role: "system", Content: "你是企业知识库助手，仅依据给定资料回答。"},
		{Role: "user", Content: prompt},
	})
	return reply, hits, err
}
