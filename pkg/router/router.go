package router

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// standardMethods are the verbs Build() synthesizes 405 responses for.
var standardMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut,
	http.MethodPatch, http.MethodDelete,
}

// ErrorHandler renders an error returned by a Handler.
type ErrorHandler func(*Context, error)

// Router maps HTTP requests to Handlers on top of the standard ServeMux.
type Router struct {
	mux        *http.ServeMux
	prefix     string
	mw         []Middleware
	routes     []RouteInfo
	registered map[string][]string // path -> methods, drives Build()

	errorHandler ErrorHandler
	mappers      []ErrorMapper
	maxBody      int64
	maxUpload    int64
	trustProxy   bool
	debug        bool
	logger       *slog.Logger

	buildOnce sync.Once
	built     bool
}

// Option configures a Router at construction.
type Option func(*Router)

// WithPrefix sets a prefix applied to every route registered through the
// Router or its Groups (Root() opts out).
func WithPrefix(prefix string) Option {
	return func(r *Router) { r.prefix = prefix }
}

// WithMaxBodyBytes caps request bodies decoded by Context.Bind.
func WithMaxBodyBytes(n int64) Option {
	return func(r *Router) { r.maxBody = n }
}

// WithMaxUploadBytes caps a single uploaded file read by Context.FormFile.
// Uploads need a far larger allowance than JSON bodies, hence a separate cap.
func WithMaxUploadBytes(n int64) Option {
	return func(r *Router) { r.maxUpload = n }
}

// WithDebug includes internal error detail in responses. Never enable it in
// production: it leaks driver and query text to clients.
func WithDebug(debug bool) Option {
	return func(r *Router) { r.debug = debug }
}

// WithTrustProxy makes Context.ClientIP honour X-Forwarded-For. Only enable
// it when the process sits behind a proxy that overwrites that header,
// otherwise clients can spoof their own address.
func WithTrustProxy(trust bool) Option {
	return func(r *Router) { r.trustProxy = trust }
}

// WithErrorHandler replaces the default error renderer.
func WithErrorHandler(h ErrorHandler) Option {
	return func(r *Router) { r.errorHandler = h }
}

// WithLogger sets the logger used for error reporting.
func WithLogger(l *slog.Logger) Option {
	return func(r *Router) { r.logger = l }
}

// New creates a Router.
func New(opts ...Option) *Router {
	r := &Router{
		mux:        http.NewServeMux(),
		registered: map[string][]string{},
		maxBody:    1 << 20,
		maxUpload:  10 << 20,
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.errorHandler == nil {
		r.errorHandler = r.defaultErrorHandler
	}
	return r
}

// Use appends global middleware, applied outside every group and route.
func (rt *Router) Use(mw ...Middleware) {
	if len(rt.routes) > 0 {
		panic("router: Use must be called before any route is registered")
	}
	rt.mw = append(rt.mw, mw...)
}

// MapError registers a router-local error mapper, consulted before the
// process-wide ones.
func (rt *Router) MapError(m ErrorMapper) { rt.mappers = append(rt.mappers, m) }

// Group returns a handle that prefixes paths and adds middleware.
func (rt *Router) Group(prefix string, mw ...Middleware) *Group {
	return &Group{
		router: rt,
		prefix: joinPath(rt.prefix, prefix),
		mw:     concatMiddleware(nil, mw),
	}
}

// Root returns a Group that ignores the router's configured prefix, for
// infrastructure endpoints like /health that must not sit under /api.
func (rt *Router) Root(mw ...Middleware) *Group {
	return &Group{
		router: rt,
		prefix: "",
		mw:     concatMiddleware(nil, mw),
	}
}

func (rt *Router) Get(path string, h Handler, mw ...Middleware) {
	rt.handle(http.MethodGet, joinPath(rt.prefix, path), h, mw, rt.prefix)
}

func (rt *Router) Post(path string, h Handler, mw ...Middleware) {
	rt.handle(http.MethodPost, joinPath(rt.prefix, path), h, mw, rt.prefix)
}

func (rt *Router) Put(path string, h Handler, mw ...Middleware) {
	rt.handle(http.MethodPut, joinPath(rt.prefix, path), h, mw, rt.prefix)
}

func (rt *Router) Patch(path string, h Handler, mw ...Middleware) {
	rt.handle(http.MethodPatch, joinPath(rt.prefix, path), h, mw, rt.prefix)
}

func (rt *Router) Delete(path string, h Handler, mw ...Middleware) {
	rt.handle(http.MethodDelete, joinPath(rt.prefix, path), h, mw, rt.prefix)
}

// Handle registers an arbitrary method, for verbs the helpers do not cover.
func (rt *Router) Handle(method, path string, h Handler, mw ...Middleware) {
	rt.handle(strings.ToUpper(method), joinPath(rt.prefix, path), h, mw, rt.prefix)
}

// handle is the single registration path for the Router and every Group.
func (rt *Router) handle(method, fullPath string, h Handler, mw []Middleware, group string) {
	if rt.built {
		panic("router: cannot register routes after Build")
	}
	if err := validatePath(fullPath); err != nil {
		panic(fmt.Sprintf("router: invalid route %s %s: %v (registered at %s)", method, fullPath, err, callerLocation()))
	}
	if !isKnownMethod(method) {
		panic(fmt.Sprintf("router: unknown HTTP method %q for %s", method, fullPath))
	}
	for _, existing := range rt.registered[fullPath] {
		if existing == method {
			panic(fmt.Sprintf("router: duplicate route %s %s (registered at %s)", method, fullPath, callerLocation()))
		}
	}

	// Record the bare handler's identity before middleware wraps it, so that
	// `route list` shows the controller method rather than Recover.func1.
	info := newRouteInfo(method, fullPath, group, h, rt.mw, mw)
	rt.routes = append(rt.routes, info)
	rt.registered[fullPath] = append(rt.registered[fullPath], method)

	composed := chain(h, concatMiddleware(rt.mw, mw))
	rt.mux.HandleFunc(method+" "+fullPath, rt.adapt(composed))
}

// Build finalizes routing: it synthesizes JSON 405 responses for the verbs
// each path does not implement, adds OPTIONS, and installs a JSON 404
// catch-all. It must be called once, after all routes are registered.
//
// The explicit 405 registration is required because a catch-all "/" pattern
// silently suppresses ServeMux's own 405 handling (a method-less pattern
// matches everything, so the not-found branch is never reached).
func (rt *Router) Build() error {
	var err error
	rt.buildOnce.Do(func() {
		for path, methods := range rt.registered {
			// Subtree patterns match everything beneath them; synthesizing
			// complements there would 405 every nested path.
			if strings.HasSuffix(path, "/") {
				continue
			}

			allow := allowHeader(methods)

			for _, m := range standardMethods {
				if containsString(methods, m) {
					continue
				}
				rt.mux.HandleFunc(m+" "+path, rt.adapt(methodNotAllowed(allow)))
				rt.routes = append(rt.routes, syntheticRoute(m, path, "405"))
			}

			if !containsString(methods, http.MethodOptions) {
				rt.mux.HandleFunc(http.MethodOptions+" "+path, rt.adapt(optionsHandler(allow)))
				rt.routes = append(rt.routes, syntheticRoute(http.MethodOptions, path, "options"))
			}
		}

		rt.mux.HandleFunc("/", rt.adapt(notFoundHandler))
		rt.built = true
	})
	return err
}

// ServeHTTP makes Router an http.Handler.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt.mux.ServeHTTP(w, r)
}

