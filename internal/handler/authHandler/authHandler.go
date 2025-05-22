package authHandler

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"registration-service/api/authproto/proto-generate"
	"registration-service/internal/service/authService"
)

type GRPChandler struct {
	authService *authService.AuthService
	auth.UnimplementedAuthServiceServer
}

func New(service *authService.AuthService) *GRPChandler {
	return &GRPChandler{authService: service}
}

func (h *GRPChandler) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.RegisterResponse, error) {
	err := h.authService.Register(ctx, req.Username, req.Email, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return &auth.RegisterResponse{Message: "user created"}, nil
}

func (h *GRPChandler) Login(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	accesstoken, refreshToken, err := h.authService.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}
	return &auth.LoginResponse{Token: accesstoken, RefreshToken: refreshToken}, nil
}

func (h *GRPChandler) GetUIDByToken(ctx context.Context, req *auth.GetUIDByTokenRequest) (*auth.GetUIDByTokenResponse, error) {
	uid, isValid := h.authService.GetUIDByToken(ctx, req.Token)

	return &auth.GetUIDByTokenResponse{Uid: uid, IsValid: isValid}, nil
}

func (h *GRPChandler) Logout(ctx context.Context, req *auth.LogoutRequest) (*auth.LogoutResponse, error) {
	err := h.authService.Logout(ctx, req.UserID, req.AccessToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}
	return &auth.LogoutResponse{Message: "logout successful"}, nil
}

func (h *GRPChandler) RefreshToken(ctx context.Context, req *auth.RefreshTokenRequest) (*auth.RefreshTokenResponse, error) {
	newToken, _, err := h.authService.RefreshToken(ctx, req.UserID, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}
	return &auth.RefreshTokenResponse{Token: newToken}, nil
}
