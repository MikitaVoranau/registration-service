package main

import (
	"context"
	"fmt"
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

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()
	var err error
	ctx, err = logger.New(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	log := logger.GetLogger(ctx)

	log.Info("Starting file service...")

	cfg, err := config.LoadFileConfig()
	if err != nil {
		log.Fatal("Error loading config", zap.Error(err))
	}
	log.Info("Config loaded successfully",
		zap.String("auth_service_addr", cfg.AuthServiceAddr),
		zap.String("minio_endpoint", cfg.MinIO.MinioEndpoint),
		zap.String("minio_bucket", cfg.MinIO.BucketName))

	authConn, err := grpc.Dial(
		cfg.AuthServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal("Failed to connect to auth service", zap.Error(err))
	}
	defer authConn.Close()
	log.Info("Connected to auth service")

	authClient := auth.NewAuthServiceClient(authConn)

	conn, err := postgres.New(cfg.Postgres)
	if err != nil {
		log.Fatal("Error connecting to postgres", zap.Error(err))
	}
	log.Info("Connected to postgres")

	minioClient, err := MinIO.New(cfg.MinIO)
	if err != nil {
		log.Fatal("Failed to initialize MinIO client", zap.Error(err))
	}
	log.Info("MinIO client initialized successfully",
		zap.String("endpoint", cfg.MinIO.MinioEndpoint),
		zap.String("bucket", cfg.MinIO.BucketName))

	fileSvc := fileService.New(
		fileRepo.New(conn),
		authClient,
		minioClient,
	)

	server := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.AuthInterceptor(authClient)),
	)
	fileproto.RegisterFileServiceServer(server, fileHandler.NewFileHandler(fileSvc))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		log.Fatal("Failed to listen", zap.Error(err))
	}
	log.Info("Starting gRPC server", zap.String("port", cfg.GRPCPort))

	if err := server.Serve(lis); err != nil {
		log.Fatal("Failed to serve", zap.Error(err))
	}
}
