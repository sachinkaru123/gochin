// Package mail sends outbound email behind a driver interface.
//
// The default driver is "log", which renders a message into the log instead
// of delivering it: a misconfigured development machine should never be able
// to email real people.
package mail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net/mail"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNoRecipients is returned for a message with nobody to send to.
var ErrNoRecipients = errors.New("mail: message has no recipients")

// Attachment is a file sent with a message.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// Message is one outbound email.
type Message struct {
	From        string
	FromName    string
	To          []string
	Cc          []string
	Bcc         []string
	ReplyTo     string
	Subject     string
	Text        string
	HTML        string
	Attachments []Attachment
}

// Recipients returns every address the message is delivered to.
func (m Message) Recipients() []string {
	out := make([]string, 0, len(m.To)+len(m.Cc)+len(m.Bcc))
	out = append(out, m.To...)
	out = append(out, m.Cc...)
	out = append(out, m.Bcc...)
	return out
}

// Validate checks a message is deliverable.
func (m Message) Validate() error {
	if len(m.Recipients()) == 0 {
		return ErrNoRecipients
	}
	for _, addr := range m.Recipients() {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("mail: invalid recipient %q: %w", addr, err)
		}
	}
	if m.From != "" {
		if _, err := mail.ParseAddress(m.From); err != nil {
			return fmt.Errorf("mail: invalid sender %q: %w", m.From, err)
		}
	}
	if strings.TrimSpace(m.Subject) == "" {
		return errors.New("mail: message has no subject")
	}
	if m.Text == "" && m.HTML == "" {
		return errors.New("mail: message has no body")
	}
	return nil
}

// Mailer delivers messages.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

var (
	mu      sync.RWMutex
	current Mailer = NewLogMailer()
	fromDef string
	nameDef string
)

// SetDefault installs the process-wide mailer.
func SetDefault(m Mailer, fromAddress, fromName string) {
	mu.Lock()
	defer mu.Unlock()
	current = m
	fromDef = fromAddress
	nameDef = fromName
}

// Default returns the process-wide mailer.
func Default() Mailer {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Send delivers a message with the default mailer, filling in the configured
// sender when the message does not set one.
func Send(ctx context.Context, m Message) error {
	mu.RLock()
	mailer, from, name := current, fromDef, nameDef
	mu.RUnlock()

	if m.From == "" {
		m.From = from
	}
	if m.FromName == "" {
		m.FromName = name
	}

	if err := m.Validate(); err != nil {
		return err
	}
	return mailer.Send(ctx, m)
}

// Build renders a message into RFC 5322 bytes.
func Build(m Message) ([]byte, error) {
	var buf bytes.Buffer

	writeHeader(&buf, "From", formatAddress(m.From, m.FromName))
	writeHeader(&buf, "To", strings.Join(m.To, ", "))
	if len(m.Cc) > 0 {
		writeHeader(&buf, "Cc", strings.Join(m.Cc, ", "))
	}
	if m.ReplyTo != "" {
		writeHeader(&buf, "Reply-To", m.ReplyTo)
	}
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	writeHeader(&buf, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&buf, "MIME-Version", "1.0")

	boundary := "gochin-boundary-f7a2c1d9"

	switch {
	case len(m.Attachments) > 0 || (m.Text != "" && m.HTML != ""):
		writeHeader(&buf, "Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
		buf.WriteString("\r\n")

		if m.Text != "" {
			writePart(&buf, boundary, "text/plain; charset=utf-8", []byte(m.Text))
		}
		if m.HTML != "" {
			writePart(&buf, boundary, "text/html; charset=utf-8", []byte(m.HTML))
		}
		for _, a := range m.Attachments {
			writeAttachment(&buf, boundary, a)
		}
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	case m.HTML != "":
		writeHeader(&buf, "Content-Type", "text/html; charset=utf-8")
		buf.WriteString("\r\n")
		buf.WriteString(m.HTML)

	default:
		writeHeader(&buf, "Content-Type", "text/plain; charset=utf-8")
		buf.WriteString("\r\n")
		buf.WriteString(m.Text)
	}

	return buf.Bytes(), nil
}

func writeHeader(buf *bytes.Buffer, name, value string) {
	// Strip CR/LF so a crafted subject or display name cannot inject extra
	// headers or a second recipient.
	value = strings.NewReplacer("\r", "", "\n", "").Replace(value)
	fmt.Fprintf(buf, "%s: %s\r\n", name, value)
}

func writePart(buf *bytes.Buffer, boundary, contentType string, body []byte) {
	fmt.Fprintf(buf, "--%s\r\n", boundary)
	fmt.Fprintf(buf, "Content-Type: %s\r\n\r\n", contentType)
	buf.Write(body)
	buf.WriteString("\r\n")
}

func writeAttachment(buf *bytes.Buffer, boundary string, a Attachment) {
	ct := a.ContentType
	if ct == "" {
		ct = mime.TypeByExtension(filepath.Ext(a.Filename))
		if ct == "" {
			ct = "application/octet-stream"
		}
	}

	fmt.Fprintf(buf, "--%s\r\n", boundary)
	fmt.Fprintf(buf, "Content-Type: %s\r\n", ct)
	fmt.Fprintf(buf, "Content-Disposition: attachment; filename=%q\r\n", filepath.Base(a.Filename))
	buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	buf.WriteString(base64Wrap(a.Content))
	buf.WriteString("\r\n")
}

func formatAddress(address, name string) string {
	if name == "" {
		return address
	}
	return (&mail.Address{Name: name, Address: address}).String()
}

// Render renders an html/template file with data.
//
// html/template, not text/template: it escapes interpolated values, so user
// data in an email body cannot inject markup.
func Render(templatePath string, data any) (string, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("mail: parsing %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("mail: rendering %s: %w", templatePath, err)
	}
	return buf.String(), nil
}
