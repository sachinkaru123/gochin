package logs

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Dir:      t.TempDir(),
		Level:    "debug",
		Format:   "text",
		ToFile:   true,
		ToStdout: false,
	}
}

func readLogDir(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var all strings.Builder
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		all.Write(b)
	}
	return all.String()
}

func TestInitWritesToFile(t *testing.T) {
	cfg := testConfig(t)
	if err := Init(cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(Close)

	Info("hello", "key", "value")
	Debug("debugging", "n", 1)

	out := readLogDir(t, cfg.Dir)
	if !strings.Contains(out, "hello") || !strings.Contains(out, "key=value") {
		t.Errorf("log file is missing the info line:\n%s", out)
	}
	if !strings.Contains(out, "debugging") {
		t.Errorf("log file is missing the debug line:\n%s", out)
	}
}

// Init must install the handler as slog's default, which is what routes the
// router, guard and server logs into these files without touching them.
func TestInitSetsSlogDefault(t *testing.T) {
	cfg := testConfig(t)
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)

	// Anything logging through plain slog must land in our file.
	slogDefaultWrite("via-slog-default")

	if out := readLogDir(t, cfg.Dir); !strings.Contains(out, "via-slog-default") {
		t.Errorf("a plain slog.Info call did not reach the log file:\n%s", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	cfg := testConfig(t)
	cfg.Level = "warn"
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)

	Debug("should-not-appear")
	Info("should-not-appear-either")
	Warn("should-appear")

	out := readLogDir(t, cfg.Dir)
	if strings.Contains(out, "should-not-appear") {
		t.Errorf("a message below the configured level was written:\n%s", out)
	}
	if !strings.Contains(out, "should-appear") {
		t.Errorf("the warn message is missing:\n%s", out)
	}
}

func TestChannelWritesToItsOwnFile(t *testing.T) {
	cfg := testConfig(t)
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)

	Info("default-line")
	Channel("payments").Info("payment-line")

	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Fatalf("got %d log files, want a separate file per channel", len(entries))
	}

	var paymentsFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "payments-") {
			paymentsFile = filepath.Join(cfg.Dir, e.Name())
		}
	}
	if paymentsFile == "" {
		t.Fatal("no payments-*.log file was created")
	}

	b, err := os.ReadFile(paymentsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "payment-line") {
		t.Error("the channel file is missing its line")
	}
	if strings.Contains(string(b), "default-line") {
		t.Error("the default logger's line leaked into the channel file")
	}
}

func TestChannelIsCached(t *testing.T) {
	if err := Init(testConfig(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)

	if Channel("a") != Channel("a") {
		t.Error("Channel returned a different logger for the same name")
	}
	if Channel("a") == Channel("b") {
		t.Error("Channel returned the same logger for different names")
	}
}

// The writer is hit from every request goroutine.
func TestConcurrentWrites(t *testing.T) {
	cfg := testConfig(t)
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			Info("concurrent", "n", n)
			Channel("worker").Info("channel-concurrent", "n", n)
		}(i)
	}
	wg.Wait()

	out := readLogDir(t, cfg.Dir)
	if got := strings.Count(out, "concurrent"); got < 100 {
		t.Errorf("wrote %d concurrent lines, want 100 (some were lost)", got)
	}
}

// The file must roll over when the date changes.
func TestDailyRotation(t *testing.T) {
	dir := t.TempDir()
	w, err := newDailyWriter(dir, "gochin")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	day := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	w.nowFunc = func() time.Time { return day }
	if _, err := w.Write([]byte("first day\n")); err != nil {
		t.Fatal(err)
	}

	day = day.Add(24 * time.Hour)
	if _, err := w.Write([]byte("second day\n")); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"gochin-2026-03-01.log", "gochin-2026-03-02.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}

	first, _ := os.ReadFile(filepath.Join(dir, "gochin-2026-03-01.log"))
	if strings.Contains(string(first), "second day") {
		t.Error("the second day's line was written to the first day's file")
	}
}

func TestDumpFormatsValues(t *testing.T) {
	cfg := testConfig(t)
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)

	type order struct {
		ID    int     `json:"id"`
		Total float64 `json:"total"`
	}

	Dump(order{ID: 7, Total: 12.5})
	Dump(map[string]int{"a": 1}, "counts")
	Dump(nil)
	Dump([]string{"x", "y"})

	out := readLogDir(t, cfg.Dir)
	for _, want := range []string{`\"id\": 7`, "counts", "logs_test.go:"} {
		if !strings.Contains(out, want) {
			t.Errorf("dump output is missing %q:\n%s", want, out)
		}
	}
}

func TestFormatHandlesAwkwardValues(t *testing.T) {
	// Channels do not marshal to JSON; Format must fall back, not panic.
	if got := Format(make(chan int)); got == "" {
		t.Error("Format returned empty for a channel")
	}
	if got := Format(nil); got != "nil" {
		t.Errorf("Format(nil) = %q, want \"nil\"", got)
	}
	if got := Format("plain"); got != "plain" {
		t.Errorf("Format(string) = %q", got)
	}
}

// slogDefaultWrite logs through the package-level slog API, standing in for
// the router/guard/server packages that already do exactly this.
func slogDefaultWrite(msg string) { slog.Info(msg) }
