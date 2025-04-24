package refreshToken

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type RefreshTokenRepo struct {
	Client *redis.Client
}

func New(client *redis.Client) *RefreshTokenRepo {
	return &RefreshTokenRepo{Client: client}
}

func (r *RefreshTokenRepo) buildKey(userID uint32) string {
	return fmt.Sprintf("refresh:%d", userID)
}

func (r *RefreshTokenRepo) SaveToken(ctx context.Context, userID uint32, token string, ttl time.Duration) error {
	key := r.buildKey(userID)
	return r.Client.Set(ctx, key, token, ttl).Err()
}

func (r *RefreshTokenRepo) GetToken(ctx context.Context, userID uint32) (string, error) {
	key := r.buildKey(userID)
	return r.Client.Get(ctx, key).Result()
}

func (r *RefreshTokenRepo) DeleteToken(ctx context.Context, userID uint32) error {
	key := r.buildKey(userID)
	return r.Client.Del(ctx, key).Err()
}

func (r *RefreshTokenRepo) ValidateToken(ctx context.Context, userID uint32, token string) (bool, error) {
	storedToken, err := r.GetToken(ctx, userID)
	if err != nil {
		return false, err
	}
	return storedToken == token, nil
}
