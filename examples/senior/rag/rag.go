// Package rag 演示简易 RAG：分块、哈希向量、Top-K 检索（S-AI-02 示例）。
package rag

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

const embedDim = 64

// Chunk 表示一段可检索文档。
type Chunk struct {
	ID     string
	Source string
	Text   string
}

// ScoredChunk 检索结果。
type ScoredChunk struct {
	Chunk Chunk
	Score float64
}

// chunkVec 内部存储向量。
type chunkVec struct {
	Chunk Chunk
	Vec   []float64
}

// Store 内存向量索引。
type Store struct {
	items []chunkVec
}

// NewStore 创建空索引。
func NewStore() *Store {
	return &Store{}
}

// Add 向索引追加文档块（自动计算向量）。
func (s *Store) Add(chunks ...Chunk) {
	for _, c := range chunks {
		s.items = append(s.items, chunkVec{
			Chunk: c,
			Vec:   embed(c.Text),
		})
	}
}

// Search 返回与 query 最相似的 Top-K 块。
func (s *Store) Search(query string, k int) []ScoredChunk {
	if k <= 0 || len(s.items) == 0 {
		return nil
	}
	qv := embed(query)
	scored := make([]ScoredChunk, 0, len(s.items))
	for _, it := range s.items {
		scored = append(scored, ScoredChunk{
			Chunk: it.Chunk,
			Score: cosine(qv, it.Vec),
		})
	}
	// 简单选择 Top-K（数据量小，全排序即可）
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
	if k > len(scored) {
		k = len(scored)
	}
	return scored[:k]
}

// SplitParagraphs 按空行切分段落作为 chunk。
func SplitParagraphs(doc string) []string {
	parts := strings.Split(doc, "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	var cur strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func embed(text string) []float64 {
	vec := make([]float64, embedDim)
	for _, tok := range tokenize(text) {
		h := fnv.New32a()
		h.Write([]byte(tok))
		idx := h.Sum32() % uint32(embedDim)
		vec[idx] += 1
	}
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	if norm == 0 {
		return vec
	}
	norm = math.Sqrt(norm)
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
