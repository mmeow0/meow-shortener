package middleware

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// GRPCAuthInterceptor подготавливает auth-контекст из metadata authorization
// и при необходимости возвращает клиенту новый токен в response headers.
func GRPCAuthInterceptor(authenticator *Authenticator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		token := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			values := md.Get("authorization")
			if len(values) > 0 {
				token = values[0]
			}
		}

		userID, signedToken, _ := authenticator.ResolveUser(token)
		if err := grpc.SetHeader(ctx, metadata.Pairs("authorization", "Bearer "+signedToken)); err != nil {
			return nil, err
		}

		return handler(WithUserAuth(ctx, userID, true), req)
	}
}

// TimeoutUnaryInterceptor ограничивает время выполнения unary gRPC-метода.
func TimeoutUnaryInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if timeout <= 0 {
			return handler(ctx, req)
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return handler(timeoutCtx, req)
	}
}
