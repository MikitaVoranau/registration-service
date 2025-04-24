package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"regexp"
	"registration-service/internal/model"
	"registration-service/internal/repository/BlackListRepo"
	"registration-service/internal/repository/refreshToken"
	"registration-service/internal/repository/userRepo"
	"strconv"
	"time"
)

// Сделал регулярку для проверки почты на валидность
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

const (
	refreshTokenExpireTime = 7 * 24 * time.Hour
	jwtTokenExpireTime     = 3 * time.Hour
)

type AuthService struct {
	userRepo      *userRepo.UserRepo
	jwtSecretKey  string
	refreshRepo   *refreshToken.RefreshTokenRepo
	blacklistRepo *BlackListRepo.BlackListRepo
}

func New(userRepo *userRepo.UserRepo, jwtString string, tokenRepo *refreshToken.RefreshTokenRepo, blacklistrepo *BlackListRepo.BlackListRepo) *AuthService {
	return &AuthService{userRepo: userRepo, jwtSecretKey: jwtString, refreshRepo: tokenRepo, blacklistRepo: blacklistrepo}
}

func (s *AuthService) Register(ctx context.Context, username, email, password string) error {
	if username == "" || email == "" || password == "" {
		return fmt.Errorf("invalid format")
	}

	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	if existingUser, err := s.userRepo.GetUserByEmail(ctx, email); err == nil && existingUser != nil {
		return fmt.Errorf("email already exists")
	}

	usersWithSameUsername, _ := s.userRepo.GetByUsername(ctx, username)
	if len(usersWithSameUsername) > 0 {
		return fmt.Errorf("username already taken")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.userRepo.Create(ctx, username, email, string(hashedPassword))
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, string, error) {
	users, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil || users == nil {
		return "", "", errors.New("user not found")
	}

	var matchedUser *model.User
	for _, user := range users {
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err == nil {
			matchedUser = user
			break
		}
	}

	if matchedUser == nil {
		return "", "", errors.New("invalid credentials")
	}

	accessToken, err := s.generateJWT(matchedUser)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken(ctx, uint32(matchedUser.ID))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

func (s *AuthService) generateJWT(user *model.User) (string, error) {
	payload := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(user.ID), 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtTokenExpireTime)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	tokenStr, err := token.SignedString([]byte(s.jwtSecretKey))
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

func (s *AuthService) GetUIDByToken(ctx context.Context, token string) (uint32, bool) {
	blacklisted, err := s.blacklistRepo.IsTokenBlacklisted(ctx, token)
	if err != nil || blacklisted {
		return 0, false
	}

	payload := &jwt.RegisteredClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, payload, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecretKey), nil
	})

	if err != nil || !parsedToken.Valid {
		return 0, false
	}

	uid, err := strconv.ParseUint(payload.Subject, 10, 32)
	if err != nil {
		return 0, false
	}

	return uint32(uid), true
}

func (s *AuthService) generateRefreshToken(ctx context.Context, userID uint32) (string, error) {
	refreshToken := uuid.NewString()
	err := s.refreshRepo.SaveToken(ctx, userID, refreshToken, refreshTokenExpireTime)
	if err != nil {
		return "", err
	}
	return refreshToken, nil

}

func (s *AuthService) Logout(ctx context.Context, userID uint32, accessToken string) error {
	if err := s.refreshRepo.DeleteToken(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}

	payload := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(accessToken, payload, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecretKey), nil
	})
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}
	if err := s.blacklistRepo.AddToken(ctx, accessToken, payload.ExpiresAt.Time); err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	return nil
}

func (s *AuthService) RefreshToken(ctx context.Context, userID uint32, oldRefreshToken string) (string, string, error) {
	valid, err := s.refreshRepo.ValidateToken(ctx, userID, oldRefreshToken)
	if err != nil || !valid {
		return "", "", fmt.Errorf("expired refresh token")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", "", err
	}

	newAccessToken, err := s.generateJWT(user)
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := s.generateRefreshToken(ctx, userID)
	if err != nil {
		return "", "", err
	}

	return newAccessToken, newRefreshToken, nil
}

// для тестов
// ---------------------------------------
func (s *AuthService) GenerateJWT(user *model.User) (string, error) {
	return s.generateJWT(user)
}

func (s *AuthService) BlacklistRepo() *BlackListRepo.BlackListRepo {
	return s.blacklistRepo
}

//---------------------------------------
