// Package bootstrap wires framework layers together at startup.
//
// It is the only place allowed to import both pkg/router and pkg/orm: keeping
// the HTTP layer free of persistence imports is what lets pkg/router stay
// reusable.
package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"

	"github.com/lib/pq"

	"github.com/gochin/framework/pkg/orm"
	"github.com/gochin/framework/pkg/router"
)

var registerOnce sync.Once

// RegisterErrorMappers teaches the router how to render ORM and Postgres
// errors, so controllers can return them directly and still get correct
// status codes.
func RegisterErrorMappers() {
	registerOnce.Do(func() {
		router.RegisterErrorMapper(mapORMError)
		router.RegisterErrorMapper(mapPostgresError)
	})
}

func mapORMError(err error) *router.HTTPError {
	switch {
	case errors.Is(err, orm.ErrRecordNotFound), errors.Is(err, sql.ErrNoRows):
		return router.NotFoundf("resource not found")
	case errors.Is(err, orm.ErrMissingPrimaryKey):
		return router.BadRequestf("missing resource identifier")
	case errors.Is(err, orm.ErrTooManyRows):
		return router.BadRequestf("result set too large; use pagination")
	case errors.Is(err, context.DeadlineExceeded):
		return router.NewError(http.StatusGatewayTimeout, "request timed out")
	case errors.Is(err, context.Canceled):
		// The client is already gone; 499 keeps these out of the 5xx budget.
		return router.NewError(499, "client closed request")
	}
	return nil
}

// mapPostgresError turns constraint violations into the 4xx they actually
// are. Without this a duplicate email surfaces as an opaque 500.
func mapPostgresError(err error) *router.HTTPError {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return nil
	}

	switch pqErr.Code {
	case "23505": // unique_violation
		return router.Conflictf("resource already exists").WithCode("duplicate_key")
	case "23503": // foreign_key_violation
		return router.BadRequestf("referenced resource does not exist").WithCode("foreign_key_violation")
	case "23502": // not_null_violation
		return router.BadRequestf("field %q is required", pqErr.Column).WithCode("not_null_violation")
	case "23514": // check_violation
		return router.BadRequestf("value violates constraint %q", pqErr.Constraint).WithCode("check_violation")
	case "22P02": // invalid_text_representation
		return router.BadRequestf("invalid input syntax").WithCode("invalid_syntax")
	case "57014": // query_canceled
		// A cancelled query reaches us as a driver error, not as a wrapped
		// context error, so it needs its own case to avoid becoming a 500.
		return router.NewError(http.StatusGatewayTimeout, "query cancelled").WithCode("timeout")
	case "53300": // too_many_connections
		return router.NewError(http.StatusServiceUnavailable, "database unavailable").WithCode("db_unavailable")
	}
	return nil
}
