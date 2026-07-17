package debuglog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerCanBeEnabledAndDisabledAtRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "palpanel-debug.log")
	logger, err := New(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()
	logger.Printf("ignored")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("disabled logger created a file: %v", err)
	}
	if err := logger.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	logger.Printf("health probe endpoint=%s", "http://127.0.0.1:8212/v1/api/metrics")
	if _, err := logger.Write([]byte("standard log line\n")); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"debug logging enabled", "health probe endpoint=", "standard log line"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("debug log does not contain %q: %s", want, body)
		}
	}
	if status := logger.Status(); !status.Enabled || status.Path != path || status.Size == 0 {
		t.Fatalf("unexpected status: %#v", status)
	}
	if err := logger.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	logger.Printf("ignored after disable")
	body, _ = os.ReadFile(path)
	if strings.Contains(string(body), "ignored after disable") {
		t.Fatal("disabled logger continued writing")
	}
}
