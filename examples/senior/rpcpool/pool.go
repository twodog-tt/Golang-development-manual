package rpcpool

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidConfig = errors.New("rpcpool: at least one endpoint and a positive hedge delay are required")

type Endpoint interface {
	Name() string
	Read(ctx context.Context, key string) (string, error)
}

type Pool struct {
	endpoints  []Endpoint
	hedgeDelay time.Duration
}

func New(endpoints []Endpoint, hedgeDelay time.Duration) (*Pool, error) {
	if len(endpoints) == 0 || hedgeDelay <= 0 {
		return nil, ErrInvalidConfig
	}
	for _, endpoint := range endpoints {
		if endpoint == nil {
			return nil, ErrInvalidConfig
		}
	}
	return &Pool{
		endpoints:  append([]Endpoint(nil), endpoints...),
		hedgeDelay: hedgeDelay,
	}, nil
}

type readResult struct {
	endpoint string
	value    string
	err      error
}

// Read returns the first successful hedged read. It is intentionally suitable
// only for idempotent reads; transaction construction/signing must not use it.
// Endpoints must honor ctx so losing requests can stop promptly.
func (p *Pool) Read(parent context.Context, key string) (string, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	results := make(chan readResult, len(p.endpoints))
	launch := func(endpoint Endpoint) {
		go func() {
			value, err := endpoint.Read(ctx, key)
			results <- readResult{
				endpoint: endpoint.Name(),
				value:    value,
				err:      err,
			}
		}()
	}

	next := 0
	active := 0
	launch(p.endpoints[next])
	next++
	active++

	var timer *time.Timer
	var timerC <-chan time.Time
	armTimer := func() {
		if next >= len(p.endpoints) {
			timerC = nil
			return
		}
		if timer == nil {
			timer = time.NewTimer(p.hedgeDelay)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(p.hedgeDelay)
		}
		timerC = timer.C
	}
	stopTimer := func() {
		if timer != nil {
			timer.Stop()
		}
	}
	armTimer()

	var failures []error
	for {
		select {
		case result := <-results:
			active--
			if result.err == nil {
				stopTimer()
				cancel()
				return result.value, nil
			}
			failures = append(failures, fmt.Errorf("%s: %w", result.endpoint, result.err))

			if active == 0 && next < len(p.endpoints) {
				launch(p.endpoints[next])
				next++
				active++
				armTimer()
			}
			if active == 0 && next == len(p.endpoints) {
				stopTimer()
				return "", errors.Join(failures...)
			}

		case <-timerC:
			timerC = nil
			launch(p.endpoints[next])
			next++
			active++
			armTimer()

		case <-ctx.Done():
			stopTimer()
			return "", ctx.Err()
		}
	}
}
