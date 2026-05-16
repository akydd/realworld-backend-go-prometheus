package grpc

import (
	"context"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey int

const UserIDKey contextKey = iota

type AuthRequirement int

const (
	NoAuth        AuthRequirement = iota
	OptionalAuth                  // parse token if present; proceed without it if absent or invalid
	MandatoryAuth                 // reject the call if token is absent or invalid
)

func AuthInterceptor(jwtSecret string, methods map[string]AuthRequirement) ggrpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *ggrpc.UnaryServerInfo, handler ggrpc.UnaryHandler) (interface{}, error) {
		authRequirement := methods[info.FullMethod]

		if authRequirement == NoAuth {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			if authRequirement == MandatoryAuth {
				return nil, status.Error(codes.Unauthenticated, "missing metadata")
			}
			return handler(ctx, req)
		}

		authHeaders := md["authorization"]
		if len(authHeaders) == 0 {
			if authRequirement == MandatoryAuth {
				return nil, status.Error(codes.Unauthenticated, "authorization token is required")
			}
			return handler(ctx, req)
		}

		authHeader := authHeaders[0]
		if !strings.HasPrefix(authHeader, "Token ") {
			if authRequirement == MandatoryAuth {
				return nil, status.Error(codes.Unauthenticated, "authorization token is required")
			}
			return handler(ctx, req)
		}

		rawToken := strings.TrimPrefix(authHeader, "Token ")
		token, err := jwt.ParseWithClaims(rawToken, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, status.Error(codes.Unauthenticated, jwt.ErrSignatureInvalid.Error())
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			if authRequirement == MandatoryAuth {
				return nil, status.Error(codes.Unauthenticated, "invalid token")
			}
			return handler(ctx, req)
		}

		if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && claims.Subject != "" {
			if userID, err := strconv.Atoi(claims.Subject); err == nil {
				ctx = context.WithValue(ctx, UserIDKey, userID)
			}
		}

		return handler(ctx, req)
	}
}
