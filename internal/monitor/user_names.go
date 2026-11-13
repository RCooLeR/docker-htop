package monitor

import (
	"context"
	"strconv"
	"sync"
	"time"
)

const (
	maxNameFetches = 4
	nameQueueSize  = 64
)

// userNames caches each container's UID-to-name map and fetches missing ones in the background,
// so a tick never waits for them. It is safe for concurrent use.
type userNames struct {
	engine   Engine
	timeout  time.Duration
	requests chan string

	mu sync.Mutex // guards known and queued
	// known holds the names per container ID; an empty map records a failed fetch, so the
	// container is not asked again.
	known  map[string]map[uint32]string
	queued map[string]bool
}

func newUserNames(engine Engine, timeout time.Duration) *userNames {
	return &userNames{
		engine:   engine,
		timeout:  timeout,
		requests: make(chan string, nameQueueSize),
		known:    make(map[string]map[uint32]string),
		queued:   make(map[string]bool),
	}
}

// name returns the user name of uid in the container, or the decimal UID while the name is
// unknown. The first lookup for a container queues a fetch of its names.
func (u *userNames) name(containerID string, uid uint32) string {
	u.mu.Lock()
	names, ok := u.known[containerID]
	enqueue := !ok && !u.queued[containerID]
	if enqueue {
		u.queued[containerID] = true
	}
	u.mu.Unlock()

	if enqueue {
		u.enqueue(containerID)
	}
	if name := names[uid]; name != "" {
		return name
	}
	return strconv.FormatUint(uint64(uid), 10)
}

func (u *userNames) enqueue(containerID string) {
	select {
	case u.requests <- containerID:
	default:
		// The queue is full; drop the mark so a later lookup queues the container again.
		u.mu.Lock()
		delete(u.queued, containerID)
		u.mu.Unlock()
	}
}

// retain forgets the names of containers not in live.
func (u *userNames) retain(live map[string]bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for id := range u.known {
		if !live[id] {
			delete(u.known, id)
		}
	}
}

// run fetches queued containers' names until ctx is done and returns when its workers ended.
func (u *userNames) run(ctx context.Context) {
	var wg sync.WaitGroup
	for range maxNameFetches {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case id := <-u.requests:
					u.fetch(ctx, id)
				}
			}
		})
	}
	wg.Wait()
}

func (u *userNames) fetch(ctx context.Context, containerID string) {
	callCtx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()
	names, err := u.engine.UserNames(callCtx, containerID)
	if err != nil || names == nil {
		names = map[uint32]string{}
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.queued, containerID)
	u.known[containerID] = names
}
