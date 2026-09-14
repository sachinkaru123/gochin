package router

// Handler serves one request. Returning an error hands control to the
// router's ErrorHandler, so handlers never have to render failures themselves.
type Handler func(*Context) error

// Middleware wraps a Handler. Returning without calling next short-circuits
// the request.
type Middleware func(Handler) Handler

// chain composes mw around h so that mw[0] ends up outermost.
//
// This runs once per route at registration time, never per request: the
// runtime cost of a request is N nested closure calls, not N allocations.
func chain(h Handler, mw []Middleware) Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// concatMiddleware joins two middleware slices into a fresh backing array.
//
// Copying is required: appending into a parent group's slice would let two
// sibling groups overwrite each other's middleware.
func concatMiddleware(a, b []Middleware) []Middleware {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := make([]Middleware, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}
