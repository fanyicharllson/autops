package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisTimeout = 100 * time.Millisecond

var client *redis.Client

// SetClient configures the Redis client used for mode storage. It is called
// once during application startup.
func SetClient(redisClient *redis.Client) {
	client = redisClient
}

// GetMode returns the current traffic-shaping mode for a tenant.
// Tenants with no entry default to "normal".
func GetMode(tenant string) (string, error) {
	if client == nil {
		return "", errors.New("redis client is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	mode, err := client.Get(ctx, modeKey(tenant)).Result()
	if errors.Is(err, redis.Nil) {
		return "normal", nil
	}
	if err != nil {
		return "", fmt.Errorf("get mode from redis: %w", err)
	}
	return mode, nil
}

// SetMode updates the traffic-shaping mode for a tenant.
// Valid modes are "normal" and "shaping".
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
	return nil
}

func modeKey(tenant string) string {
	return "autops:mode:" + tenant
}
