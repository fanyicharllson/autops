package main

import (
	"log"
	"net/http"
	"os"

	"github.com/fanyicharllson/autops/proxy/internal/admin"
	"github.com/fanyicharllson/autops/proxy/internal/config"
	"github.com/fanyicharllson/autops/proxy/internal/middleware"
	"github.com/fanyicharllson/autops/proxy/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	proxyHandler, err := router.New(cfg.Tenants)
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/admin/", admin.NewHandler(os.Getenv("ADMIN_TOKEN")))
	mux.Handle("/", proxyHandler)

	addr := cfg.ListenAddr
	log.Printf("autops proxy listening on %s (%d tenant(s))", addr, len(cfg.Tenants))
	if err := http.ListenAndServe(addr, middleware.Logging(mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
