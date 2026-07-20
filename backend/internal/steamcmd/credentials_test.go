package steamcmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAuthenticatePersistsPasswordWithoutSteamGuardAndCleansScript(t *testing.T) {
	client, cfg := newTestClient(t)
	if err := os.Remove(cfg.SteamWorkshopCredentialsPath()); err != nil {
		t.Fatal(err)
	}
	password := `space quote" slash\fixture`
	guard := "123456"
	staleScript := filepath.Join(filepath.Dir(cfg.SteamWorkshopCredentialsPath()), "steamcmd-login-stale.txt")
	if err := os.WriteFile(staleScript, []byte("old secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	var scriptPath, scriptBody string
	client.runCommand = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		scriptPath = argumentAfter(t, args, "+runscript")
		body, err := os.ReadFile(scriptPath)
		if err != nil {
			t.Fatal(err)
		}
		scriptBody = string(body)
		return []byte("Waiting for user info...OK"), nil
	}

	status, err := client.Authenticate(t.Context(), LoginRequest{AccountName: "fixture_user", Password: password, SteamGuardCode: guard})
	if err != nil {
		t.Fatal(err)
	}
	if !status.LoggedIn || !status.PasswordConfigured || status.SteamGuardRequired || status.LoginInProgress {
		t.Fatalf("status = %#v", status)
	}
	if !strings.Contains(scriptBody, steamScriptArg(password)) || !strings.Contains(scriptBody, steamScriptArg(guard)) {
		t.Fatalf("credential script did not quote submitted values: %q", scriptBody)
	}
	if _, err := os.Stat(scriptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary credential script remains: %v", err)
	}
	if _, err := os.Stat(staleScript); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale credential script remains: %v", err)
	}
	body, err := os.ReadFile(cfg.SteamWorkshopCredentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), guard) {
		t.Fatalf("Steam Guard code was persisted: %s", body)
	}
	var stored storedCredentials
	if err := json.Unmarshal(body, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.AccountName != "fixture_user" || stored.Password != password || stored.LastVerifiedAt == "" {
		t.Fatalf("stored credentials = %#v", stored)
	}
}

func TestAuthenticateUsesManagedLinuxLoginRunnerAndPersistsCredentials(t *testing.T) {
	client, cfg := newTestClient(t)
	client.goos = "linux"
	if err := os.Remove(cfg.SteamWorkshopCredentialsPath()); err != nil {
		t.Fatal(err)
	}
	var received LoginRequest
	client.SetCredentialLoginRunner(func(_ context.Context, request LoginRequest) ([]byte, error) {
		received = request
		return []byte("Logging in using password and Steam Guard code...OK\nLogged in OK"), nil
	})
	status, err := client.Authenticate(t.Context(), LoginRequest{
		AccountName: "fixture_user", Password: "linux fixture password", SteamGuardCode: "654321",
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.Password != "linux fixture password" || received.SteamGuardCode != "654321" {
		t.Fatalf("managed login request = %#v", received)
	}
	if !status.Supported || !status.SteamCMDInstalled || !status.LoggedIn || !status.PasswordConfigured {
		t.Fatalf("managed Linux credential status = %#v", status)
	}
	body, err := os.ReadFile(cfg.SteamWorkshopCredentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "654321") || !strings.Contains(string(body), "linux fixture password") {
		t.Fatalf("persisted Linux credentials contain the wrong fields: %s", body)
	}
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(cfg.SteamWorkshopCredentialsPath())
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("Linux credential permissions = %v", info.Mode().Perm())
		}
	}
}

func TestAuthenticateSteamGuardFailureDoesNotPersistPassword(t *testing.T) {
	client, cfg := newTestClient(t)
	if err := os.Remove(cfg.SteamWorkshopCredentialsPath()); err != nil {
		t.Fatal(err)
	}
	client.runCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("Account logon denied, need two-factor code"), errors.New("exit status 5")
	}

	status, err := client.Authenticate(t.Context(), LoginRequest{AccountName: "fixture_user", Password: "wrong-or-unverified"})
	if !errors.Is(err, ErrSteamGuardRequired) || !status.SteamGuardRequired || status.PasswordConfigured {
		t.Fatalf("Authenticate = %#v, %v", status, err)
	}
	if _, err := os.Stat(cfg.SteamWorkshopCredentialsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed login persisted credentials: %v", err)
	}
}

