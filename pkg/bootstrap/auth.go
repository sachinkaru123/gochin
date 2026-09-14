package bootstrap

import (
	"github.com/sachinkaru123/gochin/pkg/auth"
	"github.com/sachinkaru123/gochin/pkg/config"
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
