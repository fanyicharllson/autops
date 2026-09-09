package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// TenantConfig describes a single host-based tenant backend.
type TenantConfig struct {
	Domain     string `json:"domain"`
	BackendURL string `json:"backend_url"`
}

// Config holds runtime settings for the reverse proxy.
type Config struct {
	ListenAddr string
	// Tenants is keyed by Domain (hostname without port).
	// TODO: replace this in-memory map with a real tenant registry.
	Tenants map[string]TenantConfig
}

// Load reads ListenAddr and the tenant map from the environment.
// If TENANT_CONFIG_JSON is set, it must be a path to a JSON file containing
// an array of TenantConfig objects. Otherwise a temporary single-tenant
// default is used (BACKEND_URL / TENANT_DOMAIN).
func Load() (Config, error) {
	cfg := Config{
		ListenAddr: getEnv("LISTEN_ADDR", ":8080"),
		Tenants:    make(map[string]TenantConfig),
	}

	if path := os.Getenv("TENANT_CONFIG_JSON"); path != "" {
		tenants, err := loadTenantsFile(path)
		if err != nil {
			return Config{}, err
		}
		cfg.Tenants = tenants
		return cfg, nil
	}

	// TODO: temporary hardcoded single-tenant map until a real tenant registry exists.
	domain := getEnv("TENANT_DOMAIN", "localhost")
	backend := getEnv("BACKEND_URL", "http://localhost:3000")
	cfg.Tenants[domain] = TenantConfig{
		Domain:     domain,
		BackendURL: backend,
	}
	return cfg, nil
}

func loadTenantsFile(path string) (map[string]TenantConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read TENANT_CONFIG_JSON %q: %w", path, err)
	}

	var list []TenantConfig
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse TENANT_CONFIG_JSON %q: %w", path, err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("TENANT_CONFIG_JSON %q: no tenants defined", path)
	}

	tenants := make(map[string]TenantConfig, len(list))
	for _, t := range list {
		if t.Domain == "" || t.BackendURL == "" {
			return nil, fmt.Errorf("TENANT_CONFIG_JSON %q: tenant requires domain and backend_url", path)
		}
		tenants[t.Domain] = t
	}
	return tenants, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
