package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisTimeout = 100 * time.Millisecond
	refreshEvery = 500 * time.Millisecond
)

var (
	client *redis.Client

	mu    sync.RWMutex
	modes = make(map[string]string) // last-known mode per tenant; hot-path reads only
)

// SetClient configures the Redis client used for mode storage. It is called
// once during application startup.
func SetClient(redisClient *redis.Client) {
	client = redisClient
}

// SeedTenants registers tenants in the local map (default "normal") so the
// background refresh loop knows which Redis keys to poll.
func SeedTenants(tenants []string) {
	mu.Lock()
	defer mu.Unlock()
	for _, t := range tenants {
		if t == "" {
			continue
		}
		if _, ok := modes[t]; !ok {
			modes[t] = "normal"
		}
	}
}

// StartRefreshLoop polls Redis every 500ms and updates the local mode map.
// Failures are logged at WARN; the local map is left unchanged so the hot
// path keeps serving last-known values. Call once from main as a goroutine.
func StartRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(refreshEvery)
	defer ticker.Stop()

	// Prime once immediately so admin-set modes are visible quickly after boot.
	refreshFromRedis()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshFromRedis()
		}
	}
}

// GetMode returns the current traffic-shaping mode for a tenant from the
// local in-memory map only — no Redis I/O on the hot path.
// Tenants with no entry default to "normal".
func GetMode(tenant string) (string, error) {
	mu.RLock()
	defer mu.RUnlock()
	if mode, ok := modes[tenant]; ok {
		return mode, nil
	}
	return "normal", nil
}

// SetMode updates the traffic-shaping mode for a tenant in Redis (source of
// truth) and immediately updates the local map so the change is visible
// without waiting for the next refresh cycle.
func SetMode(tenant, mode string) error {
	if tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	switch mode {
	case "normal", "shaping":
	default:
		return fmt.Errorf("invalid mode %q: must be \"normal\" or \"shaping\"", mode)
	}

	if client == nil {
		return errors.New("redis client is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	if err := client.Set(ctx, modeKey(tenant), mode, 0).Err(); err != nil {
		return fmt.Errorf("set mode in redis: %w", err)
	}

	mu.Lock()
	modes[tenant] = mode
	mu.Unlock()
	return nil
}

func refreshFromRedis() {
	if client == nil {
		slog.Warn("mode refresh skipped: redis client is not configured")
		return
	}

	mu.RLock()
	tenants := make([]string, 0, len(modes))
	for t := range modes {
		tenants = append(tenants, t)
	}
	mu.RUnlock()

	if len(tenants) == 0 {
		return
	}

	keys := make([]string, len(tenants))
	for i, t := range tenants {
		keys[i] = modeKey(t)
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	vals, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		slog.Warn("mode refresh failed; keeping last-known local modes", "err", err)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	for i, v := range vals {
		tenant := tenants[i]
		if v == nil {
			modes[tenant] = "normal"
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		switch s {
		case "normal", "shaping":
			modes[tenant] = s
		default:
			// Ignore unrecognized Redis values; keep last-known local mode.
		}
	}
}

func modeKey(tenant string) string {
	return "autops:mode:" + tenant
}
