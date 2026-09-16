package common

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RequestLoggerInterceptor is a gRPC unary interceptor that logs every RPC:
// method, request id, user id (when authenticated), status code, and elapsed
// time. Chain it after AuthInterceptor so request_id and user_id are already
// populated in the context it logs with.
func RequestLoggerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		attrs := []any{
			"method", info.FullMethod,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if rid := RequestIDFromContext(ctx); rid != "" {
			attrs = append(attrs, "request_id", rid)
		}
		if claims, ok := ClaimsFromContext(ctx); ok {
			attrs = append(attrs, "user_id", claims.UserID)
		}

		switch {
		case err == nil:
			slog.InfoContext(ctx, "grpc request", attrs...)
		case status.Code(err) == codes.Unauthenticated:
			slog.WarnContext(ctx, "grpc request denied", append(attrs, "code", codes.Unauthenticated.String())...)
		default:
			slog.ErrorContext(ctx, "grpc request failed", append(attrs, "code", status.Code(err).String(), "err", err)...)
		}

		return resp, err
	}
}
