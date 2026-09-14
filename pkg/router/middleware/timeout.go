package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/gochin/framework/pkg/router"
)

// Timeout bounds how long a handler may run.
//
// It attaches a deadline to the request context rather than using
// http.TimeoutHandler, which buffers the whole response in memory to be able
// to replace it — that defeats streaming responses. The deadline propagates
// into the ORM, so an expired request also cancels its Postgres query.
func Timeout(d time.Duration) router.Middleware {
	if d <= 0 {
		return func(next router.Handler) router.Handler { return next }
	}

	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			ctx, cancel := context.WithTimeout(c.Ctx(), d)
			defer cancel()

			c.SetRequest(c.Request.WithContext(ctx))

			err := next(c)
			if err == nil && ctx.Err() != nil && !c.Written() {
				return router.NewError(504, "request timed out")
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return router.NewError(504, "request timed out").Wrap(err)
			}
			return err
		}
	}
}