// adapt bridges a Handler to the stdlib signature, managing the pooled
// Context and funnelling every returned error through the ErrorHandler.
func (rt *Router) adapt(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := rt.acquire(w, r)
		defer c.release()

		if err := h(c); err != nil {
			rt.errorHandler(c, err)
			return
		}
		if !c.wroteHeader {
			c.WriteHeader(http.StatusOK)
		}
	}
}

type errorBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	Detail    string            `json:"detail,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

func (rt *Router) defaultErrorHandler(c *Context, err error) {
	he := rt.resolveError(err)

	attrs := []any{
		"method", c.Request.Method,
		"pattern", c.Request.Pattern,
		"status", he.Status,
	}
	if c.requestID != "" {
		attrs = append(attrs, "request_id", c.requestID)
	}
	if he.Status >= 500 {
		// 4xx are client mistakes; only 5xx should page an operator.
		rt.logger.Error(he.Message, append(attrs, "error", err)...)
	} else {
		rt.logger.Debug(he.Message, append(attrs, "error", err)...)
	}

	if c.Written() {
		return
	}

	body := errorBody{
		Code:      he.Code,
		Message:   he.Message,
		Fields:    he.Fields,
		RequestID: c.requestID,
	}
	if rt.debug && he.Err != nil {
		body.Detail = he.Err.Error()
	}
	_ = c.JSON(he.Status, errorEnvelope{Error: body})
}

// resolveError turns any error into an HTTPError, consulting router-local
// mappers before the process-wide ones.
func (rt *Router) resolveError(err error) *HTTPError {
	var he *HTTPError
	if errors.As(err, &he) {
		return he
	}
	for _, m := range rt.mappers {
		if mapped := m(err); mapped != nil {
			return mapped.Wrap(err)
		}
	}
	for _, m := range globalMappers() {
		if mapped := m(err); mapped != nil {
			return mapped.Wrap(err)
		}
	}
	return NewError(http.StatusInternalServerError, "internal server error").Wrap(err)
}

func notFoundHandler(c *Context) error {
	return NotFoundf("no route matches %s %s", c.Request.Method, c.Request.URL.Path)
}

func methodNotAllowed(allow string) Handler {
	return func(c *Context) error {
		c.Header().Set("Allow", allow)
		return NewError(http.StatusMethodNotAllowed,
			fmt.Sprintf("method %s is not allowed for this resource", c.Request.Method))
	}
}

func optionsHandler(allow string) Handler {
	return func(c *Context) error {
		c.Header().Set("Allow", allow)
		return c.NoContent(http.StatusNoContent)
	}
}

// allowHeader renders the Allow value for a path, including the HEAD and
// OPTIONS support the server provides implicitly.
func allowHeader(methods []string) string {
	set := map[string]bool{http.MethodOptions: true}
	for _, m := range methods {
		set[m] = true
		if m == http.MethodGet {
			set[http.MethodHead] = true // a GET pattern already serves HEAD
		}
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// Group prefixes a set of routes and applies shared middleware.
type Group struct {
	router    *Router
	prefix    string
	mw        []Middleware
	hasRoutes bool
}

// Group nests another Group, extending both prefix and middleware.
func (g *Group) Group(prefix string, mw ...Middleware) *Group {
	return &Group{
		router: g.router,
		prefix: joinPath(g.prefix, prefix),
		mw:     concatMiddleware(g.mw, mw),
	}
}

// Use adds middleware to the Group.
//
// It panics once the Group has registered routes: chains are composed at
// registration, so a late Use would silently skip earlier routes.
func (g *Group) Use(mw ...Middleware) {
	if g.hasRoutes {
		panic("router: Group.Use must be called before the group registers routes")
	}
	g.mw = concatMiddleware(g.mw, mw)
}

// Prefix returns the group's fully resolved path prefix.
func (g *Group) Prefix() string { return g.prefix }

func (g *Group) Get(path string, h Handler, mw ...Middleware) {
	g.handle(http.MethodGet, path, h, mw)
}

func (g *Group) Post(path string, h Handler, mw ...Middleware) {
	g.handle(http.MethodPost, path, h, mw)
}

func (g *Group) Put(path string, h Handler, mw ...Middleware) {
	g.handle(http.MethodPut, path, h, mw)
}

func (g *Group) Patch(path string, h Handler, mw ...Middleware) {
	g.handle(http.MethodPatch, path, h, mw)
}

func (g *Group) Delete(path string, h Handler, mw ...Middleware) {
	g.handle(http.MethodDelete, path, h, mw)
}

// Handle registers an arbitrary method within the group.
func (g *Group) Handle(method, path string, h Handler, mw ...Middleware) {
	g.handle(strings.ToUpper(method), path, h, mw)
}

func (g *Group) handle(method, path string, h Handler, mw []Middleware) {
	g.hasRoutes = true
	full := joinPath(g.prefix, path)
	g.router.handle(method, full, h, concatMiddleware(g.mw, mw), g.prefix)
}

// joinPath normalizes and joins path fragments into a single clean pattern.
//
// ServeMux panics at registration on any path it cannot clean (for instance
// the "/api/v1" + "/users" -> "/api/v1//users" that naive concatenation
// produces), so normalizing here is what keeps boot from crashing.
func joinPath(parts ...string) string {
	var segs []string
	for _, part := range parts {
		for _, seg := range strings.Split(part, "/") {
			if seg != "" {
				segs = append(segs, seg)
			}
		}
	}
	if len(segs) == 0 {
		return "/"
	}
	return "/" + strings.Join(segs, "/")
}

// validatePath rejects patterns ServeMux would panic on, reporting the
// offending route rather than an opaque stdlib panic.
func validatePath(p string) error {
	if !strings.HasPrefix(p, "/") {
		return errors.New("path must start with /")
	}

	seen := map[string]bool{}
	segs := strings.Split(strings.TrimPrefix(p, "/"), "/")

	for i, seg := range segs {
		if seg == "." || seg == ".." {
			return fmt.Errorf("path segment %q is not allowed", seg)
		}

		if !strings.ContainsAny(seg, "{}") {
			continue
		}
		if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
			return fmt.Errorf("wildcard must occupy a whole path segment, got %q", seg)
		}

		name := seg[1 : len(seg)-1]
		if name == "$" {
			if i != len(segs)-1 {
				return errors.New(`{$} is only valid as the final segment`)
			}
			continue
		}

		multi := strings.HasSuffix(name, "...")
		name = strings.TrimSuffix(name, "...")
		if multi && i != len(segs)-1 {
			return errors.New("{name...} is only valid as the final segment")
		}
		if name == "" {
			return errors.New("wildcard must be named")
		}
		if strings.ContainsAny(name, "{}/") {
			return fmt.Errorf("invalid wildcard name %q", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate wildcard name %q", name)
		}
		seen[name] = true
	}

	return nil
}

func isKnownMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodOptions, http.MethodHead:
		return true
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// callerLocation reports the application file that registered a route,
// skipping frames inside this package.
func callerLocation() string {
	for i := 2; i < 12; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		if strings.Contains(file, "/pkg/router/") {
			continue
		}
		return fmt.Sprintf("%s:%d", file, line)
	}
	return "unknown"
}
