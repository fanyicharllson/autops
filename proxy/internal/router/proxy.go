package router

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/fanyicharllson/autops/proxy/internal/cache"
)

// New creates a reverse-proxy handler that checks the tenant mode
// via cache.GetMode before forwarding to the backend.
// TODO: when mode is "shaping", route into the virtual queue instead of proxying.
func New(backendURL string) (http.Handler, error) {
	target, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: extract real tenant ID from request (header / host / path).
		tenant := "default"
		mode, err := cache.GetMode(tenant)

		// Fail open: never block or enter shaping on cache failure / bad values.
		if err != nil || !recognizedMode(mode) {
			slog.Warn("decision cache fallback to normal",
				"tenant", tenant,
				"mode", mode,
				"err", err,
			)
			mode = "normal"
		}

		if mode == "shaping" {
			// TODO: implement shaping / waiting-room path.
			http.Error(w, "shaping mode not implemented", http.StatusServiceUnavailable)
			return
		}

		proxy.ServeHTTP(w, r)
	}), nil
}

func recognizedMode(mode string) bool {
	switch mode {
	case "normal", "shaping":
		return true
	default:
		return false
	}
}
