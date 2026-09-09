package router

import (
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
		mode := cache.GetMode(tenant)

		if mode != "normal" {
			// TODO: implement shaping / waiting-room path.
			http.Error(w, "shaping mode not implemented", http.StatusServiceUnavailable)
			return
		}

		proxy.ServeHTTP(w, r)
	}), nil
}
