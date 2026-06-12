package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"crypto/tls"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
var rdb *redis.Client

func main() {
	// Read from environment variables (injected by ECS Task Definition)
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisAuth := os.Getenv("REDIS_AUTH_TOKEN")

	if redisHost == "" {
		log.Fatal("REDIS_HOST environment variable not set")
	}
	if redisPort == "" {
		redisPort = "6379"
	}

	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	// Build Redis client

	opts := &redis.Options{
    Addr: addr,
    Password: redisAuth,
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
}

	rdb = redis.NewClient(opts)

	// Test connection on startup
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("❌ Could not connect to ElastiCache: %v", err)
	}
	log.Println("✅ Connected to ElastiCache successfully")

	// HTTP routes
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/set", handleSet)
	http.HandleFunc("/get", handleGet)
	http.HandleFunc("/health", handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Server running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// GET / — shows all test results
func handleHome(w http.ResponseWriter, r *http.Request) {
	results := ""

	// PING
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		results += fmt.Sprintf("❌ PING failed: %v\n", err)
	} else {
		results += fmt.Sprintf("✅ PING: %s\n", pong)
	}

	// SET
	err = rdb.Set(ctx, "testkey", "hello-from-ecs", 60*time.Second).Err()
	if err != nil {
		results += fmt.Sprintf("❌ SET failed: %v\n", err)
	} else {
		results += "✅ SET testkey = hello-from-ecs (TTL: 60s)\n"
	}

	// GET
	val, err := rdb.Get(ctx, "testkey").Result()
	if err != nil {
		results += fmt.Sprintf("❌ GET failed: %v\n", err)
	} else {
		results += fmt.Sprintf("✅ GET testkey = %s\n", val)
	}

	// TTL
	ttl, err := rdb.TTL(ctx, "testkey").Result()
	if err != nil {
		results += fmt.Sprintf("❌ TTL failed: %v\n", err)
	} else {
		results += fmt.Sprintf("✅ TTL testkey = %s\n", ttl)
	}

	// Server info
	info, err := rdb.Info(ctx, "server").Result()
	if err != nil {
		results += fmt.Sprintf("❌ INFO failed: %v\n", err)
	} else {
		results += fmt.Sprintf("✅ INFO server:\n%s\n", info)
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, results)
}

// GET /set?key=foo&value=bar
func handleSet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	value := r.URL.Query().Get("value")

	if key == "" || value == "" {
		http.Error(w, "key and value query params required", http.StatusBadRequest)
		return
	}

	err := rdb.Set(ctx, key, value, 60*time.Second).Err()
	if err != nil {
		http.Error(w, fmt.Sprintf("❌ SET failed: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "✅ SET %s = %s (TTL: 60s)", key, value)
}

// GET /get?key=foo
func handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key query param required", http.StatusBadRequest)
		return
	}

	val, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		fmt.Fprintf(w, "⚠️  Key '%s' not found (nil)", key)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("❌ GET failed: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "✅ GET %s = %s", key, val)
}

// GET /health — ECS health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "unhealthy: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "healthy")
}