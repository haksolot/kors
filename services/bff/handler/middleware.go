package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/haksolot/kors/libs/core"
)

// ── Context key ────────────────────────────────────────────────────────────────

type contextKey int

const claimsKey contextKey = iota

func claimsFromCtx(r *http.Request) *core.Claims {
	c, _ := r.Context().Value(claimsKey).(*core.Claims)
	return c
}

func contextWithClaims(ctx context.Context, c *core.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ── JSON helpers ───────────────────────────────────────────────────────────────

var protoJSONMarshal = protojson.MarshalOptions{
	UseProtoNames:   true,
	EmitUnpopulated: false,
}

var protoJSONUnmarshal = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

func writeJSON(w http.ResponseWriter, status int, msg proto.Message) {
	b, err := protoJSONMarshal.Marshal(msg)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":` + `"` + jsonEscape(msg) + `"}`))
}

func unmarshalBody(r *http.Request, msg proto.Message) error {
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return protoJSONUnmarshal.Unmarshal(b, msg)
}

func jsonEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// ── Auth middleware ────────────────────────────────────────────────────────────

// AuthMiddleware validates the Bearer token on every request and injects claims
// into the context. Returns 401 if the token is missing or invalid.
func AuthMiddleware(v *core.JWTValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}
			token := header[len("Bearer "):]
			claims, err := v.ValidateJWT(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := r.Context()
			ctx = contextWithClaims(ctx, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ── Logging middleware ─────────────────────────────────────────────────────────

func LoggingMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.status).
				Dur("duration", time.Since(start)).
				Msg("http")
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// ── Metrics middleware ─────────────────────────────────────────────────────────

func MetricsMiddleware(reg prometheus.Registerer) func(http.Handler) http.Handler {
	httpDuration := promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kors_bff_http_request_duration_seconds",
		Help:    "HTTP request latency by method and path pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			httpDuration.WithLabelValues(
				r.Method,
				r.URL.Path,
				http.StatusText(rw.status),
			).Observe(time.Since(start).Seconds())
		})
	}
}
