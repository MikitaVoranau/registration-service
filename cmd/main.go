package main

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/signal"
	auth "registration-service/api/proto-generate"
	"registration-service/internal/config"
	"registration-service/internal/handler"
	"registration-service/internal/repository/BlackListRepo"
	"registration-service/internal/repository/refreshToken"
	"registration-service/internal/repository/userRepo"
	"registration-service/internal/service"
	"registration-service/pkg/database/postgres"
	"registration-service/pkg/database/redis"
	"registration-service/pkg/logger"
)

func main() {
	ctx := context.Background()

	ctx, _ = logger.New(ctx)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		logger.GetLogger(ctx).Fatal("Failed to load config", zap.Error(err))
	}

	conn, err := postgres.New(cfg.Postgres)
	if err != nil {
		logger.GetLogger(ctx).Fatal("Failed to connect to database", zap.Error(err))
	}

	redisClient := redis.New(cfg.Redis)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.GetLogger(ctx).Fatal("cannot connect to Redis", zap.Error(err))
	}

	userRepo := userRepo.New(conn)
	tokenRepo := refreshToken.New(redisClient)
	blacklistRepo := BlackListRepo.NewBlackListRepo(redisClient)
	authService := service.New(userRepo, cfg.JWTSecret, tokenRepo, blacklistRepo)
	grpcHandler := handler.New(authService)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		log.Printf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	auth.RegisterAuthServiceServer(grpcServer, grpcHandler)
	fmt.Println(cfg.JWTSecret)
	fmt.Println(cfg.Postgres.Password)
	go func() {
		logger.GetLogger(ctx).Info("server started", zap.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.GetLogger(ctx).Info("failed to serve", zap.Error(err))
		}
	}()
	<-ctx.Done()
	grpcServer.GracefulStop()
	logger.GetLogger(ctx).Info("server stopped")
}
