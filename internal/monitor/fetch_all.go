package monitor

import (
	"context"
	"sync"
	"time"
)

const maxParallelCalls = 8

// fetchAll calls call for every container ID, at most maxParallelCalls at a time, each bounded by
// timeout, and returns the successful results by ID. A failed call is left out: the container
// most likely vanished between the listing and the call, and its readings stay unknown.
func fetchAll[T any](ctx context.Context, ids []string, timeout time.Duration,
	call func(context.Context, string) (T, error)) map[string]T {
	results := make([]T, len(ids))
	succeeded := make([]bool, len(ids))
	slots := make(chan struct{}, maxParallelCalls)
	var wg sync.WaitGroup
	for i, id := range ids {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Go(func() {
			defer func() { <-slots }()
			callCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if value, err := call(callCtx, id); err == nil {
				results[i], succeeded[i] = value, true
			}
		})
	}
	wg.Wait()

	byID := make(map[string]T, len(ids))
	for i, id := range ids {
		if succeeded[i] {
			byID[id] = results[i]
		}
	}
	return byID
}
