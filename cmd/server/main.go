package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"splat-boston/internal/api"
	"splat-boston/internal/geo"
	redisclient "splat-boston/internal/redis"
	"splat-boston/internal/ws"
)

func main() {
	// Load configuration from environment
	config := api.Config{
		EnableTurnstile: getEnvBool("ENABLE_TURNSTILE", false),
		TurnstileSecret: getEnv("TURNSTILE_SECRET", ""),
		GeofenceRadiusM: getEnvFloat("GEOFENCE_RADIUS_M", 300.0),
		SpeedMaxKmh:     getEnvFloat("SPEED_MAX_KMH", 150.0),
		PaintCooldownMs: getEnvInt("PAINT_COOLDOWN_MS", 5000),
		WSWriteBuffer:   getEnvInt("WS_WRITE_BUFFER", 1048576),
		WSPingIntervalS: getEnvInt("WS_PING_INTERVAL_S", 20),
	}

	bindAddr := getEnv("BIND_ADDR", ":8080")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")

	// Connect to Redis
	rdb, err := redisclient.NewClient(redisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()

	log.Println("Connected to Redis")

	// Create WebSocket hub
	hub := ws.NewHub()
	go hub.Run()

	log.Println("WebSocket hub started")

	// Load mask (optional - for now we'll use nil)
	var mask *geo.Mask = nil

	// Create handler
	handler := api.NewHandler(rdb, hub, config, mask)

	// CORS middleware
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Allow requests from any origin in development
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			
			// Handle preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next(w, r)
		}
	}

	// Setup routes with CORS
	http.HandleFunc("/state/chunk", corsMiddleware(handler.GetChunk))
	http.HandleFunc("/paint", corsMiddleware(handler.PostPaint))
	http.HandleFunc("/sub", corsMiddleware(handler.HandleWebSocket))

	// Health check endpoint
	http.HandleFunc("/healthz", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if err := rdb.Ping(); err != nil {
			http.Error(w, "Redis unhealthy", 500)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))

	// Start server
	log.Printf("Starting server on %s", bindAddr)
	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
