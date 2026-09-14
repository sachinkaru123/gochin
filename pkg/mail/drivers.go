package mail

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/gochin/framework/pkg/logs"
)

// LogMailer writes messages to the log instead of sending them.
//
// This is the default driver, so an unconfigured application cannot
// accidentally deliver mail to real addresses.
type LogMailer struct {
	// MaxBodyChars truncates the logged body; a full HTML email is noise in
	// a log file and may contain personal data.
	MaxBodyChars int
}

func NewLogMailer() *LogMailer { return &LogMailer{MaxBodyChars: 2000} }

func (m *LogMailer) Send(ctx context.Context, msg Message) error {
	body := msg.Text
	if body == "" {
		body = msg.HTML
	}
	if m.MaxBodyChars > 0 && len(body) > m.MaxBodyChars {
		body = body[:m.MaxBodyChars] + "… (truncated)"
	}

	logs.Channel("mail").Info("mail (not sent: log driver)",
		"from", msg.From,
		"to", strings.Join(msg.To, ", "),
		"cc", strings.Join(msg.Cc, ", "),
		"subject", msg.Subject,
		"attachments", len(msg.Attachments),
		"body", body,
	)
	return nil
}

// ArrayMailer keeps messages in memory for assertions in tests.
type ArrayMailer struct {
	mu       sync.Mutex
	messages []Message
}

func NewArrayMailer() *ArrayMailer { return &ArrayMailer{} }

func (m *ArrayMailer) Send(ctx context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

// Messages returns everything captured so far.
func (m *ArrayMailer) Messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Message, len(m.messages))
	copy(out, m.messages)
	return out
}

// Reset discards captured messages.
func (m *ArrayMailer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

// Count returns how many messages were captured.
func (m *ArrayMailer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

// SMTPMailer delivers over SMTP using the standard library.
type SMTPMailer struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool
	Timeout  time.Duration
}

func NewSMTPMailer(host string, port int, username, password string, useTLS bool) *SMTPMailer {
	return &SMTPMailer{
		Host: host, Port: port,
		Username: username, Password: password,
		UseTLS: useTLS, Timeout: 10 * time.Second,
	}
}

func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	raw, err := Build(msg)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(m.Host, fmt.Sprint(m.Port))
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("mail: dialing %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mail: smtp handshake: %w", err)
	}
	defer client.Close()

	if m.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			// ServerName must be set or the certificate is not verified
			// against the host being talked to.
			if err := client.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
				return fmt.Errorf("mail: starttls: %w", err)
			}
		}
	}

	if m.Username != "" {
		auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
		if err := client.Auth(auth); err != nil {
			// Never include the credential in the error.
			return fmt.Errorf("mail: authentication failed for %q", m.Username)
		}
	}

	if err := client.Mail(msg.From); err != nil {
		return fmt.Errorf("mail: sender rejected: %w", err)
	}
	for _, to := range msg.Recipients() {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("mail: recipient %q rejected: %w", to, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: starting data: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return fmt.Errorf("mail: writing body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: finishing body: %w", err)
	}

	return client.Quit()
}

// base64Wrap encodes content into 76-character lines, as MIME requires.
func base64Wrap(content []byte) string {
	encoded := base64.StdEncoding.EncodeToString(content)

	var b strings.Builder
	for len(encoded) > 76 {
		b.WriteString(encoded[:76])
		b.WriteString("\r\n")
		encoded = encoded[76:]
	}
	b.WriteString(encoded)
	return b.String()
}
