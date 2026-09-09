package cache

// GetMode returns the current traffic-shaping mode for a tenant.
// TODO: replace this stub with a Redis-backed decision cache that is
// updated by the control-plane forecaster (async control loop).
// Callers must fail open: on error or unrecognized mode, treat as "normal".
func GetMode(tenant string) (string, error) {
	_ = tenant
	return "normal", nil
}
