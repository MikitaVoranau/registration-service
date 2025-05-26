package main

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
	auth "registration-service/api/authproto/proto-generate"
	"registration-service/internal/config"
	"registration-service/internal/handler/authHandler"
	"registration-service/internal/repository/BlackListRepo"
	"registration-service/internal/repository/refreshToken"
	"registration-service/internal/repository/userRepo"
	"registration-service/internal/service/authService"
	"registration-service/pkg/database/postgres"
	"registration-service/pkg/database/redis"
	"registration-service/pkg/logger"
)

func main() {
	ctx := context.Background()
	ctx, _ = logger.New(ctx)

	cfg, err := config.LoadAuthConfig()
	if err != nil {
		logger.GetLogger(ctx).Fatal("Error loading config", zap.Error(err))
	}
	conn, err := postgres.New(cfg.Postgres)
	if err != nil {
		logger.GetLogger(ctx).Fatal("Failed to connect to database", zap.Error(err))
	}
	redisClient := redis.New(cfg.Redis)

	authSvc := authService.New(
		userRepo.New(conn),
		cfg.JWTSecret,
		refreshToken.New(redisClient),
		BlackListRepo.NewBlackListRepo(redisClient),
	)

	server := grpc.NewServer()
	auth.RegisterAuthServiceServer(server, authHandler.New(authSvc))

	lis, _ := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err := server.Serve(lis); err != nil {
		logger.GetLogger(ctx).Fatal("Failed to serve", zap.Error(err))
	}
}
