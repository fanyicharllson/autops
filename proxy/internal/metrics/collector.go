// Package metrics records lightweight, per-tenant Redis counters.
package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisTimeout = 100 * time.Millisecond

var client *redis.Client

// SetClient configures the Redis client used for metrics. It is called once
// during application startup.
func SetClient(redisClient *redis.Client) {
	client = redisClient
}

// RecordRequest increments a tenant's request counter. Callers invoke it in a
// goroutine, so Redis latency never delays an HTTP response.
func RecordRequest(tenant string) {
	if client == nil {
		slog.Warn("metrics collector unavailable", "tenant", tenant, "err", "redis client is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	if err := client.Incr(ctx, "autops:metrics:"+tenant+":requests").Err(); err != nil {
		slog.Warn("metrics request counter failed", "tenant", tenant, "err", err)
	}
}
