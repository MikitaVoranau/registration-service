package service_test

import (
	"context"
	"testing"
	"time"

	"registration-service/internal/model"
	"registration-service/internal/repository/BlackListRepo"
	"registration-service/internal/repository/refreshToken"
	"registration-service/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func setupService(t *testing.T) *service.AuthService {
	// стартуем miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	// клиент go-redis
	cli := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	// репозитории
	refRepo := refreshToken.New(cli)
	blRepo := BlackListRepo.NewBlackListRepo(cli)
	// userRepo нам не нужен для этих тестов, передаём nil, но не будем вызывать методы, где он нужен
	return service.New(nil, "test-jwt-secret", refRepo, blRepo)
}

func TestGenerateJWT_And_GetUIDByToken(t *testing.T) {
	s := setupService(t)

	// делаем payload вручную
	user := &model.User{ID: 42}
	tokenStr, err := s.GenerateJWT(user) // экспортируемая обёртка над generateJWT
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	// распарсим и убедимся, что вернулся наш UID
	uid, valid := s.GetUIDByToken(context.Background(), tokenStr)
	assert.True(t, valid)
	assert.Equal(t, uint32(42), uid)
}

func TestGetUIDByToken_InvalidAndExpired(t *testing.T) {
	s := setupService(t)

	// 1) совсем не токен
	_, valid := s.GetUIDByToken(context.Background(), "not-a-token")
	assert.False(t, valid)

	// 2) токен с правильной подписью, но сразу истёк
	now := time.Now().Add(-time.Hour)
	claims := &jwt.RegisteredClaims{
		Subject:   "7",
		ExpiresAt: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Hour)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expired, err := tok.SignedString([]byte("test-jwt-secret"))
	assert.NoError(t, err)

	uid, valid2 := s.GetUIDByToken(context.Background(), expired)
	assert.False(t, valid2)
	assert.Equal(t, uint32(0), uid)
}

func TestGetUIDByToken_Blacklisted(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()

	// сгенерим рабочий токен
	claims := &jwt.RegisteredClaims{
		Subject:   "5",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ts, err := tok.SignedString([]byte("test-jwt-secret"))
	assert.NoError(t, err)

	// заносим в чёрный список
	err = s.BlacklistRepo().AddToken(ctx, ts, claims.ExpiresAt.Time)
	assert.NoError(t, err)

	uid, valid := s.GetUIDByToken(ctx, ts)
	assert.False(t, valid)
	assert.Equal(t, uint32(0), uid)
}

func TestRefreshToken_Expired(t *testing.T) {
	s := setupService(t)

	// нет сохранённого токена → ValidateToken вернёт false → RefreshToken error
	_, _, err := s.RefreshToken(context.Background(), 123, "some-random")
	assert.Error(t, err)
}

func TestLogout_BlacklistsAccessToken(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()

	// сделаем токен, который ещё поживёт минуту
	claims := &jwt.RegisteredClaims{
		Subject:   "9",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ts, err := tok.SignedString([]byte("test-jwt-secret"))
	assert.NoError(t, err)

	// вызов Logout
	err = s.Logout(ctx, 9, ts)
	assert.NoError(t, err)

	// теперь токен должен быть в blacklist
	blacklisted, err := s.BlacklistRepo().IsTokenBlacklisted(ctx, ts)
	assert.NoError(t, err)
	assert.True(t, blacklisted)
}
