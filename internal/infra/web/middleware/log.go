package middleware

import (
	"github.com/DioGolang/GoFleet/pkg/logger"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
	"time"
)

func RequestLogger(log logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info(r.Context(), "http request processed",
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path),
				logger.Int("status", ww.Status()),
				logger.Int("bytes", ww.BytesWritten()),
				logger.Any("latency", time.Since(start).String()),
			)
		})
	}
}
