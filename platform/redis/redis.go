package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Config holds Redis connection settings.
type Config struct {
	Host       string
	Port       int
	Password   string
	TLSEnabled bool
}

// NewClient creates and pings a Redis client.
func NewClient(ctx context.Context, cfg Config) (*goredis.Client, error) {
	opts := &goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
	}

	client := goredis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return client, nil
}
