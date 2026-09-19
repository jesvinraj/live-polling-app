package database

import (
	"context"
	"fmt"
	"time"

	"live-polling-tool/config"

	"github.com/redis/go-redis/v9"
)

type RedisInstance struct {
	Client *redis.Client
}

func ConnectRedis(cfg *config.Config) (*RedisInstance, error) {
	var client *redis.Client

	addr := cfg.RedisAddr
	if len(addr) >= 8 && (addr[:8] == "redis://" || addr[:9] == "rediss://") {
		opts, err := redis.ParseURL(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid redis connection url: %w", err)
		}
		if cfg.RedisPassword != "" {
			opts.Password = cfg.RedisPassword
		}
		client = redis.NewClient(opts)
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &RedisInstance{
		Client: client,
	}, nil
}
