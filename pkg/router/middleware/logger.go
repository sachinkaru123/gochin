package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sachinkaru123/gochin/pkg/router"
)

// LoggerConfig tunes request logging.
type LoggerConfig struct {
	Logger *slog.Logger
	// SkipPaths are not logged, for endpoints polled constantly by
	// load balancers.
	SkipPaths []string
}

// statusOf reports the HTTP status an error will be rendered as.
func statusOf(err error) int {
	var he *router.HTTPError
	if errors.As(err, &he) {
		return he.Status
	}
	return http.StatusInternalServerError
}

// Logger records one structured line per request.
//
// It labels each line with the matched route pattern rather than the raw URL
// path: "/api/v1/users/{id}" stays a single low-cardinality label instead of
// one label per user id.
func Logger(cfg LoggerConfig) router.Middleware {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	skip := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = true
	}

	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			if skip[c.Request.URL.Path] {
				return next(c)
			}

			start := time.Now()
			err := next(c)
			elapsed := time.Since(start)

			pattern := c.Request.Pattern
			if pattern == "" {
				pattern = c.Request.URL.Path
			}

			// A returned error has not been rendered yet — the router's error
			// handler runs outside this chain — so c.Status() is still 0 here.
			// Read the status the error will produce instead.
			status := c.Status()
			if err != nil {
				status = statusOf(err)
			}

			attrs := []any{
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"route", pattern,
				"status", status,
				"duration_ms", elapsed.Milliseconds(),
				"ip", c.ClientIP(),
			}
			if id := c.RequestID(); id != "" {
				attrs = append(attrs, "request_id", id)
			}

			// Only 5xx is an operator problem. Logging rejected logins and
			// bad input at ERROR drowns the alerts that matter.
			switch {
			case status >= 500:
				logger.Error("request failed", append(attrs, "error", err)...)
			case status >= 400:
				logger.Info("request rejected", append(attrs, "error", err)...)
			default:
				logger.Info("request", attrs...)
			}

			return err
		}
	}
}
