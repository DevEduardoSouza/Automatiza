package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
	IsDevelopment    bool
}

// DefaultCORSConfig returns the default CORS configuration
func DefaultCORSConfig(corsOrigin string, isDevelopment bool) *CORSConfig {
	origins := []string{}
	if corsOrigin != "" {
		origins = strings.Split(corsOrigin, ",")
	}

	return &CORSConfig{
		AllowedOrigins: origins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Requested-With",
			"x-client-info",
			"apikey",
			"X-Api-Key",
			// ConnectRPC headers
			"Connect-Protocol-Version",
			"Connect-Timeout-Ms",
			"X-Csrf-Token",
			"Grpc-Timeout",
			// Tracing headers (W3C Trace Context)
			"traceparent",
			"tracestate",
			"x-trace-id",
		},
		ExposedHeaders: []string{
			"Content-Length",
			"Content-Type",
			// ConnectRPC headers
			"Grpc-Status",
			"Grpc-Message",
			"Connect-Content-Encoding",
		},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
		IsDevelopment:    isDevelopment,
	}
}

// CORSMiddleware creates the CORS middleware
func CORSMiddleware(cfg *CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Handle requests without Origin (mobile apps, Postman, etc.)
		if origin == "" {
			c.Next()
			return
		}

		// Check if origin is allowed
		allowed := false

		// In development, allow localhost
		if cfg.IsDevelopment {
			if strings.HasPrefix(origin, "http://localhost") ||
				strings.HasPrefix(origin, "http://127.0.0.1") ||
				strings.HasPrefix(origin, "https://localhost") {
				allowed = true
			}
		}

		// Check configured origins
		if !allowed {
			for _, allowedOrigin := range cfg.AllowedOrigins {
				allowedOrigin = strings.TrimSpace(allowedOrigin)
				if origin == allowedOrigin {
					allowed = true
					break
				}
				// Support wildcard subdomains
				if strings.HasPrefix(allowedOrigin, "*.") {
					domain := allowedOrigin[2:]
					if strings.HasSuffix(origin, domain) {
						allowed = true
						break
					}
				}
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))

			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}

			if cfg.MaxAge > 0 {
				c.Header("Access-Control-Max-Age", itoa(cfg.MaxAge))
			}
		}

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// HTTPCORSMiddleware creates a standard HTTP CORS middleware for non-Gin handlers (e.g., ConnectRPC)
func HTTPCORSMiddleware(cfg *CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Handle requests without Origin (mobile apps, Postman, etc.)
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if origin is allowed
			allowed := false

			// In development, allow localhost
			if cfg.IsDevelopment {
				if strings.HasPrefix(origin, "http://localhost") ||
					strings.HasPrefix(origin, "http://127.0.0.1") ||
					strings.HasPrefix(origin, "https://localhost") {
					allowed = true
				}
			}

			// Check configured origins
			if !allowed {
				for _, allowedOrigin := range cfg.AllowedOrigins {
					allowedOrigin = strings.TrimSpace(allowedOrigin)
					if origin == allowedOrigin {
						allowed = true
						break
					}
					// Support wildcard subdomains
					if strings.HasPrefix(allowedOrigin, "*.") {
						domain := allowedOrigin[2:]
						if strings.HasSuffix(origin, domain) {
							allowed = true
							break
						}
					}
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))

				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if cfg.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", itoa(cfg.MaxAge))
				}
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// itoa converts int to string (simple implementation)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	neg := false
	if i < 0 {
		neg = true
		i = -i
	}

	var b [20]byte
	bp := len(b) - 1

	for i > 0 {
		b[bp] = byte('0' + i%10)
		bp--
		i /= 10
	}

	if neg {
		b[bp] = '-'
		bp--
	}

	return string(b[bp+1:])
}
