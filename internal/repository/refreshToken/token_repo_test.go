package refreshToken_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"registration-service/internal/repository/refreshToken"
)

func TestRefreshTokenRepo(t *testing.T) {
	ctx := context.Background()
	db, mock := redismock.NewClientMock()
	repo := refreshToken.New(db)

	t.Run("SaveToken", func(t *testing.T) {
		// Используем ExpectSet вместо ExpectSetEx
		mock.ExpectSet("refresh:1", "token123", 7*24*time.Hour).SetVal("OK")
		err := repo.SaveToken(ctx, 1, "token123", 7*24*time.Hour)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetToken", func(t *testing.T) {
		mock.ExpectGet("refresh:1").SetVal("token123")
		token, err := repo.GetToken(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, "token123", token)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DeleteToken", func(t *testing.T) {
		mock.ExpectDel("refresh:1").SetVal(1)
		err := repo.DeleteToken(ctx, 1)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ValidateToken (valid)", func(t *testing.T) {
		mock.ExpectGet("refresh:1").SetVal("token123")
		valid, err := repo.ValidateToken(ctx, 1, "token123")
		assert.NoError(t, err)
		assert.True(t, valid)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ValidateToken (invalid)", func(t *testing.T) {
		mock.ExpectGet("refresh:1").SetVal("token123")
		valid, err := repo.ValidateToken(ctx, 1, "wrongtoken")
		assert.NoError(t, err)
		assert.False(t, valid)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
