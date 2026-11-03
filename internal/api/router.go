package api

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"popcornhub/internal/api/handlers"
	"popcornhub/internal/api/middleware"
	"popcornhub/internal/config"
	"popcornhub/internal/websocket"
)

type RouterDependencies struct {
	Config          *config.Config
	AuthHandler     *handlers.AuthHandler
	GroupHandler    *handlers.GroupHandler
	MediaHandler    *handlers.MediaHandler
	RoomHandler     *handlers.RoomHandler
	PresenceHub     *websocket.PresenceHub
	RoomHub         *websocket.RoomHub
	StaticDir       string
}

func NewRouter(deps RouterDependencies) http.Handler {
	mux := http.NewServeMux()
	authMW := middleware.Auth(deps.Config.JWTSecret)

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"app":    "popcornhub",
		})
	})

	// Prometheus Metrics
	mux.Handle("GET /metrics", promhttp.Handler())

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

	// Global middleware chain: CORS -> Recover -> Logger & Metrics -> Handler
	return middleware.CORS(
		middleware.Recover(
			middleware.LoggerAndMetrics(mux),
		),
	)
}
