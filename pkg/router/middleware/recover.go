// Package middleware provides the standard middleware Gochin ships with.
//
// Recommended order, outermost first:
//
//	RequestID -> Logger -> Recover -> CORS -> RateLimit -> Timeout
package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gochin/framework/pkg/router"
)

// Recover converts a panic into a normal 500 error return.
//
// It sits inside Logger deliberately: converting the panic here lets the
// stack unwind normally, so Logger observes the real status instead of
// running its deferred log mid-panic.
func Recover(debugMode bool) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *router.Context) (err error) {
			defer func() {
				p := recover()
				if p == nil {
					return
				}
				// http.ErrAbortHandler is the stdlib's signal to drop the
				// response silently; swallowing it would break that contract.
				if p == http.ErrAbortHandler {
					panic(p)
				}

				stack := debug.Stack()
				slog.Error("recovered from panic",
					"panic", p,
					"method", c.Request.Method,
					"pattern", c.Request.Pattern,
					"request_id", c.RequestID(),
					"stack", string(stack),
				)

				cause := fmt.Errorf("panic: %v", p)
				if debugMode {
					err = router.Internalf("panic: %v", p).Wrap(fmt.Errorf("%v\n%s", p, stack))
					return
				}
				err = router.Internalf("internal server error").Wrap(cause)
			}()

			return next(c)
		}
	}
}
