package BlackListRepo

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type BlackListRepo struct {
	Client *redis.Client
}

func NewBlackListRepo(client *redis.Client) *BlackListRepo {
	return &BlackListRepo{
		Client: client,
	}
}

func (r *BlackListRepo) buildKey(token string) string {
	return fmt.Sprintf("blacklist:%s", token)
}

func (r *BlackListRepo) AddToken(ctx context.Context, token string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	key := r.buildKey(token)
	return r.Client.Set(ctx, key, "1", ttl).Err()
}

func (r *BlackListRepo) RemoveToken(ctx context.Context, token string) error {
	key := r.buildKey(token)
	return r.Client.Del(ctx, key).Err()
}

func (r *BlackListRepo) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	key := r.buildKey(token)
	_, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
