package bootstrap

import (
	"fmt"

	"github.com/sachinkaru123/gochin/pkg/config"
	"github.com/sachinkaru123/gochin/pkg/logs"
)

// ConfigureLogging installs the application logger.
//
// It also becomes slog's default, so the router middleware, the auth guard
// and the HTTP server's error log all write to the same files without any
// of those packages knowing about pkg/logs.
func ConfigureLogging(cfg *config.AppConfig) error {
	err := logs.Init(logs.Config{
		Dir:      cfg.Log.Dir,
		Level:    cfg.Log.Level,
		Format:   cfg.Log.Format,
		ToFile:   cfg.Log.ToFile,
		ToStdout: cfg.Log.ToStdout,
	})
	if err != nil {
		return fmt.Errorf("configuring logging: %w", err)
	}
	return nil
}
