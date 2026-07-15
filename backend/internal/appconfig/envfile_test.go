package appconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseEnvFileTreatsShellSyntaxAsData(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "must-not-exist")
	path := filepath.Join(root, "palpanel.env")
	body := "PALPANEL_LOG_LEVEL='debug'\n" +
		"LITERAL=$(touch " + marker + ")\n" +
		"URL=\"http://127.0.0.1:8090/path\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := ParseEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if values["LITERAL"] != "$(touch "+marker+")" || values["URL"] != "http://127.0.0.1:8090/path" {
		t.Fatalf("unexpected parsed values: %#v", values)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("shell content was executed: %v", err)
	}
}

func TestParseEnvFileRejectsShellAndAmbiguousSyntax(t *testing.T) {
	for _, body := range []string{
		"export PALPANEL_LOG_LEVEL=debug\n",
		"PALPANEL_LOG_LEVEL=debug\nPALPANEL_LOG_LEVEL=info\n",
		"NOT A NAME=value\n",
		"PALPANEL_LOG_LEVEL='unterminated\n",
	} {
		path := filepath.Join(t.TempDir(), "palpanel.env")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ParseEnvFile(path); err == nil {
			t.Fatalf("expected parser to reject %q", body)
		}
	}
}

func TestLoadFileGivesProcessEnvironmentPriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "palpanel.env")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		"PALPANEL_REQUIRE_AUTH=true",
		"PALPANEL_LISTEN_ADDR=127.0.0.1:9000",
	}, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PALPANEL_LISTEN_ADDR", "127.0.0.1:9100")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:9100" || !cfg.RequireAuth {
		t.Fatalf("process environment did not win: %#v", cfg)
	}
}

func TestLoadFileRejectsUnsafeEnvironmentNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "palpanel.env")
	if err := os.WriteFile(path, []byte("LD_PRELOAD=/tmp/untrusted.so\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil || !strings.Contains(err.Error(), "LD_PRELOAD") {
		t.Fatalf("expected LD_PRELOAD to be rejected, got %v", err)
	}
}

func TestInitFileCreatesPrivateRegistrationConfigOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "palpanel.env")
	created, err := InitFile(path)
	if err != nil || !created {
		t.Fatalf("InitFile = %v, %v", created, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o", info.Mode().Perm())
	}
	body, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(body), "PALPANEL_REQUIRE_AUTH=true") {
		t.Fatalf("config does not contain browser-registration settings: %v, %s", err, body)
	}
	values, err := ParseEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(values["PALWORLD_ADMIN_PASSWORD"]) < 40 {
		t.Fatal("production configuration did not generate a strong Palworld administrator password")
	}
	secondCreated, err := InitFile(path)
	if err != nil || secondCreated {
		t.Fatalf("second InitFile = %v, %v", secondCreated, err)
	}
	secondBody, _ := os.ReadFile(path)
	if string(secondBody) != string(body) {
		t.Fatal("existing configuration was changed")
	}
}
