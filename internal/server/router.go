package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	// Blank import: each file in app/Routes registers its routes from init().
	_ "github.com/gochin/framework/app/Routes"

	"github.com/gochin/framework/bootstrap"
	"github.com/gochin/framework/pkg/config"
	"github.com/gochin/framework/pkg/router"
	mw "github.com/gochin/framework/pkg/router/middleware"
)

// BuildRouter assembles the application router: global middleware, framework
// routes, then every route file registered from app/Routes.
func BuildRouter(cfg *config.AppConfig) (*router.Router, error) {
	// Logging first: everything below logs through slog, which this installs.
	if err := bootstrap.ConfigureLogging(cfg); err != nil {
		return nil, err
	}
	bootstrap.RegisterErrorMappers()
	bootstrap.ConfigureAuth(cfg)
	bootstrap.ConfigureMail(cfg)

	r := router.New(
		router.WithPrefix(cfg.Server.APIPrefix),
		router.WithMaxBodyBytes(cfg.Server.MaxBodyBytes),
		router.WithMaxUploadBytes(cfg.Storage.MaxUploadBytes),
		router.WithDebug(cfg.App.Debug),
		router.WithTrustProxy(cfg.Server.TrustProxy),
	)

	r.Use(
		mw.RequestID(cfg.Server.TrustProxy),
		mw.Logger(mw.LoggerConfig{
			Logger:    slog.Default(),
			SkipPaths: []string{"/health"},
		}),
		mw.Recover(cfg.App.Debug),
		mw.CORS(mw.CORSConfig{AllowedOrigins: corsOrigins(cfg)}),
		mw.RateLimit(mw.RateLimitConfig{RequestsPerMinute: cfg.Server.RateLimitPerMinute}),
		mw.Timeout(cfg.Server.HandlerTimeout),
	)

	// Infrastructure endpoints stay off the API prefix so load balancers are
	// not pointed at /api/v1/health.
	root := r.Root()
	root.Get("/{$}", handleHome)
	root.Get("/health", handleHealth)

	// Public assets only. Uploads live under cfg.Storage.Root and are
	// deliberately not served, so an uploaded file can never be fetched back
	// from this origin.
	if cfg.Storage.PublicDir != "" && cfg.Storage.PublicURL != "" {
		r.Static(cfg.Storage.PublicURL, cfg.Storage.PublicDir)
	}

	router.Apply(r)

	if err := r.Build(); err != nil {
		return nil, err
	}
	return r, nil
}

func corsOrigins(cfg *config.AppConfig) []string {
	raw := strings.TrimSpace(cfg.Server.CORSAllowedOrigins)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func handleHome(c *router.Context) error {
	body, err := GetHTMLContent("index.html")
	if err != nil {
		return router.Internalf("template not found").Wrap(err)
	}
	return c.HTML(http.StatusOK, body)
}

func handleHealth(c *router.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
