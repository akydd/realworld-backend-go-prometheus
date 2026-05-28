package webserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
)

type contextKey string

const userIDKey contextKey = "userID"

func optionalAuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			const prefix = "Token "
			if authHeader != "" && strings.HasPrefix(authHeader, prefix) {
				rawToken := strings.TrimPrefix(authHeader, prefix)
				token, err := jwt.ParseWithClaims(rawToken, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return []byte(jwtSecret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && claims.Subject != "" {
						if userID, err := strconv.Atoi(claims.Subject); err == nil {
							ctx := context.WithValue(r.Context(), userIDKey, userID)
							r = r.WithContext(ctx)
						}
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			authHeader := r.Header.Get("Authorization")
			const prefix = "Token "
			if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write(createErrResponse("token", []string{"is missing"}))
				return
			}

			rawToken := strings.TrimPrefix(authHeader, prefix)

			token, err := jwt.ParseWithClaims(rawToken, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
				return
			}

			claims, ok := token.Claims.(*jwt.RegisteredClaims)
			if !ok || claims.Subject == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
				return
			}

			userID, err := strconv.Atoi(claims.Subject)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Prometheus middleware for timing all http requests, labeled by request method and url path.

var httpDurationCollector = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "http_request_duration",
	Help: "http reuest duration in ms",
}, []string{"endpoint"})

func requestTimer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()

		next.ServeHTTP(w, r)

		dur := time.Since(now).Milliseconds()

		httpDurationCollector.WithLabelValues(r.URL.Path).Observe(float64(dur))
	})
}
