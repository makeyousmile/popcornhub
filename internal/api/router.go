package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"popcornhub/internal/api/handlers"
	"popcornhub/internal/api/middleware"
	"popcornhub/internal/config"
	"popcornhub/internal/database"
	"popcornhub/internal/websocket"
)

type RouterDependencies struct {
	Config       *config.Config
	AuthHandler  *handlers.AuthHandler
	GroupHandler *handlers.GroupHandler
	MediaHandler *handlers.MediaHandler
	RoomHandler  *handlers.RoomHandler
	PresenceHub  *websocket.PresenceHub
	RoomHub      *websocket.RoomHub
	StaticDir    string
	MongoDB      *database.MongoDB
	RedisClient  *database.RedisClient
}

func NewRouter(deps RouterDependencies) http.Handler {
	mux := http.NewServeMux()
	authMW := middleware.Auth(deps.Config.JWTSecret)

	// Health check (verifies Mongo and Redis connectivity)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var mongoOK, redisOK = true, true
		if deps.MongoDB != nil && deps.MongoDB.Database != nil {
			if err := deps.MongoDB.Database.Client().Ping(ctx, nil); err != nil {
				mongoOK = false
			}
		}
		if deps.RedisClient != nil && deps.RedisClient.Client != nil {
			if err := deps.RedisClient.Client.Ping(ctx).Err(); err != nil {
				redisOK = false
			}
		}

		if !mongoOK || !redisOK {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "degraded",
				"mongo":  mongoOK,
				"redis":  redisOK,
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"app":    "popcornhub",
		})
	})

	// Prometheus Metrics (with optional METRICS_TOKEN authorization)
	promHandler := promhttp.Handler()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		if deps.Config.MetricsToken != "" {
			token := r.Header.Get("X-Metrics-Token")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			if token != deps.Config.MetricsToken {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
		}
		promHandler.ServeHTTP(w, r)
	})

	// Public Auth endpoints
	mux.HandleFunc("POST /api/v1/auth/register", deps.AuthHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", deps.AuthHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", deps.AuthHandler.Refresh)

	// Protected Auth endpoints
	mux.Handle("GET /api/v1/auth/me", authMW(http.HandlerFunc(deps.AuthHandler.Me)))

	// Protected Group endpoints
	mux.Handle("POST /api/v1/groups", authMW(http.HandlerFunc(deps.GroupHandler.Create)))
	mux.Handle("GET /api/v1/groups", authMW(http.HandlerFunc(deps.GroupHandler.ListUserGroups)))
	mux.Handle("GET /api/v1/groups/{id}", authMW(http.HandlerFunc(deps.GroupHandler.GetGroup)))
	mux.Handle("POST /api/v1/groups/join", authMW(http.HandlerFunc(deps.GroupHandler.Join)))
	mux.Handle("GET /api/v1/groups/{id}/members", authMW(http.HandlerFunc(deps.GroupHandler.GetMembers)))
	mux.Handle("GET /api/v1/groups/{id}/presence", authMW(http.HandlerFunc(deps.GroupHandler.GetPresence)))

	// Protected Media endpoints
	mux.Handle("GET /api/v1/media", authMW(http.HandlerFunc(deps.MediaHandler.List)))
	mux.Handle("GET /api/v1/media/{id}", authMW(http.HandlerFunc(deps.MediaHandler.Get)))
	mux.Handle("POST /api/v1/media", authMW(http.HandlerFunc(deps.MediaHandler.Create)))

	// Protected Room endpoints
	mux.Handle("POST /api/v1/groups/{id}/room", authMW(http.HandlerFunc(deps.RoomHandler.CreateOrGetRoom)))
	mux.Handle("GET /api/v1/rooms/{id}", authMW(http.HandlerFunc(deps.RoomHandler.GetRoom)))

	// WebSockets (authenticates via query parameter token)
	mux.HandleFunc("GET /ws/presence", deps.PresenceHub.HandleWebSocket)
	mux.HandleFunc("GET /ws/rooms/{id}", deps.RoomHub.HandleWebSocket)

	// Static UI serving
	if deps.StaticDir != "" {
		fs := http.FileServer(http.Dir(deps.StaticDir))
		mux.Handle("GET /static/", http.StripPrefix("/static/", fs))
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, deps.StaticDir+"/index.html")
		})
	}

	// Global middleware chain: SecurityHeaders -> CORS -> Recover -> Logger & Metrics -> Handler
	return middleware.SecurityHeaders(
		middleware.CORS(
			middleware.Recover(
				middleware.LoggerAndMetrics(mux),
			),
		),
	)
}
