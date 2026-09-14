package router

import (
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// RouteInfo describes one registered route, for `gochin route list`.
type RouteInfo struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Pattern     string   `json:"pattern"`
	Handler     string   `json:"handler"`
	HandlerFile string   `json:"handler_file,omitempty"`
	Middleware  []string `json:"middleware,omitempty"`
	Group       string   `json:"group,omitempty"`
	Synthetic   bool     `json:"synthetic,omitempty"`
}

func newRouteInfo(method, path, group string, h Handler, global, local []Middleware) RouteInfo {
	name, file := funcInfo(h)
	return RouteInfo{
		Method:      method,
		Path:        path,
		Pattern:     method + " " + path,
		Handler:     prettyFuncName(name),
		HandlerFile: file,
		Middleware:  middlewareNames(global, local),
		Group:       group,
	}
}

func syntheticRoute(method, path, kind string) RouteInfo {
	return RouteInfo{
		Method:    method,
		Path:      path,
		Pattern:   method + " " + path,
		Handler:   "router." + kind,
		Synthetic: true,
	}
}

// Routes returns the registered routes sorted by path then method. Synthetic
// 405/OPTIONS entries are excluded unless includeSynthetic is set.
func (rt *Router) Routes(includeSynthetic bool) []RouteInfo {
	out := make([]RouteInfo, 0, len(rt.routes))
	for _, r := range rt.routes {
		if r.Synthetic && !includeSynthetic {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

func middlewareNames(global, local []Middleware) []string {
	if len(global)+len(local) == 0 {
		return nil
	}
	names := make([]string, 0, len(global)+len(local))
	for _, m := range global {
		n, _ := funcInfo(m)
		names = append(names, prettyFuncName(n))
	}
	for _, m := range local {
		n, _ := funcInfo(m)
		names = append(names, prettyFuncName(n))
	}
	return names
}

func funcInfo(v any) (name, file string) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Func || rv.IsNil() {
		return "unknown", ""
	}
	pc := rv.Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown", ""
	}
	f, line := fn.FileLine(pc)
	return fn.Name(), f + ":" + strconv.Itoa(line)
}

// prettyFuncName renders a runtime symbol as something readable:
//
//	…/app/Controllers.(*UserController).Show-fm  -> UserController.Show
//	…/internal/server.BuildRouter.Recover.func7  -> Recover
//
// The "-fm" suffix marks a method value and the trailing ".funcN" segments
// are the closure a middleware constructor returned; neither helps a reader.
func prettyFuncName(name string) string {
	if name == "" {
		return "unknown"
	}

	name = strings.TrimSuffix(name, "-fm")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}

	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		parts = parts[1:] // drop the package qualifier
	}
	for len(parts) > 1 && isClosureSegment(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return "unknown"
	}

	last := parts[len(parts)-1]
	if len(parts) >= 2 && strings.HasPrefix(parts[len(parts)-2], "(*") {
		recv := strings.Trim(parts[len(parts)-2], "(*)")
		return recv + "." + last
	}
	return last
}

func isClosureSegment(s string) bool {
	if s == "glob" {
		return true
	}
	rest, ok := strings.CutPrefix(s, "func")
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
