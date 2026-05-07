package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gotify-relay/internal/callers"
	"gotify-relay/internal/config"
	"gotify-relay/internal/relay"
	"gotify-relay/internal/subscriptions"
)

func main() {
	configPath := flag.String("config", getenv("CONFIG_PATH", "config.yaml"), "path to relay config YAML")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if listenAddr := os.Getenv("LISTEN_ADDR"); listenAddr != "" {
		cfg.Server.ListenAddr = listenAddr
	}

	gotifyClient := relay.NewGotifyClient(cfg.Gotify.URL, &http.Client{
		Timeout: 10 * time.Second,
	})
	subscriptionStore, err := subscriptions.NewJSONStore(cfg.Subscriptions.Path, cfg)
	if err != nil {
		log.Fatalf("load subscriptions: %v", err)
	}

	callerTokenPath := filepath.Join(filepath.Dir(cfg.Subscriptions.Path), "caller-tokens.json")
	callerStore, err := callers.NewJSONStore(callerTokenPath, cfg)
	if err != nil {
		log.Fatalf("load caller tokens: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", relay.NewHandler(cfg, subscriptionStore, gotifyClient, callerStore))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	server := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("gotify-relay listening on %s", cfg.Server.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