func TestVerifyCredentialsUsesSavedPasswordAndDoesNotPersistGuard(t *testing.T) {
	client, cfg := newTestClient(t)
	var scriptBody string
	client.runCommand = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		scriptBody = credentialScript(t, args)
		return []byte("Logging in using password and Steam Guard code...OK\nWaiting for user info...OK"), nil
	}
	status, err := client.VerifyCredentials(t.Context(), "fixture_user", "654321")
	if err != nil || !status.LoggedIn {
		t.Fatalf("VerifyCredentials = %#v, %v", status, err)
	}
	if !strings.Contains(scriptBody, steamScriptArg("fixture password")) || !strings.Contains(scriptBody, steamScriptArg("654321")) {
		t.Fatalf("verification script = %q", scriptBody)
	}
	body, err := os.ReadFile(cfg.SteamWorkshopCredentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "654321") {
		t.Fatalf("Steam Guard code was persisted: %s", body)
	}
}

func TestExplicitLoginFailureRequiresAnActualSteamGuardError(t *testing.T) {
	if err := explicitLoginFailure([]byte("Logging in using password and Steam Guard code...OK\nLogged in OK")); err != nil {
		t.Fatalf("successful Steam Guard login was rejected: %v", err)
	}
	for _, output := range []string{
		"Login Failure: Two-factor code mismatch (88)",
		"FAILED (Invalid Login Auth Code)",
		"Account logon denied, need two-factor code",
	} {
		if err := explicitLoginFailure([]byte(output)); !errors.Is(err, ErrSteamGuardRequired) {
			t.Errorf("explicitLoginFailure(%q) = %v", output, err)
		}
	}
	if err := explicitLoginFailure([]byte("Please confirm the login in the Steam Mobile app on your phone.\nTimed out waiting for confirmation.")); !errors.Is(err, ErrSteamMobileConfirmationRequired) {
		t.Fatalf("mobile confirmation timeout = %v", err)
	}
	if err := explicitLoginFailure([]byte("Please confirm the login in the Steam Mobile app on your phone.\nLogged in OK")); err != nil {
		t.Fatalf("approved mobile confirmation was rejected: %v", err)
	}
	if err := explicitLoginFailure([]byte("Please confirm the login in the Steam Mobile app on your phone.\nWaiting for user info...\x1b[0mOK\n")); err != nil {
		t.Fatalf("approved mobile confirmation with ANSI output was rejected: %v", err)
	}
}

func TestCachedLoginFailureRequiresReauthorizationWithoutCache(t *testing.T) {
	if err := cachedLoginFailure([]byte("Cached credentials not found. FAILED (No cached credentials and @NoPromptForPassword is set)")); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("cached login failure = %v", err)
	}
	if err := cachedLoginFailure([]byte("Logging in using cached credentials.\nLogged in OK")); err != nil {
		t.Fatalf("cached login success = %v", err)
	}
	if err := cachedLoginFailure([]byte("Logging in using cached credentials.\x1b[0mOK\nWaiting for user info...\x1b[0mOK")); err != nil {
		t.Fatalf("cached login success with ANSI output = %v", err)
	}
}

func TestExplicitCredentialErrorsAreRedacted(t *testing.T) {
	client, _ := newTestClient(t)
	client.runCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("transport failure for fixture_user with fixture password and 123456"), errors.New("exit status 1")
	}
	_, err := client.Authenticate(t.Context(), LoginRequest{AccountName: "fixture_user", Password: "fixture password", SteamGuardCode: "123456"})
	if err == nil || strings.Contains(err.Error(), "fixture_user") || strings.Contains(err.Error(), "fixture password") || strings.Contains(err.Error(), "123456") {
		t.Fatalf("credential error was not redacted: %v", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("redaction marker missing: %v", err)
	}
}

func TestClearCredentialsRemovesSavedFile(t *testing.T) {
	client, cfg := newTestClient(t)
	status, err := client.ClearCredentials(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if status.PasswordConfigured || status.LoggedIn || !status.VerificationRequired {
		t.Fatalf("status = %#v", status)
	}
	if _, err := os.Stat(cfg.SteamWorkshopCredentialsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credential file remains: %v", err)
	}
	if leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(cfg.SteamWorkshopCredentialsPath()), "steamcmd-login-*.txt")); err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary scripts remain: %#v, %v", leftovers, err)
	}
}
