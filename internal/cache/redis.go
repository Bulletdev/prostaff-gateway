package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is an L2 distributed cache backed by Redis.
type Redis struct {
	client  *redis.Client
	enabled bool
	logger  *slog.Logger
}

func NewRedis(redisURL string, enabled bool, logger *slog.Logger) *Redis {
	if !enabled {
		return &Redis{enabled: false, logger: logger}
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("invalid redis url, L2 cache disabled", "error", err)
		return &Redis{enabled: false, logger: logger}
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Warn("redis unreachable at startup, L2 cache disabled", "error", err)
		return &Redis{enabled: false, logger: logger}
	}

	logger.Info("redis L2 cache connected", "url", redisURL)
	return &Redis{client: client, enabled: true, logger: logger}
}

func (r *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	if !r.enabled {
		return nil, redis.Nil
	}
	return r.client.Get(ctx, key).Bytes()
}

func (r *Redis) Set(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if !r.enabled {
		return
	}
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		r.logger.Warn("redis set failed", "key", key, "error", err)
	}
}

func (r *Redis) Ping(ctx context.Context) error {
	if !r.enabled || r.client == nil {
		return nil
	}
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Enabled() bool {
	return r.enabled
}
