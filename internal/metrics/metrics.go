package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ActiveWebSockets = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "popcornhub_active_websockets",
			Help: "Current number of active WebSocket connections by type (presence, room)",
		},
		[]string{"type"},
	)

	RoomActionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "popcornhub_room_actions_total",
			Help: "Total count of collaborative actions in watch rooms",
		},
		[]string{"action"}, // "play", "pause", "seek", "chat", "reaction"
	)

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "popcornhub_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "popcornhub_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	SyncLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "popcornhub_sync_latency_seconds",
			Help:    "Latency of synchronization broadcast in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
	)
)
