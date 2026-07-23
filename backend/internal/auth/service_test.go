package auth

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"palpanel/internal/db"
)

func TestConcurrentInitialRegistrationCreatesOneAdministrator(t *testing.T) {
	store := openAuthTestStore(t)
	service := New(store)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, username := range []string{"admin-one", "admin-two"} {
		wait.Add(1)
		go func(username string) {
			defer wait.Done()
			<-start
			_, _, err := service.Register(context.Background(), username, "strong-password-123")
			results <- err
		}(username)
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, db.ErrAlreadyInitialized):
			conflicts++
		default:
			t.Fatalf("unexpected registration result: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("registration results: successes=%d conflicts=%d", successes, conflicts)
	}
	count, err := store.UserCount(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("user count = %d, %v", count, err)
	}
}

func TestCredentialsExpireRevokeAndResetTogether(t *testing.T) {
	store := openAuthTestStore(t)
	service := New(store)
	baseTime := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return baseTime }
	user, session, err := service.Register(context.Background(), "admin", "strong-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if identity, err := service.AuthenticateSession(context.Background(), session); err != nil || identity.Username != "admin" || identity.Credential != CredentialSession {
		t.Fatalf("session authentication = %#v, %v", identity, err)
	}
	if _, _, err := service.Login(context.Background(), "admin", "wrong-password"); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("invalid login error = %v", err)
	}
	loginUser, secondSession, err := service.Login(context.Background(), "admin", "strong-password-123")
	if err != nil || loginUser.ID != user.ID {
		t.Fatalf("login = %#v, %v", loginUser, err)
	}

	createdKey, err := service.CreateAPIKey(context.Background(), user.ID, "automation")
	if err != nil || createdKey.Token == "" || createdKey.Token[:4] != "ppk_" {
		t.Fatalf("CreateAPIKey = %#v, %v", createdKey, err)
	}
	if identity, err := service.AuthenticateAPIKey(context.Background(), createdKey.Token); err != nil || identity.Credential != CredentialAPIKey {
		t.Fatalf("key authentication = %#v, %v", identity, err)
	}
	if err := service.RevokeAPIKey(context.Background(), user.ID, createdKey.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateAPIKey(context.Background(), createdKey.Token); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("revoked key authentication error = %v", err)
	}

	resetKey, err := service.CreateAPIKey(context.Background(), user.ID, "before-reset")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPassword(context.Background(), "admin", "replacement-password-123"); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{session, secondSession} {
		if _, err := service.AuthenticateSession(context.Background(), token); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("reset session authentication error = %v", err)
		}
	}
	if _, err := service.AuthenticateAPIKey(context.Background(), resetKey.Token); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("reset key authentication error = %v", err)
	}
	if _, _, err := service.Login(context.Background(), "admin", "replacement-password-123"); err != nil {
		t.Fatalf("replacement password login failed: %v", err)
	}

	expiringToken, err := service.newSession(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return baseTime.Add(SessionLifetime + time.Second) }
	if _, err := service.AuthenticateSession(context.Background(), expiringToken); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired session authentication error = %v", err)
	}
}

func TestChangePasswordVerifiesCurrentPasswordAndRevokesCredentials(t *testing.T) {
	store := openAuthTestStore(t)
	service := New(store)
	user, session, err := service.Register(context.Background(), "admin", "strong-password-123")
	if err != nil {
		t.Fatal(err)
	}
	key, err := service.CreateAPIKey(context.Background(), user.ID, "before-password-change")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.ChangePassword(context.Background(), "admin", "wrong-password", "replacement-password-123"); !errors.Is(err, ErrInvalidCurrentPassword) {
		t.Fatalf("wrong current password error = %v", err)
	}
	if _, err := service.AuthenticateSession(context.Background(), session); err != nil {
		t.Fatalf("wrong current password revoked session: %v", err)
	}
	if _, err := service.AuthenticateAPIKey(context.Background(), key.Token); err != nil {
		t.Fatalf("wrong current password revoked development key: %v", err)
	}

	if err := service.ChangePassword(context.Background(), "admin", "strong-password-123", "replacement-password-123"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateSession(context.Background(), session); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("changed-password session authentication error = %v", err)
	}
	if _, err := service.AuthenticateAPIKey(context.Background(), key.Token); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("changed-password key authentication error = %v", err)
	}
	if _, _, err := service.Login(context.Background(), "admin", "strong-password-123"); !errors.Is(err, ErrInvalidLogin) {
		t.Fatalf("old password login error = %v", err)
	}
	if _, _, err := service.Login(context.Background(), "admin", "replacement-password-123"); err != nil {
		t.Fatalf("new password login failed: %v", err)
	}
}

func TestCredentialValidation(t *testing.T) {
	if ValidateUsername("ab") == nil || ValidateUsername("contains space") == nil || ValidateUsername("valid_admin") != nil {
		t.Fatal("unexpected username validation")
	}
	if _, err := HashPassword("short"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("short password error = %v", err)
	}
	store := openAuthTestStore(t)
	if _, err := New(store).CreateAPIKey(context.Background(), "missing", ""); !errors.Is(err, ErrInvalidKeyName) {
		t.Fatalf("invalid key name error = %v", err)
	}
}

func openAuthTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
