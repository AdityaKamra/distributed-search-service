package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

func NewCache(addr string) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		PoolSize: 50,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &Cache{client: client}, nil
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Cache) AllowRateLimit(ctx context.Context, tenantID string, limitPerMinute int64) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s", tenantID)
	count, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		c.client.Expire(ctx, key, time.Minute)
	}
	return count <= limitPerMinute, nil
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *Cache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Cache) InvalidateTenantCache(ctx context.Context, tenantID string) {
	iter := c.client.Scan(ctx, 0, fmt.Sprintf("search:%s:*", tenantID), 0).Iterator()
	for iter.Next(ctx) {
		c.client.Del(ctx, iter.Val())
	}
}
