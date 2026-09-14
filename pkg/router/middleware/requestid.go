package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/sachinkaru123/gochin/pkg/router"
)

// HeaderRequestID is the header carrying the correlation id.
const HeaderRequestID = "X-Request-Id"

// RequestID assigns every request a correlation id, echoes it in the response
// and makes it available to logs and error envelopes.
//
// trustInbound should only be set behind a proxy that sanitizes the header;
// otherwise a client can pick its own id and poison log correlation.
func RequestID(trustInbound bool) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			id := ""
			if trustInbound {
				id = c.Request.Header.Get(HeaderRequestID)
			}
			if id == "" {
				id = newRequestID()
			}

			c.SetRequestID(id)
			c.Header().Set(HeaderRequestID, id)
			return next(c)
		}
	}
}

func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf[:])
}
