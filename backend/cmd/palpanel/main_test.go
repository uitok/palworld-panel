package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	panelauth "palpanel/internal/auth"
	"palpanel/internal/db"
)

const (
	cliOldSession = "old-session"
	cliOldAPIKey  = "ppk_old-development-key"
)

func TestRunAdminResetPasswordDefaultAndExplicitUser(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "explicit"}[explicit], func(t *testing.T) {
			configPath, databasePath := prepareAdminTest(t, true)
			args := []string{"reset-password", "--config", configPath}
			if explicit {
				args = append(args, "--username", "admin")
			}
			var output, prompts bytes.Buffer
			if err := runAdminWithIO(args, strings.NewReader("new-password-123\nnew-password-123\n"), &output, &prompts); err != nil {
				t.Fatalf("runAdminWithIO returned error: %v", err)
			}
			if !strings.Contains(output.String(), "password reset for admin") || !strings.Contains(prompts.String(), "New password for admin") {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", output.String(), prompts.String())
			}

			store, err := db.Open(databasePath)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			service := panelauth.New(store)
			if _, err := service.AuthenticateSession(context.Background(), cliOldSession); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("old session was not revoked: %v", err)
			}
			if _, err := service.AuthenticateAPIKey(context.Background(), cliOldAPIKey); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("old development key was not revoked: %v", err)
			}
			if _, _, err := service.Login(context.Background(), "admin", "new-password-123"); err != nil {
				t.Fatalf("new password did not authenticate: %v", err)
			}
		})
	}
}

func TestRunAdminResetPasswordRejectsMismatchedConfirmation(t *testing.T) {
	configPath, databasePath := prepareAdminTest(t, true)
	err := runAdminWithIO(
		[]string{"reset-password", "--config", configPath},
		strings.NewReader("new-password-123\ndifferent-password\n"),
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	store, openErr := db.Open(databasePath)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer store.Close()
	if _, authErr := panelauth.New(store).AuthenticateSession(context.Background(), cliOldSession); authErr != nil {
		t.Fatalf("failed reset should preserve sessions: %v", authErr)
	}
}

func TestRunAdminResetPasswordRequiresAnAccount(t *testing.T) {
	configPath, _ := prepareAdminTest(t, false)
	err := runAdminWithIO(
		[]string{"reset-password", "--config", configPath},
		strings.NewReader("unused\nunused\n"),
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "no administrator account exists") {
		t.Fatalf("expected no-account error, got %v", err)
	}
}

func prepareAdminTest(t *testing.T, createUser bool) (string, string) {
	t.Helper()
	root := t.TempDir()
	databasePath := filepath.Join(root, "panel.db")
	configPath := filepath.Join(root, "palpanel.env")
	body := strings.Join([]string{
		"PALPANEL_DATA_DIR=" + root,
		"PALPANEL_DB_PATH=" + databasePath,
		"PALPANEL_REQUIRE_AUTH=true",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PALPANEL_DATA_DIR", root)
	t.Setenv("PALPANEL_DB_PATH", databasePath)
	t.Setenv("PALPANEL_REQUIRE_AUTH", "true")

	store, err := db.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if createUser {
		user := db.User{ID: "usr_admin", Username: "admin", PasswordHash: "unused", Role: "admin"}
		if err := store.CreateInitialUser(context.Background(), user); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateSession(context.Background(), db.Session{
			ID: "ses_old", UserID: user.ID, TokenHash: panelauth.TokenHash(cliOldSession),
			ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano),
		}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateAPIKey(context.Background(), db.APIKey{
			ID: "key_old", UserID: user.ID, Name: "old", Prefix: cliOldAPIKey[:12], TokenHash: panelauth.TokenHash(cliOldAPIKey),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return configPath, databasePath
}
