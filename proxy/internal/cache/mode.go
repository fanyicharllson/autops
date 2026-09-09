package cache

import (
	"fmt"
	"sync"
)

// Temporary in-memory decision cache. This will be replaced by a Redis-backed
// cache in the next milestone (updated by the control-plane forecaster).
// Callers must fail open: on error or unrecognized mode, treat as "normal".
var (
	mu    sync.RWMutex
	modes = make(map[string]string)
)

// GetMode returns the current traffic-shaping mode for a tenant.
// Tenants with no entry default to "normal".
func GetMode(tenant string) (string, error) {
	mu.RLock()
	defer mu.RUnlock()
	if mode, ok := modes[tenant]; ok {
		return mode, nil
	}
	return "normal", nil
}

// SetMode updates the traffic-shaping mode for a tenant.
// Valid modes are "normal" and "shaping".
func SetMode(tenant, mode string) error {
	if tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	switch mode {
	case "normal", "shaping":
	default:
		return fmt.Errorf("invalid mode %q: must be \"normal\" or \"shaping\"", mode)
	}

	mu.Lock()
	defer mu.Unlock()
	modes[tenant] = mode
	return nil
}
