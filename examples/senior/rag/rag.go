// Package rag demonstrates sparse lexical retrieval with authorization, not semantic embeddings.
package rag

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"td-homework/examples/senior/agentmemory"
)

type Chunk struct {
	ID, Source, Text, Policy string
	Scope                    agentmemory.Scope
	Readers                  []string
	Version                  uint64
	Expires                  time.Time
}
type ScoredChunk struct {
	Chunk Chunk
	Score float64
}
type Store struct {
	mu       sync.RWMutex
	items    map[string]Chunk
	revision uint64
}

func NewStore() *Store { return &Store{items: map[string]Chunk{}} }
func chunkKey(scope agentmemory.Scope, id string) string {
	b, _ := json.Marshal([]any{scope, id})
	return string(b)
}
func (s *Store) Add(ctx context.Context, chunks ...Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, c := range chunks {
		who, err := agentmemory.Check(ctx, c.Scope, "write")
		if err != nil {
			return err
		}
		if c.ID == "" || c.Source == "" || c.Text == "" || len(c.Text) > 4096 || c.Policy != who.Policy || !c.Expires.After(time.Now()) {
			return errors.New("rag: invalid chunk metadata")
		}
		k := chunkKey(c.Scope, c.ID)
		if _, ok := s.items[k]; ok || seen[k] {
			return agentmemory.ErrConflict
		}
		seen[k] = true
	}
	for _, c := range chunks {
		s.revision++
		c.Version = s.revision
		c.Readers = slices.Clone(c.Readers)
		s.items[chunkKey(c.Scope, c.ID)] = c
	}
	return nil
}
func (s *Store) Delete(ctx context.Context, scope agentmemory.Scope, id string) error {
	if _, err := agentmemory.Check(ctx, scope, "delete"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := agentmemory.Check(ctx, scope, "delete"); err != nil {
		return err
	}
	delete(s.items, chunkKey(scope, id))
	return nil
}
func visible(c Chunk, who agentmemory.Identity) bool {
	return c.Policy == who.Policy && c.Expires.After(time.Now()) && (len(c.Readers) == 0 || slices.Contains(c.Readers, who.Subject))
}
func (s *Store) Search(ctx context.Context, scope agentmemory.Scope, query string, k int, minScore float64) ([]ScoredChunk, error) {
	who, err := agentmemory.Check(ctx, scope, "read")
	if err != nil {
		return nil, err
	}
	if k < 1 || k > 20 || len(query) > 4096 || math.IsNaN(minScore) || math.IsInf(minScore, 0) || minScore <= 0 || minScore > 1 {
		return nil, errors.New("rag: invalid query limits")
	}
	q := terms(query)
	if len(q) == 0 {
		return nil, nil
	}
	scored := []ScoredChunk{}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := agentmemory.Check(ctx, scope, "read"); err != nil {
		return nil, err
	}
	for _, c := range s.items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.Scope != scope || !visible(c, who) {
			continue
		} // authorization before scoring
		score := cosine(q, terms(c.Text))
		if score >= minScore && score > 0 {
			c.Readers = slices.Clone(c.Readers)
			scored = append(scored, ScoredChunk{c, score})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Chunk.ID < scored[j].Chunk.ID
		}
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > k {
		scored = scored[:k]
	}
	if _, err := agentmemory.Check(ctx, scope, "read"); err != nil {
		return nil, err
	}
	return scored, nil
}
func (s *Store) Validate(ctx context.Context, scope agentmemory.Scope, hits []ScoredChunk) error {
	who, err := agentmemory.Check(ctx, scope, "read")
	if err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, h := range hits {
		c, ok := s.items[chunkKey(scope, h.Chunk.ID)]
		if !ok || c.Version != h.Chunk.Version || !visible(c, who) {
			return agentmemory.ErrDenied
		}
	}
	return nil
}

// Han bigrams avoid treating a complete Chinese sentence as a single token.
// Exact sparse terms avoid hash collisions; this is still lexical, not semantic similarity.
func terms(text string) map[string]float64 {
	out := map[string]float64{}
	var run []rune
	kind := 0
	flush := func() {
		if len(run) == 0 {
			return
		}
		if kind == 1 && len(run) > 1 {
			for i := 0; i+1 < len(run); i++ {
				out[string(run[i:i+2])]++
			}
		} else {
			out[string(run)]++
		}
		run = nil
	}
	for _, r := range strings.ToLower(text) {
		next := 0
		if unicode.Is(unicode.Han, r) {
			next = 1
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			next = 2
		}
		if next == 0 {
			flush()
			kind = 0
			continue
		}
		if kind != next {
			flush()
		}
		kind = next
		run = append(run, r)
	}
	flush()
	return out
}
func cosine(a, b map[string]float64) float64 {
	var dot, aa, bb float64
	for k, v := range a {
		aa += v * v
		dot += v * b[k]
	}
	for _, v := range b {
		bb += v * v
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return dot / math.Sqrt(aa*bb)
}
func SplitParagraphs(doc string) []string {
	parts := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n\n")
	out := []string{}
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
