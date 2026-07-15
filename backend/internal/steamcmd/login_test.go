package steamcmd

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestValidateAccountNameRejectsSteamCMDArgumentInjection(t *testing.T) {
	for _, value := range []string{"", "ab", "user name", "user+quit", "+login", "user@example.com", "用户", strings.Repeat("a", 65)} {
		if err := ValidateAccountName(value); !errors.Is(err, ErrInvalidAccountName) {
			t.Errorf("ValidateAccountName(%q) = %v", value, err)
		}
	}
	for _, value := range []string{"abc", "Fixture_User_123", strings.Repeat("a", 64)} {
		if err := ValidateAccountName(value); err != nil {
			t.Errorf("ValidateAccountName(%q) = %v", value, err)
		}
	}
}

func TestVerifyCachedSessionUsesOnlyAccountNameAndNoPromptMode(t *testing.T) {
	client, _ := newTestClient(t)
	var captured []string
	client.runCommand = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		captured = append([]string(nil), args...)
		return []byte("Logging in using cached credentials.\nWaiting for user info...OK"), nil
	}
	verified, err := client.verifyCachedSession(t.Context(), "fixture_user")
	if err != nil || !verified {
		t.Fatalf("verifyCachedSession = %v, %v", verified, err)
	}
	want := []string{"+@ShutdownOnFailedCommand", "1", "+@NoPromptForPassword", "1", "+login", "fixture_user", "+quit"}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("SteamCMD arguments = %#v, want %#v", captured, want)
	}
}

func TestStartInteractiveLoginHardensCredentialsAndLaunchesNewSession(t *testing.T) {
	client, cfg := newTestClient(t)
	client.login = loginState{}
	hardened := false
	client.credentialHardener = func(_ context.Context, directory string) error {
		hardened = directory == cfg.SteamCMDDir
		return nil
	}
	client.interactiveLauncher = func(binary, directory, account string) error {
		if !hardened {
			t.Fatal("interactive launcher ran before the credential ACL hardener")
		}
		if binary != cfg.SteamCMDBinaryPath() || directory != cfg.SteamCMDDir || account != "fixture_user" {
			t.Fatalf("launcher values = %q, %q, %q", binary, directory, account)
		}
		return nil
	}
	status, err := client.StartInteractiveLogin(t.Context(), "fixture_user")
	if err != nil {
		t.Fatal(err)
	}
	if !status.LoginInProgress || status.LoggedIn || !status.CredentialsSecure {
		t.Fatalf("status = %#v", status)
	}
}

func TestVerifyLoginReturnsOperationalFailureWithoutExposingItInStatus(t *testing.T) {
	client, _ := newTestClient(t)
	client.login = loginState{}
	client.credentialHardener = func(context.Context, string) error { return nil }
	client.sessionVerifier = func(context.Context, string) (bool, error) {
		return false, errors.New("private cached credential detail")
	}
	status, err := client.VerifyLogin(t.Context(), "fixture_user")
	if err == nil || !strings.Contains(err.Error(), "private cached credential detail") {
		t.Fatalf("VerifyLogin error = %v", err)
	}
	if status.LoggedIn || status.LoginInProgress || strings.Contains(status.Message, "private") {
		t.Fatalf("status = %#v", status)
	}
}

func TestVerifyCachedSessionTreatsCredentialPromptAsLoggedOut(t *testing.T) {
	client, _ := newTestClient(t)
	client.runCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("FAILED TO LOG IN: password required"), errors.New("exit status 5")
	}
	verified, err := client.verifyCachedSession(t.Context(), "fixture_user")
	if err != nil || verified {
		t.Fatalf("verifyCachedSession = %v, %v", verified, err)
	}
}

func TestLoginStatusDoesNotLeakAnotherAccountsProgress(t *testing.T) {
	client, _ := newTestClient(t)
	client.login = loginState{
		account:           "first_user",
		verifiedAt:        time.Now(),
		loginInProgress:   true,
		credentialsSecure: true,
	}
	status := client.LoginStatus("second_user")
	if status.LoginInProgress || status.LoggedIn {
		t.Fatalf("status = %#v", status)
	}
}

func TestRequireLoginWithoutAccountIsStableLoginRequired(t *testing.T) {
	client, _ := newTestClient(t)
	if _, err := client.RequireLogin(t.Context(), ""); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("RequireLogin error = %v", err)
	}
}
