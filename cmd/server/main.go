package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"popcornhub/internal/api"
	"popcornhub/internal/api/handlers"
	"popcornhub/internal/config"
	"popcornhub/internal/database"
	"popcornhub/internal/repository"
	"popcornhub/internal/service"
	"popcornhub/internal/websocket"
)

func main() {
	cfg := config.Load()

	// Initialize structured logger with configured LogLevel
	var logLevel slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))

	slog.Info("Starting PopcornHub collaborative streaming server...", "log_level", cfg.LogLevel)

	if cfg.JWTSecret == "super-secret-popcorn-key-change-in-prod-2026" {
		slog.Warn("USING DEFAULT INSECURE JWT SECRET! Change JWT_SECRET in production!")
	}

	// Context for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Connect to MongoDB
	mongoDB, err := database.ConnectMongoDB(ctx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		slog.Error("Failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		_ = mongoDB.Close(closeCtx)
	}()

	// Connect to Redis
	redisClient, err := database.ConnectRedis(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// Initialize repositories
	userRepo := repository.NewUserRepo(mongoDB.Database)
	groupRepo := repository.NewGroupRepo(mongoDB.Database)
	mediaRepo := repository.NewMediaRepo(mongoDB.Database)
	roomRepo := repository.NewRoomRepo(mongoDB.Database)

	// Initialize services
	authService := service.NewAuthService(userRepo, cfg)
	groupService := service.NewGroupService(groupRepo, userRepo)
	mediaService := service.NewMediaService(mediaRepo)
	roomService := service.NewRoomService(roomRepo, groupRepo, mediaRepo)
	presenceService := service.NewPresenceService(redisClient.Client)

	// Initialize WebSocket hubs
	presenceHub := websocket.NewPresenceHub(presenceService, groupRepo, cfg)
	roomHub := websocket.NewRoomHub(roomRepo, groupRepo, mediaRepo, userRepo, presenceService, redisClient.Client, cfg)

	// Initialize HTTP handlers
	authHandler := handlers.NewAuthHandler(authService)
	groupHandler := handlers.NewGroupHandler(groupService, presenceService)
	mediaHandler := handlers.NewMediaHandler(mediaService)
	roomHandler := handlers.NewRoomHandler(roomService)

	// Determine static files directory
	staticDir := "web/static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// Try absolute path or relative to binary
		execPath, _ := os.Executable()
		candidate := filepath.Join(filepath.Dir(execPath), "web/static")
		if _, err := os.Stat(candidate); err == nil {
			staticDir = candidate
		}
	}

	// Build HTTP router
	router := api.NewRouter(api.RouterDependencies{
		Config:       cfg,
		AuthHandler:  authHandler,
		GroupHandler: groupHandler,
		MediaHandler: mediaHandler,
		RoomHandler:  roomHandler,
		PresenceHub:  presenceHub,
		RoomHub:      roomHub,
		StaticDir:    staticDir,
		MongoDB:      mongoDB,
		RedisClient:  redisClient,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server runner
	go func() {
		slog.Info(fmt.Sprintf("PopcornHub server listening on http://localhost:%s", cfg.Port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("Shutting down PopcornHub server...", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("PopcornHub stopped cleanly")
}
