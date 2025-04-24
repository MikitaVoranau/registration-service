package redis_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"registration-service/pkg/database/redis"
)

func TestNew(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		cfg := redis.RedisConfig{
			Host: "localhost",
			Port: "6379",
		}

		client := redis.New(cfg)
		assert.Equal(t, "localhost:6379", client.Options().Addr)
		assert.Equal(t, 0, client.Options().DB)
	})

	t.Run("With password and DB", func(t *testing.T) {
		cfg := redis.RedisConfig{
			Host:     "redis",
			Port:     "6380",
			Password: "secret",
			Db:       1,
		}

		client := redis.New(cfg)
		assert.Equal(t, "redis:6380", client.Options().Addr)
		assert.Equal(t, "secret", client.Options().Password)
		assert.Equal(t, 1, client.Options().DB)
	})
}
