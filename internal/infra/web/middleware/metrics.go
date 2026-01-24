package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/DioGolang/GoFleet/pkg/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Pre-alocação de strings para Status Codes comuns (100-599)
// Isso evita milhares de alocações de string via strconv.Itoa por segundo.
var statusStrings [600]string

func init() {
	for i := 100; i < 600; i++ {
		statusStrings[i] = strconv.Itoa(i)
	}
}

func getStatusString(code int) string {
	if code >= 100 && code < 600 {
		return statusStrings[code]
	}
	return strconv.Itoa(code)
}

func MetricsWrapper(m metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				path := chi.RouteContext(r.Context()).RoutePattern()
				if path == "" {
					path = "unknown"
				}

				duration := time.Since(start).Seconds()
				status := getStatusString(ww.Status())
				m.ObserveHTTPRequestDuration(r.Method, path, status, duration)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
