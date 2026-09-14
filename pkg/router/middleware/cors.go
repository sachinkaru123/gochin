package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gochin/framework/pkg/router"
)

// CORSConfig configures cross-origin access.
type CORSConfig struct {
	// AllowedOrigins lists permitted origins, or a single "*" for any.
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposedHeaders []string
	// AllowCredentials cannot be combined with a "*" origin: browsers reject
	// that pairing, so CORS silently enables it by echoing the request origin.
	AllowCredentials bool
	MaxAge           int
}

// CORS adds cross-origin headers and answers preflight requests.
func CORS(cfg CORSConfig) router.Middleware {
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodOptions,
		}
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"Content-Type", "Authorization", HeaderRequestID}
	}

	allowAll := len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*"
	allowed := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposed := strings.Join(cfg.ExposedHeaders, ", ")

	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			origin := c.Request.Header.Get("Origin")
			if origin == "" {
				return next(c)
			}

			// Responses vary by Origin even when rejected, or a shared cache
			// can serve one origin's response to another.
			c.Header().Add("Vary", "Origin")

			switch {
			case allowAll && !cfg.AllowCredentials:
				c.Header().Set("Access-Control-Allow-Origin", "*")
			case allowAll || allowed[origin]:
				c.Header().Set("Access-Control-Allow-Origin", origin)
			default:
				return next(c)
			}

			if cfg.AllowCredentials {
				c.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if exposed != "" {
				c.Header().Set("Access-Control-Expose-Headers", exposed)
			}

			if c.Request.Method == http.MethodOptions {
				c.Header().Set("Access-Control-Allow-Methods", methods)
				c.Header().Set("Access-Control-Allow-Headers", headers)
				if cfg.MaxAge > 0 {
					c.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
				}
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}
