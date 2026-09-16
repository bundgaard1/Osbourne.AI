package common

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type claimsKey struct{}

// WithClaims stores authenticated claims in the context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// ClaimsFromContext retrieves the authenticated claims, if present.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(*Claims)
	return claims, ok
}

// RequiresAuthentication is the canonical error for missing/invalid tokens,
// so callers can distinguish "unauthenticated" from other failures.
func RequiresAuthentication() error {
	return status.Error(codes.Unauthenticated, "missing or invalid bearer token")
}

// AuthInterceptor is a gRPC unary interceptor that reads the
// `authorization: Bearer <token>` metadata, verifies the JWT against the
// shared secret, and injects its claims into the request context. Requests
// without a valid token are rejected with Unauthenticated.
func AuthInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, RequiresAuthentication()
		}

		values := md.Get(MetadataKey)
		if len(values) == 0 {
			return nil, RequiresAuthentication()
		}

		token := strings.TrimPrefix(values[0], BearerPrefix)
		token = strings.TrimSpace(token)

		claims, err := ParseJWT(secret, token)
		if err != nil {
			return nil, RequiresAuthentication()
		}

		ctx = WithClaims(ctx, claims)
		if rids := md.Get(RequestIDMetadataKey); len(rids) > 0 {
			ctx = WithRequestID(ctx, rids[0])
		}

		return handler(ctx, req)
	}
}

// AttachToken appends the bearer token to the outgoing gRPC metadata.
func AttachToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, MetadataKey, BearerPrefix+token)
}