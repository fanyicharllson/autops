package main

import (
	"log"
	"net/http"

	"github.com/fanyicharllson/autops/proxy/internal/config"
	"github.com/fanyicharllson/autops/proxy/internal/middleware"
	"github.com/fanyicharllson/autops/proxy/internal/router"
)

func main() {
	cfg := config.Load()

	handler, err := router.New(cfg.BackendURL)
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	addr := cfg.ListenAddr
	log.Printf("autops proxy listening on %s (backend %s)", addr, cfg.BackendURL)
	if err := http.ListenAndServe(addr, middleware.Logging(handler)); err != nil {
		log.Fatalf("server: %v", err)
	}
}
