package bootstrap

import (
	"github.com/gochin/framework/pkg/auth"
	"github.com/gochin/framework/pkg/config"
)

// ConfigureAuth applies the configured argon2id parameters.
//
// Call it once at startup, before any password is hashed or verified.
func ConfigureAuth(cfg *config.AppConfig) {
	auth.Configure(auth.Params{
		Memory:        uint32(cfg.Auth.Argon2MemoryKiB),
		Time:          uint32(cfg.Auth.Argon2Time),
		Parallelism:   uint8(cfg.Auth.Argon2Parallelism),
		MaxConcurrent: cfg.Auth.Argon2MaxConcurrent,
	})
}
