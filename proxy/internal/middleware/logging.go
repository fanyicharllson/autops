// Package middleware provides HTTP middleware for the AutOps proxy.
//
// Request log levels (structured JSON via log/slog):
//   - INFO:   status < 400 and latency_ms < 500
//   - WARN:   status < 500 and latency_ms >= 500 (backend slowing down)
//   - ACTION: status >= 500 (backend error/unreachable) OR latency_ms >= 2000
//     (severe degradation); includes a human-readable "message" field
//
// Security: never log PII or secrets. No raw client IPs (only a truncated
// SHA-256 client_id), no headers (Authorization/Cookie/Set-Cookie), no
// query strings, and no request/response bodies.
package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
)

// LevelAction is a custom slog level for conditions that need operator attention.
const LevelAction = slog.Level(6) // between WARN (4) and ERROR (8)

var requestLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{Key: "timestamp", Value: a.Value}
		}
		if a.Key == slog.LevelKey {
			if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == LevelAction {
				return slog.String(slog.LevelKey, "ACTION")
			}
		}
		return a
	},
}))

// Logging wraps next with structured, security-conscious request logging.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		latencyMs := time.Since(start).Milliseconds()
		path := r.URL.Path // query string intentionally omitted
		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("path", path),
			slog.Int("status_code", sw.status),
			slog.Int64("latency_ms", latencyMs),
			slog.String("client_id", hashClientID(r.RemoteAddr)),
		}

		switch {
		case sw.status >= 500 || latencyMs >= 2000:
			msg := actionMessage(sw.status, latencyMs)
			requestLogger.LogAttrs(r.Context(), LevelAction, "action required",
				append(attrs, slog.String("message", msg))...)
		case sw.status < 500 && latencyMs >= 500:
			requestLogger.LogAttrs(r.Context(), slog.LevelWarn, "request", attrs...)
		default:
			requestLogger.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs...)
		}
	})
}

func actionMessage(status int, latencyMs int64) string {
	switch {
	case status >= 500 && latencyMs >= 2000:
		return "backend unreachable or error; latency threshold breached"
	case status >= 500:
		return "backend unreachable"
	default:
		return "latency threshold breached"
	}
}

func hashClientID(remoteAddr string) string {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	sum := sha256.Sum256([]byte(host))
	return hex.EncodeToString(sum[:])[:12]
}

// statusWriter captures the HTTP status code written by downstream handlers.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	// Ensure status is set if Write is called without WriteHeader.
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
