package middleware

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	auth "registration-service/api/authproto/proto-generate"
	"strings"
)

func AuthInterceptor(authClient auth.AuthServiceClient) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Пропускаем методы авторизации
		if info.FullMethod == "/auth.AuthService/Login" ||
			info.FullMethod == "/auth.AuthService/Register" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "metadata not provided")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "authorization token not provided")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")

		// Используем существующий метод GetUIDByToken
		resp, err := authClient.GetUIDByToken(ctx, &auth.GetUIDByTokenRequest{
			Token: token,
		})
		if err != nil || !resp.IsValid {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// Добавляем userID в контекст
		newCtx := context.WithValue(ctx, "userID", resp.Uid)
		return handler(newCtx, req)
	}
}
