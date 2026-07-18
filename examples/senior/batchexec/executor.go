package batchexec

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrInvalidLimit = errors.New("batchexec: workers must be positive and queue must be non-negative")

type job[T any] struct {
	index int
	value T
}

type result[R any] struct {
	index int
	value R
	err   error
}

// MapOrdered executes at most workers callbacks concurrently and returns
// successful results in input order. On the first error it cancels remaining
// work and returns no partial result. fn must honor ctx for prompt cancellation.
func MapOrdered[T, R any](
	parent context.Context,
	items []T,
	workers int,
	queue int,
	fn func(context.Context, T) (R, error),
) ([]R, error) {
	if workers <= 0 || queue < 0 {
		return nil, ErrInvalidLimit
	}
	if fn == nil {
		return nil, errors.New("batchexec: nil callback")
	}
	if len(items) == 0 {
		return []R{}, nil
	}

	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)

	jobs := make(chan job[T], queue)
	results := make(chan result[R], workers)

	var workersWG sync.WaitGroup
	workersWG.Add(workers)
	for range workers {
		go func() {
			defer workersWG.Done()
			for current := range jobs {
				if context.Cause(ctx) != nil {
					return
				}
				value, err := fn(ctx, current.value)
				results <- result[R]{
					index: current.index,
					value: value,
					err:   err,
				}
				if err != nil {
					cancel(fmt.Errorf("item %d: %w", current.index, err))
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for index, item := range items {
			select {
			case jobs <- job[T]{index: index, value: item}:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		workersWG.Wait()
		close(results)
	}()

	ordered := make([]R, len(items))
	completed := 0
	for current := range results {
		if current.err == nil {
			ordered[current.index] = current.value
			completed++
		}
	}

	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	if completed != len(items) {
		return nil, errors.New("batchexec: incomplete result without cancellation cause")
	}
	return ordered, nil
}
