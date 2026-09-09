package config

import "os"

// Config holds runtime settings for the reverse proxy.
type Config struct {
	BackendURL string
	ListenAddr string
}

// Load reads configuration from environment variables with sane localhost defaults.
func Load() Config {
	return Config{
		BackendURL: getEnv("BACKEND_URL", "http://localhost:3000"),
		ListenAddr: getEnv("LISTEN_ADDR", ":8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
