package router

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/fanyicharllson/autops/proxy/internal/cache"
	"github.com/fanyicharllson/autops/proxy/internal/config"
	"github.com/fanyicharllson/autops/proxy/internal/middleware"
)

const backendTimeout = 10 * time.Second

type tenantHandler struct {
	cfg   config.TenantConfig
	proxy *httputil.ReverseProxy
}

// New builds a host-based multi-tenant reverse proxy.
// Tenants are looked up by the incoming Host header (port stripped).
func New(tenants map[string]config.TenantConfig) (http.Handler, error) {
	if len(tenants) == 0 {
		return nil, errors.New("no tenants configured")
	}

	byHost := make(map[string]tenantHandler, len(tenants))
	for domain, tc := range tenants {
		target, err := url.Parse(tc.BackendURL)
		if err != nil {
			return nil, err
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		tenantDomain := tc.Domain
		backendURL := tc.BackendURL
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			middleware.LogAction("backend unreachable",
				"message", "backend unreachable",
				"tenant", tenantDomain,
				"backend_url", backendURL,
				"error", err.Error(),
			)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("backend unreachable"))
		}
		byHost[strings.ToLower(domain)] = tenantHandler{
			cfg:   tc,
			proxy: proxy,
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := requestHost(r.Host)
		th, ok := byHost[host]
		if !ok {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		mode, err := cache.GetMode(th.cfg.Domain)

		// Fail open: never block or enter shaping on cache failure / bad values.
		if err != nil || !recognizedMode(mode) {
			slog.Warn("decision cache fallback to normal",
				"tenant", th.cfg.Domain,
				"mode", mode,
				"err", err,
			)
			mode = "normal"
		}

		if mode == "shaping" {
			// TODO: replace with virtual queue / waiting-room in a later milestone.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("Shaping mode active -- queueing not yet implemented"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), backendTimeout)
		defer cancel()
		th.proxy.ServeHTTP(w, r.WithContext(ctx))
	}), nil
}

func requestHost(hostport string) string {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	return strings.ToLower(host)
}

func recognizedMode(mode string) bool {
	switch mode {
	case "normal", "shaping":
		return true
	default:
		return false
	}
}
