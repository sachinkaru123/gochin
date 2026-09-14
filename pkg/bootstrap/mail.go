package bootstrap

import (
	"strings"

	"github.com/gochin/framework/pkg/config"
	"github.com/gochin/framework/pkg/logs"
	"github.com/gochin/framework/pkg/mail"
)

// ConfigureMail installs the configured mailer.
//
// An unrecognized driver falls back to the log driver rather than failing
// open into real delivery.
func ConfigureMail(cfg *config.AppConfig) {
	var mailer mail.Mailer

	switch strings.ToLower(strings.TrimSpace(cfg.Mail.Driver)) {
	case "smtp":
		mailer = mail.NewSMTPMailer(
			cfg.Mail.Host, cfg.Mail.Port,
			cfg.Mail.Username, cfg.Mail.Password,
			cfg.Mail.TLS,
		)
	case "array":
		mailer = mail.NewArrayMailer()
	case "log", "":
		mailer = mail.NewLogMailer()
	default:
		logs.Warn("unknown mail driver, falling back to log", "driver", cfg.Mail.Driver)
		mailer = mail.NewLogMailer()
	}

	mail.SetDefault(mailer, cfg.Mail.FromAddress, cfg.Mail.FromName)
}
