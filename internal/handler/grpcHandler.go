package handler

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "registration-service/api/proto-generate"
	"registration-service/internal/service"
)

type GRPChandler struct {
	authService *service.AuthService
	pb.UnimplementedAuthServiceServer
}

func New(service *service.AuthService) *GRPChandler {
	return &GRPChandler{authService: service}
}

func (h *GRPChandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	err := h.authService.Register(ctx, req.Username, req.Email, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return &pb.RegisterResponse{Message: "user created"}, nil
}

func (h *GRPChandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	accesstoken, refreshToken, err := h.authService.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}
	return &pb.LoginResponse{Token: accesstoken, RefreshToken: refreshToken}, nil
}

func (h *GRPChandler) GetUIDByToken(ctx context.Context, req *pb.GetUIDByTokenRequest) (*pb.GetUIDByTokenResponse, error) {
	uid, isValid := h.authService.GetUIDByToken(ctx, req.Token)

	return &pb.GetUIDByTokenResponse{Uid: uid, IsValid: isValid}, nil
}

func (h *GRPChandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	err := h.authService.Logout(ctx, req.UserID, req.AccessToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}
	return &pb.LogoutResponse{Message: "logout successful"}, nil
}

func (h *GRPChandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	newToken, _, err := h.authService.RefreshToken(ctx, req.UserID, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}
	return &pb.RefreshTokenResponse{Token: newToken}, nil
}
