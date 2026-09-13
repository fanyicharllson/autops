package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/fanyicharllson/autops/proxy/internal/admin"
	"github.com/fanyicharllson/autops/proxy/internal/cache"
	"github.com/fanyicharllson/autops/proxy/internal/config"
	"github.com/fanyicharllson/autops/proxy/internal/metrics"
	"github.com/fanyicharllson/autops/proxy/internal/middleware"
	"github.com/fanyicharllson/autops/proxy/internal/queue"
	"github.com/fanyicharllson/autops/proxy/internal/router"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("close redis client: %v", err)
		}
	}()
	cache.SetClient(redisClient)
	metrics.SetClient(redisClient)
	queue.SetClient(redisClient)

	tenants := make([]string, 0, len(cfg.Tenants))
	for domain := range cfg.Tenants {
		tenants = append(tenants, domain)
	}
	cache.SeedTenants(tenants)
	queue.ConfigureRelease(tenants, cfg.ReleaseIntervalSeconds, cfg.ReleaseBatchSize)

	bgCtx, cancelBG := context.WithCancel(context.Background())
	defer cancelBG()
	go cache.StartRefreshLoop(bgCtx)
	go queue.StartReleaseLoop(bgCtx)

	proxyHandler, err := router.New(cfg.Tenants)
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/admin/", admin.NewHandler(os.Getenv("ADMIN_TOKEN")))
	mux.Handle("/queue/", queue.NewHandler())
	mux.Handle("/", proxyHandler)

	addr := cfg.ListenAddr
	log.Printf("autops proxy listening on %s (%d tenant(s))", addr, len(cfg.Tenants))
	if err := http.ListenAndServe(addr, middleware.Logging(mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
