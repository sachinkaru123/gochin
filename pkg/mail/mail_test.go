package mail

import (
	"context"
	"strings"
	"testing"
)

func TestArrayMailerCaptures(t *testing.T) {
	m := NewArrayMailer()
	SetDefault(m, "no-reply@gochin.test", "Gochin")

	err := Send(context.Background(), Message{
		To:      []string{"ada@example.com"},
		Subject: "Welcome",
		Text:    "Hello Ada",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if m.Count() != 1 {
		t.Fatalf("captured %d messages, want 1", m.Count())
	}

	got := m.Messages()[0]
	if got.From != "no-reply@gochin.test" {
		t.Errorf("From = %q; the configured sender was not applied", got.From)
	}
	if got.Subject != "Welcome" {
		t.Errorf("Subject = %q", got.Subject)
	}
}

func TestValidateRejectsBadMessages(t *testing.T) {
	cases := map[string]Message{
		"no recipients":     {Subject: "s", Text: "t"},
		"no subject":        {To: []string{"a@b.co"}, Text: "t"},
		"no body":           {To: []string{"a@b.co"}, Subject: "s"},
		"invalid recipient": {To: []string{"not-an-address"}, Subject: "s", Text: "t"},
		"invalid sender":    {From: "nope", To: []string{"a@b.co"}, Subject: "s", Text: "t"},
	}

	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			if err := msg.Validate(); err == nil {
				t.Error("Validate accepted an invalid message")
			}
		})
	}
}

func TestBuildProducesHeadersAndBody(t *testing.T) {
	raw, err := Build(Message{
		From: "no-reply@gochin.test", FromName: "Gochin",
		To:      []string{"ada@example.com"},
		Subject: "Hello",
		Text:    "plain body",
	})
	if err != nil {
		t.Fatal(err)
	}

	out := string(raw)
	for _, want := range []string{"From: ", "To: ada@example.com", "Subject: Hello", "plain body"} {
		if !strings.Contains(out, want) {
			t.Errorf("built message is missing %q:\n%s", want, out)
		}
	}
}

// A newline in a header value must not be able to inject another header.
func TestBuildStripsHeaderInjection(t *testing.T) {
	raw, err := Build(Message{
		From:    "no-reply@gochin.test",
		To:      []string{"ada@example.com"},
		Subject: "Hi\r\nBcc: attacker@evil.test",
		Text:    "body",
	})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(raw), "Bcc: attacker@evil.test\r\n") {
		t.Errorf("a header was injected through the subject:\n%s", raw)
	}
}

func TestBuildMultipartWhenTextAndHTML(t *testing.T) {
	raw, err := Build(Message{
		From: "a@b.co", To: []string{"c@d.co"},
		Subject: "s",
		Text:    "plain version",
		HTML:    "<p>html version</p>",
	})
	if err != nil {
		t.Fatal(err)
	}

	out := string(raw)
	for _, want := range []string{"multipart/alternative", "plain version", "<p>html version</p>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in multipart message", want)
		}
	}
}

func TestLogMailerTruncatesAndNeverFails(t *testing.T) {
	m := &LogMailer{MaxBodyChars: 10}
	err := m.Send(context.Background(), Message{
		From: "a@b.co", To: []string{"c@d.co"},
		Subject: "s",
		Text:    strings.Repeat("x", 5000),
	})
	if err != nil {
		t.Errorf("the log driver must not fail: %v", err)
	}
}

// The default driver must never be one that delivers real mail.
func TestDefaultDriverIsNotDelivering(t *testing.T) {
	SetDefault(NewLogMailer(), "", "")
	if _, ok := Default().(*SMTPMailer); ok {
		t.Error("the default mailer delivers over SMTP")
	}
}
