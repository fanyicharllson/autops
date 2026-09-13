package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisTimeout = 100 * time.Millisecond
	releasedTTL  = 120 * time.Second
)

// ErrUnknownTicket is returned when a queue_id is neither queued nor released.
var ErrUnknownTicket = errors.New("queue slot expired or unknown")

var (
	client           *redis.Client
	knownTenants     []string
	releaseInterval        = 5 * time.Second
	releaseBatchSize int64 = 5
)

// SetClient configures the Redis client used for the virtual queue.
func SetClient(redisClient *redis.Client) {
	client = redisClient
}

// ConfigureRelease sets release-loop parameters and the tenants to drain.
func ConfigureRelease(tenants []string, intervalSeconds, batchSize int) {
	knownTenants = append([]string(nil), tenants...)
	if intervalSeconds > 0 {
		releaseInterval = time.Duration(intervalSeconds) * time.Second
	}
	if batchSize > 0 {
		releaseBatchSize = int64(batchSize)
	}
}

// ReleaseIntervalSeconds returns the configured release interval in seconds.
func ReleaseIntervalSeconds() int {
	return int(releaseInterval / time.Second)
}

// ReleaseBatchSize returns the configured release batch size.
func ReleaseBatchSize() int {
	return int(releaseBatchSize)
}

// Enqueue adds a new ticket for tenant and returns its queue ID.
func Enqueue(tenant string) (string, error) {
	if client == nil {
		return "", errors.New("redis client is not configured")
	}
	id, err := newQueueID()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()
	score := float64(time.Now().UnixMilli())
	if err := client.ZAdd(ctx, queueKey(tenant), redis.Z{Score: score, Member: id}).Err(); err != nil {
		return "", fmt.Errorf("enqueue: %w", err)
	}
	return id, nil
}

// Position returns the 1-based queue position and whether the ticket is released.
func Position(tenant, queueID string) (position int, released bool, err error) {
	if client == nil {
		return 0, false, errors.New("redis client is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	n, err := client.Exists(ctx, releasedKey(tenant, queueID)).Result()
	if err != nil {
		return 0, false, fmt.Errorf("check released: %w", err)
	}
	if n > 0 {
		return 0, true, nil
	}

	rank, err := client.ZRank(ctx, queueKey(tenant), queueID).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, ErrUnknownTicket
	}
	if err != nil {
		return 0, false, fmt.Errorf("zrank: %w", err)
	}
	return int(rank) + 1, false, nil
}

// Redeem consumes a one-time released token. Returns true if redeemed.
func Redeem(tenant, queueID string) (bool, error) {
	if client == nil {
		return false, errors.New("redis client is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	n, err := client.Del(ctx, releasedKey(tenant, queueID)).Result()
	if err != nil {
		return false, fmt.Errorf("redeem: %w", err)
	}
	return n > 0, nil
}

// StartReleaseLoop periodically releases the oldest queued tickets.
// Redis failures are logged at WARN and skipped until the next interval.
func StartReleaseLoop(ctx context.Context) {
	ticker := time.NewTicker(releaseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			releaseBatch()
		}
	}
}

func releaseBatch() {
	if client == nil {
		slog.Warn("queue release skipped: redis client is not configured")
		return
	}
	for _, tenant := range knownTenants {
		if err := releaseTenant(tenant); err != nil {
			slog.Warn("queue release failed; skipping cycle for tenant",
				"tenant", tenant,
				"err", err,
			)
		}
	}
}

func releaseTenant(tenant string) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
	defer cancel()

	ids, err := client.ZRange(ctx, queueKey(tenant), 0, releaseBatchSize-1).Result()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		pipeCtx, pipeCancel := context.WithTimeout(context.Background(), redisTimeout)
		pipe := client.TxPipeline()
		pipe.ZRem(pipeCtx, queueKey(tenant), id)
		pipe.SetEx(pipeCtx, releasedKey(tenant, id), "1", releasedTTL)
		_, err := pipe.Exec(pipeCtx)
		pipeCancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func queueKey(tenant string) string {
	return "autops:queue:" + tenant
}

func releasedKey(tenant, queueID string) string {
	return "autops:released:" + tenant + ":" + queueID
}

func newQueueID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// UUID v4 variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}
