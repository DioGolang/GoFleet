package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/DioGolang/GoFleet/pkg/logger"
	"golang.org/x/time/rate"
)

type RateLimiterConfig struct {
	RequestsPerSecond int           // Quantos tokens entram por segundo (r)
	Burst             int           // Tamanho máximo do balde (b)
	CleanupInterval   time.Duration // Frequência de limpeza de IPs inativos
	ClientTimeout     time.Duration // Tempo para considerar um IP inativo
}

type IPDispatcher struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	config   RateLimiterConfig
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(conf RateLimiterConfig) *IPDispatcher {
	d := &IPDispatcher{
		visitors: make(map[string]*visitor),
		config:   conf,
	}

	go d.cleanupLoop()

	return d
}

func (d *IPDispatcher) cleanupLoop() {
	ticker := time.NewTicker(d.config.CleanupInterval)
	for range ticker.C {
		d.mu.Lock()
		for ip, v := range d.visitors {
			if time.Since(v.lastSeen) > d.config.ClientTimeout {
				delete(d.visitors, ip)
			}
		}
		d.mu.Unlock()
	}
}

func (d *IPDispatcher) Handler(log logger.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			// Se estiver atrás de proxy (Nginx/Cloudflare), use X-Forwarded-For:
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}

			limiter := d.getVisitor(ip)

			if !limiter.Allow() {
				log.Warn(r.Context(), "Rate limit exceeded",
					logger.String("ip", ip),
					logger.String("path", r.URL.Path),
				)

				w.Header().Set("Retry-After", "1")
				http.Error(w, "Too Many Requests - Slow down", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (d *IPDispatcher) getVisitor(ip string) *rate.Limiter {
	d.mu.Lock()
	defer d.mu.Unlock()

	v, exists := d.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(d.config.RequestsPerSecond), d.config.Burst)
		d.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}
