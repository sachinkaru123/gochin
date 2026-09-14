package router

import "sync"

// RouteRegistrar registers a set of routes on the application router.
type RouteRegistrar func(*Router)

var (
	registrarMu sync.Mutex
	registrars  []RouteRegistrar
)

// Register queues fn to run when the application router is built. Route files
// in app/Routes call this from their init().
//
// Registering a function rather than data is deliberate: it defers building
// controllers and services until boot, so package initialization never opens a
// database connection and `route list` can run without one.
func Register(fn RouteRegistrar) {
	registrarMu.Lock()
	defer registrarMu.Unlock()
	registrars = append(registrars, fn)
}

// Registered returns a copy of every queued registrar.
func Registered() []RouteRegistrar {
	registrarMu.Lock()
	defer registrarMu.Unlock()
	out := make([]RouteRegistrar, len(registrars))
	copy(out, registrars)
	return out
}

// Apply runs every queued registrar against rt.
func Apply(rt *Router) {
	for _, fn := range Registered() {
		fn(rt)
	}
}
