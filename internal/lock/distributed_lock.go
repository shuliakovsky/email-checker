package lock

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"time"
)

// DistributedLock provides Redis-based distributed locking mechanism
type DistributedLock struct {
	client      redis.UniversalClient
	key         string
	token       string
	ttl         time.Duration
	clusterMode bool
}

// Creates new distributed lock instance with unique token
func NewLock(client redis.UniversalClient, key string, ttl time.Duration, clusterMode bool) *DistributedLock {
	return &DistributedLock{
		client:      client,
		key:         key,
		ttl:         ttl,
		token:       generateToken(),
		clusterMode: clusterMode,
	}
}

// Attempts to acquire lock. Returns true if successful.
// Always succeeds in non-cluster mode.
func (dl *DistributedLock) Acquire(ctx context.Context) bool {
	if !dl.clusterMode {
		return true // Bypass locking in standalone mode
	}
	return dl.client.SetNX(ctx, dl.key, dl.token, dl.ttl).Val()
}

// Releases lock atomically using Lua script for safety
func (dl *DistributedLock) Release(ctx context.Context) {
	if !dl.clusterMode {
		return
	}
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		end
		return 0
	`
	dl.client.Eval(ctx, script, []string{dl.key}, dl.token)
}

// Extends lock expiration time if still held
func (dl *DistributedLock) Refresh(ctx context.Context) bool {
	if !dl.clusterMode {
		return true
	}
	return dl.client.Expire(ctx, dl.key, dl.ttl).Val()
}

// Starts background goroutine to periodically refresh lock
func (dl *DistributedLock) StartRefresh(ctx context.Context) {
	if !dl.clusterMode {
		return
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !dl.Refresh(ctx) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Generates unique lock identifier using current timestamp
func generateToken() string {
	return fmt.Sprintf("lock:%d", time.Now().UnixNano())
}
