package cache

// GetMode returns the current traffic-shaping mode for a tenant.
// TODO: replace this stub with a Redis-backed decision cache that is
// updated by the control-plane forecaster (async control loop).
func GetMode(tenant string) string {
	_ = tenant
	return "normal"
}
