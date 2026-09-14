package router

import (
	"fmt"
	"net/http"
	"sync"
)

// HTTPError is an error carrying the HTTP status it should be rendered as.
// Handlers and services return it; the router's ErrorHandler renders it.
type HTTPError struct {
	Status  int
	Message string
	Code    string
	Err     error
	Fields  map[string]string
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%d %s: %v", e.Status, e.Message, e.Err)
	}
	return fmt.Sprintf("%d %s", e.Status, e.Message)
}

func (e *HTTPError) Unwrap() error { return e.Err }

// WithCode returns a copy carrying a machine-readable code.
func (e *HTTPError) WithCode(code string) *HTTPError {
	c := *e
	c.Code = code
	return &c
}

// WithFields returns a copy carrying per-field validation detail.
func (e *HTTPError) WithFields(fields map[string]string) *HTTPError {
	c := *e
	c.Fields = fields
	return &c
}

// Wrap returns a copy carrying cause, which is logged but never serialized.
func (e *HTTPError) Wrap(cause error) *HTTPError {
	c := *e
	c.Err = cause
	return &c
}

// NewError builds an HTTPError for an arbitrary status.
func NewError(status int, message string) *HTTPError {
	return &HTTPError{Status: status, Message: message, Code: defaultCode(status)}
}

// Errorf builds an HTTPError with a formatted message.
func Errorf(status int, format string, a ...any) *HTTPError {
	return &HTTPError{Status: status, Message: fmt.Sprintf(format, a...), Code: defaultCode(status)}
}

// Sentinels are typed as error so that callers cannot mutate the shared value
// by accident; use the constructors below to build customized instances.
var (
	ErrBadRequest       error = NewError(http.StatusBadRequest, "bad request")
	ErrUnauthorized     error = NewError(http.StatusUnauthorized, "unauthorized")
	ErrForbidden        error = NewError(http.StatusForbidden, "forbidden")
	ErrNotFound         error = NewError(http.StatusNotFound, "not found")
	ErrMethodNotAllowed error = NewError(http.StatusMethodNotAllowed, "method not allowed")
	ErrConflict         error = NewError(http.StatusConflict, "conflict")
	ErrRequestTooLarge  error = NewError(http.StatusRequestEntityTooLarge, "request body too large")
	ErrUnprocessable    error = NewError(http.StatusUnprocessableEntity, "unprocessable entity")
	ErrTooManyRequests  error = NewError(http.StatusTooManyRequests, "too many requests")
	ErrInternal         error = NewError(http.StatusInternalServerError, "internal server error")
	ErrTimeout          error = NewError(http.StatusGatewayTimeout, "request timeout")
)

func BadRequestf(format string, a ...any) *HTTPError {
	return Errorf(http.StatusBadRequest, format, a...)
}

func Unauthorizedf(format string, a ...any) *HTTPError {
	return Errorf(http.StatusUnauthorized, format, a...)
}

func Forbiddenf(format string, a ...any) *HTTPError {
	return Errorf(http.StatusForbidden, format, a...)
}

func NotFoundf(format string, a ...any) *HTTPError {
	return Errorf(http.StatusNotFound, format, a...)
}

func Conflictf(format string, a ...any) *HTTPError {
	return Errorf(http.StatusConflict, format, a...)
}

func Unprocessablef(format string, a ...any) *HTTPError {
	return Errorf(http.StatusUnprocessableEntity, format, a...)
}

func Internalf(format string, a ...any) *HTTPError {
	return Errorf(http.StatusInternalServerError, format, a...)
}

// ErrorMapper converts a domain error into an HTTPError, or returns nil if it
// does not recognize the error. It is the seam that lets bootstrap/ map ORM
// and driver errors to statuses without pkg/router importing pkg/orm.
type ErrorMapper func(error) *HTTPError

var (
	mappersMu sync.RWMutex
	mappers   []ErrorMapper
)

// RegisterErrorMapper adds a process-wide mapper consulted by every Router.
func RegisterErrorMapper(m ErrorMapper) {
	mappersMu.Lock()
	defer mappersMu.Unlock()
	mappers = append(mappers, m)
}

func globalMappers() []ErrorMapper {
	mappersMu.RLock()
	defer mappersMu.RUnlock()
	out := make([]ErrorMapper, len(mappers))
	copy(out, mappers)
	return out
}

func defaultCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "request_too_large"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusGatewayTimeout:
		return "timeout"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "error"
	}
}
