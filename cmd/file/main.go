package main

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	auth "registration-service/api/authproto/proto-generate"
	fileproto "registration-service/api/fileproto/proto-generate"
	"registration-service/internal/MinIO"
	"registration-service/internal/config"
	"registration-service/internal/handler/fileHandler"
	"registration-service/internal/repository/fileRepo"
	"registration-service/internal/service/fileService"
	"registration-service/pkg/database/postgres"
	"registration-service/pkg/logger"
	"registration-service/pkg/middleware"
)

func main() {
	ctx := context.Background()
	ctx, _ = logger.New(ctx)

	cfg, err := config.LoadFileConfig()
	if err != nil {
		logger.GetLogger(ctx).Fatal("Error loading config", zap.Error(err))
	}
	authConn, _ := grpc.Dial(
		cfg.AuthServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	defer authConn.Close()

	authClient := auth.NewAuthServiceClient(authConn)

	conn, err := postgres.New(cfg.Postgres)
	if err != nil {
		logger.GetLogger(ctx).Fatal("Error connecting to postgres", zap.Error(err))
	}

	fileSvc := fileService.New(
		fileRepo.New(conn),
		authClient,
		MinIO.New(cfg.MinIO),
	)

	server := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor(authClient)),
	)
	fileproto.RegisterFileServiceServer(server, fileHandler.NewFileHandler(fileSvc))

	lis, _ := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err := server.Serve(lis); err != nil {
		logger.GetLogger(ctx).Fatal("Failed to serve", zap.Error(err))
	}
}
