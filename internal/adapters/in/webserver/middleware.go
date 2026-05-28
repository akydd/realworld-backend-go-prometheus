package webserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
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

// routePattern returns the matched Gorilla Mux route template (e.g.
// /api/articles/{slug}) rather than the actual request path. Using the
// template as a Prometheus label keeps cardinality bounded — one time series
// per route, not one per unique URL.
func routePattern(r *http.Request) string {
	route := mux.CurrentRoute(r)
	if route == nil {
		return r.URL.Path
	}
	tmpl, err := route.GetPathTemplate()
	if err != nil {
		return r.URL.Path
	}
	return tmpl
}

var httpDurationCollector = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "http_request_duration_ms",
	Help: "HTTP request duration in milliseconds, labeled by route template and method",
}, []string{"path", "method"})

func requestTimer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(now).Milliseconds()
		httpDurationCollector.WithLabelValues(routePattern(r), r.Method).Observe(float64(dur))
	})
}

var httpRequestCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "http_request_count",
	Help: "HTTP request count, labeled by route template, method, and status code",
}, []string{"path", "method", "status"})

func requestCounter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		httpRequestCounter.WithLabelValues(routePattern(r), r.Method, w.Header().Get("status")).Inc()
	})
}
