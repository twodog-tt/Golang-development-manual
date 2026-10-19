// Package llmops is a bounded read-only FAQ cache and illustrative attempt-cost ledger.
package llmops

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"td-homework/examples/senior/agentmemory"
	"td-homework/examples/senior/llmclient"
)

// Rates are integer micro-USD per token for a named teaching tariff, NOT real prices.
type Rates struct {
	Version                    string
	Input, CachedInput, Output int64
}
type Costs struct {
	Attempts, Failed, Unknown int
	KnownMicroUSD             int64
	Tariff                    string
}
type Meter struct {
	mu    sync.Mutex
	rates Rates
	costs Costs
}

func NewMeter(r Rates) (*Meter, error) {
	if r.Version == "" || r.Input < 0 || r.CachedInput < 0 || r.Output < 0 {
		return nil, errors.New("invalid tariff")
	}
	return &Meter{rates: r, costs: Costs{Tariff: r.Version}}, nil
}
func (m *Meter) Record(a llmclient.Attempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.costs.Attempts++
	if a.Err != nil {
		m.costs.Failed++
	}
	u := a.Usage
	if !u.Known {
		m.costs.Unknown++
		return nil
	}
	if u.Source == "" || u.PromptTokens < 0 || u.CompletionTokens < 0 || u.CachedInputTokens < 0 || u.CachedInputTokens > u.PromptTokens {
		m.costs.Unknown++
		return errors.New("invalid reported usage")
	}
	counts := []int64{int64(u.PromptTokens - u.CachedInputTokens), int64(u.CachedInputTokens), int64(u.CompletionTokens)}
	rates := []int64{m.rates.Input, m.rates.CachedInput, m.rates.Output}
	var cost int64
	for i, n := range counts {
		if rates[i] != 0 && n > math.MaxInt64/rates[i] {
			m.costs.Unknown++
			return errors.New("cost overflow")
		}
		part := n * rates[i]
		if cost > math.MaxInt64-part {
			m.costs.Unknown++
			return errors.New("cost overflow")
		}
		cost += part
	}
	if m.costs.KnownMicroUSD > math.MaxInt64-cost {
		m.costs.Unknown++
		return errors.New("cost total overflow")
	}
	m.costs.KnownMicroUSD += cost
	return nil
}
func (m *Meter) Snapshot() Costs { m.mu.Lock(); defer m.mu.Unlock(); return m.costs }

type Config struct {
	PromptVersion, ModelVersion, CorpusVersion string
	TTL                                        time.Duration
}
type entry struct {
	Boundary, Question, Answer string
	Until                      time.Time
}
type Stats struct{ Tasks, Hits, Generations, Failures int }
type Engine struct {
	mu      sync.Mutex
	client  llmclient.Client
	config  Config
	entries []entry
	stats   Stats
	match   func(string, string) float64
}

func New(client llmclient.Client, c Config) (*Engine, error) {
	if client == nil || c.PromptVersion == "" || c.ModelVersion == "" || c.CorpusVersion == "" || c.TTL <= 0 || c.TTL > time.Hour {
		return nil, errors.New("invalid cache configuration")
	}
	return &Engine{client: client, config: c, match: similarity}, nil
}
func (e *Engine) Snapshot() Stats { e.mu.Lock(); defer e.mu.Unlock(); return e.stats }
func (e *Engine) Answer(ctx context.Context, scope agentmemory.Scope, q string) (answer string, hit bool, err error) {
	e.mu.Lock()
	e.stats.Tasks++
	e.mu.Unlock()
	defer func() {
		if err != nil {
			e.mu.Lock()
			e.stats.Failures++
			e.mu.Unlock()
		}
	}()
	who, err := agentmemory.Check(ctx, scope, "read")
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(q) == "" || len(q) > 4096 {
		return "", false, errors.New("invalid question")
	}
	boundaryBytes, _ := json.Marshal([]any{scope, who, e.config.PromptVersion, e.config.ModelVersion, e.config.CorpusVersion})
	boundary := string(boundaryBytes)
	e.mu.Lock()
	now := time.Now()
	live := e.entries[:0]
	for _, item := range e.entries {
		if item.Until.After(now) {
			live = append(live, item)
		}
	}
	e.entries = live
	for _, item := range e.entries {
		// Identity/scope/policy/versions/TTL checked BEFORE similarity is evaluated.
		if item.Boundary == boundary && e.match(q, item.Question) == 1 {
			answer = item.Answer
			hit = true
			break
		}
	}
	e.mu.Unlock()
	if hit {
		if _, err = agentmemory.Check(ctx, scope, "read"); err != nil {
			return "", false, err
		}
		e.mu.Lock()
		e.stats.Hits++
		e.mu.Unlock()
		return answer, true, nil
	}
	e.mu.Lock()
	e.stats.Generations++
	e.mu.Unlock()
	answer, _, err = e.client.Complete(ctx, []llmclient.Message{{Role: "system", Content: "Read-only local FAQ fixture. No tools or business side effects."}, {Role: "user", Content: q}})
	if err != nil {
		return "", false, err
	}
	if len(answer) == 0 || len(answer) > 64<<10 {
		return "", false, errors.New("invalid output budget")
	}
	if _, err = agentmemory.Check(ctx, scope, "read"); err != nil {
		return "", false, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.entries) >= 128 {
		e.entries = e.entries[1:]
	}
	e.entries = append(e.entries, entry{boundary, q, answer, time.Now().Add(e.config.TTL)})
	return answer, false, nil
}

// Local word-set overlap only. It cannot distinguish negation or guarantee semantic equivalence.
// The caller must restrict this demo to curated read-only FAQs; production needs quality evaluation.
func similarity(a, b string) float64 {
	words := func(s string) map[string]bool {
		m := map[string]bool{}
		for _, v := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			m[v] = true
		}
		return m
	}
	x, y := words(a), words(b)
	if len(x) == 0 || len(y) == 0 {
		return 0
	}
	same := 0
	for w := range x {
		if y[w] {
			same++
		}
	}
	return float64(same) / float64(len(x)+len(y)-same)
}
