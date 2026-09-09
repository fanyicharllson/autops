package admin

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/fanyicharllson/autops/proxy/internal/cache"
)

type modeRequest struct {
	Tenant string `json:"tenant"`
	Mode   string `json:"mode"`
}

type modeResponse struct {
	Tenant string `json:"tenant"`
	Mode   string `json:"mode"`
}

// NewHandler returns the /admin HTTP API.
// TODO: replace shared-secret X-Admin-Token check with real auth later.
func NewHandler(adminToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/mode", requireAdmin(adminToken, handleGetMode))
	mux.HandleFunc("POST /admin/mode", requireAdmin(adminToken, handleSetMode))
	return mux
}

func requireAdmin(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validAdminToken(r.Header.Get("X-Admin-Token"), token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func validAdminToken(got, want string) bool {
	if want == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func handleGetMode(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	if tenant == "" {
		http.Error(w, "tenant query parameter is required", http.StatusBadRequest)
		return
	}

	mode, err := cache.GetMode(tenant)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, modeResponse{Tenant: tenant, Mode: mode})
}

func handleSetMode(w http.ResponseWriter, r *http.Request) {
	var req modeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Tenant == "" {
		http.Error(w, "tenant is required", http.StatusBadRequest)
		return
	}

	if err := cache.SetMode(req.Tenant, req.Mode); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mode, err := cache.GetMode(req.Tenant)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, modeResponse{Tenant: req.Tenant, Mode: mode})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
