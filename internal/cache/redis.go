package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/user/fx-settlement-engine/internal/domain"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr, password string) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	return &RedisCache{client: rdb}
}

func (c *RedisCache) SetAccount(ctx context.Context, acc *domain.Account, ttl time.Duration) error {
	data, err := json.Marshal(acc)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("account:%s", acc.ID)
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	key := fmt.Sprintf("account:%s", id)
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	acc := &domain.Account{}
	if err := json.Unmarshal([]byte(val), acc); err != nil {
		return nil, err
	}
	return acc, nil
}

func (c *RedisCache) InvalidateAccount(ctx context.Context, id string) error {
	key := fmt.Sprintf("account:%s", id)
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) SetQuote(ctx context.Context, quote *domain.Quote, ttl time.Duration) error {
	data, err := json.Marshal(quote)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("quote:%s", quote.ID)
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) GetQuote(ctx context.Context, id string) (*domain.Quote, error) {
	key := fmt.Sprintf("quote:%s", id)
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	q := &domain.Quote{}
	if err := json.Unmarshal([]byte(val), q); err != nil {
		return nil, err
	}
	return q, nil
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
