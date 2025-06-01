package middleware

import (
	"context"
	"log"
	auth "registration-service/api/authproto/proto-generate"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
			log.Printf("[AuthInterceptor] Invalid token or error: %v, IsValid: %t", err, resp.IsValid)
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		log.Printf("[AuthInterceptor] Token validated. UID: %d. Adding to context with key 'userID'", resp.Uid)

		// Добавляем userID в контекст
		newCtx := context.WithValue(ctx, "userID", resp.Uid)

		// Проверка сразу после добавления
		if val := newCtx.Value("userID"); val != nil {
			log.Printf("[AuthInterceptor] userID successfully added to newCtx. Value: %v, Type: %T", val, val)
		} else {
			log.Printf("[AuthInterceptor] ERROR: userID NOT FOUND in newCtx immediately after adding!")
		}

		return handler(newCtx, req)
	}
}

func StreamAuthInterceptor(authClient auth.AuthServiceClient) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Пропускаем методы, если они не требуют аутентификации (если такие есть для стримов)
		// if info.FullMethod == "/some.Service/UnprotectedStream" {
		// 	 return handler(srv, ss)
		// }

		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			log.Printf("[StreamAuthInterceptor] Metadata not provided for method %s", info.FullMethod)
			return status.Error(codes.Unauthenticated, "metadata not provided")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			log.Printf("[StreamAuthInterceptor] Authorization token not provided for method %s", info.FullMethod)
			return status.Error(codes.Unauthenticated, "authorization token not provided")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")

		resp, err := authClient.GetUIDByToken(ctx, &auth.GetUIDByTokenRequest{
			Token: token,
		})
		if err != nil || !resp.IsValid {
			log.Printf("[StreamAuthInterceptor] Invalid token or error for method %s: %v, IsValid: %t", info.FullMethod, err, resp.IsValid)
			return status.Error(codes.Unauthenticated, "invalid token")
		}

		log.Printf("[StreamAuthInterceptor] Token validated for method %s. UID: %d. Adding to context with key 'userID'", info.FullMethod, resp.Uid)
		newCtx := context.WithValue(ctx, "userID", resp.Uid)

		// Оборачиваем ServerStream для использования нового контекста
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          newCtx,
		}

		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream помогает передать измененный контекст в обработчик стрима.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
