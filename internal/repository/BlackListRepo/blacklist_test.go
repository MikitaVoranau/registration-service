package BlackListRepo_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"registration-service/internal/repository/BlackListRepo"
)

func TestBlackListRepo(t *testing.T) {
	ctx := context.Background()
	db, mock := redismock.NewClientMock()
	repo := BlackListRepo.NewBlackListRepo(db)

	t.Run("AddToken success", func(t *testing.T) {
		// Используем ExpectSet вместо ExpectSetEx, так как в коде используется Set с TTL
		mock.ExpectSet("blacklist:token123", "1", time.Hour).SetVal("OK")
		err := repo.AddToken(ctx, "token123", time.Now().Add(time.Hour))
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("IsTokenBlacklisted (true)", func(t *testing.T) {
		mock.ExpectGet("blacklist:token123").SetVal("1")
		blacklisted, err := repo.IsTokenBlacklisted(ctx, "token123")
		assert.NoError(t, err)
		assert.True(t, blacklisted)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("IsTokenBlacklisted (false)", func(t *testing.T) {
		mock.ExpectGet("blacklist:token123").RedisNil()
		blacklisted, err := repo.IsTokenBlacklisted(ctx, "token123")
		assert.NoError(t, err)
		assert.False(t, blacklisted)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RemoveToken", func(t *testing.T) {
		mock.ExpectDel("blacklist:token123").SetVal(1)
		err := repo.RemoveToken(ctx, "token123")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
