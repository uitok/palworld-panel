package mods

import (
	"context"
	"errors"
	"testing"
	"time"

	"palpanel/internal/db"
	"palpanel/internal/server"
	"palpanel/internal/steamcmd"
)

type fakeWorkshopAuthenticator struct {
	status       steamcmd.LoginStatus
	startCalls   []steamcmd.LoginRequest
	verifyCalls  []string
	requireCalls []string
	requireErr   error
}

type fakeWorkshopDockerRunner struct {
	imageExists bool
	secure      bool
	verified    bool
	verifyCalls []string
	authCalls   []steamcmd.LoginRequest
}

func (f *fakeWorkshopDockerRunner) BuildImage(context.Context) error { return nil }
func (f *fakeWorkshopDockerRunner) DownloadWorkshopTo(context.Context, string, string, ...string) error {
	return nil
}
func (f *fakeWorkshopDockerRunner) ImageExists(context.Context) (bool, error) {
	return f.imageExists, nil
}
func (f *fakeWorkshopDockerRunner) AuthenticateWorkshop(_ context.Context, request steamcmd.LoginRequest) ([]byte, error) {
	f.authCalls = append(f.authCalls, request)
	return []byte("Logged in OK"), nil
}
func (f *fakeWorkshopDockerRunner) VerifyWorkshopLogin(_ context.Context, account string) (bool, error) {
	f.verifyCalls = append(f.verifyCalls, account)
	return f.verified, nil
}
func (f *fakeWorkshopDockerRunner) WorkshopCredentialsSecure() bool { return f.secure }

func (f *fakeWorkshopAuthenticator) CredentialStatus(account string) (steamcmd.LoginStatus, error) {
	status := f.status
	if account != "" {
		status.AccountName = account
	}
	return status, nil
}

func (f *fakeWorkshopAuthenticator) Authenticate(_ context.Context, request steamcmd.LoginRequest) (steamcmd.LoginStatus, error) {
	f.startCalls = append(f.startCalls, request)
	status, _ := f.CredentialStatus(request.AccountName)
	status.PasswordConfigured = true
	status.LoggedIn = true
	status.VerificationRequired = false
	return status, nil
}

func (f *fakeWorkshopAuthenticator) VerifyCredentials(_ context.Context, account, guard string) (steamcmd.LoginStatus, error) {
	f.verifyCalls = append(f.verifyCalls, account)
	status, _ := f.CredentialStatus(account)
	status.LoggedIn = true
	status.PasswordConfigured = true
	status.VerificationRequired = false
	status.LastVerifiedAt = time.Now().UTC().Format(time.RFC3339)
	return status, nil
}

func (f *fakeWorkshopAuthenticator) ClearCredentials(context.Context) (steamcmd.LoginStatus, error) {
	return steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true, VerificationRequired: true}, nil
}

func (f *fakeWorkshopAuthenticator) RequireCredentials(_ context.Context, account string) (steamcmd.LoginStatus, error) {
	f.requireCalls = append(f.requireCalls, account)
	if f.requireErr != nil {
		status, _ := f.CredentialStatus(account)
		return status, f.requireErr
	}
	return f.VerifyCredentials(context.Background(), account, "")
}

func TestWorkshopAuthStatusReloadsPersistedAccountAndProbesCachedSession(t *testing.T) {
	manager, store := newImportTestManager(t)
	useNativeWorkshopAuth(t, store)
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "persisted_user"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true, CredentialsSecure: true, VerificationRequired: true}}
	manager.steamAuth = fake

	status, err := manager.WorkshopAuthStatus(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if status.AccountName != "persisted_user" || len(fake.verifyCalls) != 0 {
		t.Fatalf("status = %#v, verify calls = %#v", status, fake.verifyCalls)
	}
}

func TestWorkshopAuthStatusDoesNotInstallSteamCMD(t *testing.T) {
	manager, store := newImportTestManager(t)
	useNativeWorkshopAuth(t, store)
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "persisted_user"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: false, VerificationRequired: true}}
	manager.steamAuth = fake
	status, err := manager.WorkshopAuthStatus(t.Context())
	if err != nil || status.SteamCMDInstalled || len(fake.verifyCalls) != 0 {
		t.Fatalf("status = %#v, verify calls = %#v, error = %v", status, fake.verifyCalls, err)
	}
}

func TestStartWorkshopLoginPersistsValidatedAccount(t *testing.T) {
	manager, store := newImportTestManager(t)
	useNativeWorkshopAuth(t, store)
	fake := &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true}}
	manager.steamAuth = fake
	if _, err := manager.StartWorkshopLogin(t.Context(), steamcmd.LoginRequest{AccountName: "fixture_user", Password: "fixture password"}); err != nil {
		t.Fatal(err)
	}
	stored, configured, err := store.GetKV(t.Context(), workshopSteamAccountKey)
	if err != nil || !configured || stored != "fixture_user" || len(fake.startCalls) != 1 || fake.startCalls[0].AccountName != "fixture_user" || fake.startCalls[0].Password != "fixture password" {
		t.Fatalf("stored = %q, configured = %v, calls = %#v, error = %v", stored, configured, fake.startCalls, err)
	}
}

func TestVerifyWorkshopLoginReusesPersistedAccount(t *testing.T) {
	manager, store := newImportTestManager(t)
	useNativeWorkshopAuth(t, store)
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "persisted_user"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true}}
	manager.steamAuth = fake
	if _, err := manager.VerifyWorkshopLogin(t.Context(), "", ""); err != nil {
		t.Fatal(err)
	}
	if len(fake.verifyCalls) != 1 || fake.verifyCalls[0] != "persisted_user" {
		t.Fatalf("verify calls = %#v", fake.verifyCalls)
	}
}

func TestWorkshopImportRejectsMissingLoginBeforeSubmittingJobAndReleasesClaim(t *testing.T) {
	manager, store := newImportTestManager(t)
	useNativeWorkshopAuth(t, store)
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "persisted_user"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWorkshopAuthenticator{
		status:     steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true},
		requireErr: steamcmd.ErrLoginRequired,
	}
	manager.steamAuth = fake
	inspection, err := manager.InspectSource(t.Context(), "3625364851")
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Import(t.Context(), inspection.ID, inspection.SelectedCandidateID)
	var failure ImportFailure
	if !errors.As(err, &failure) || failure.Code != "steam_login_required" {
		t.Fatalf("Import error = %v", err)
	}
	record := manager.imports.records[inspection.ID]
	if record == nil || record.claimed {
		t.Fatalf("inspection claim was not released: %#v", record)
	}
	jobs, listErr := store.ListJobs(t.Context(), 10)
	if listErr != nil || len(jobs) != 0 {
		t.Fatalf("jobs = %#v, error = %v", jobs, listErr)
	}
}

func TestDockerWorkshopAuthUsesSavedCredentialStatusWithoutCacheProbe(t *testing.T) {
	manager, store := newImportTestManager(t)
	if err := store.SetKV(t.Context(), "runtime_mode", server.RuntimeWineDocker); err != nil {
		t.Fatal(err)
	}
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "fixture_user"); err != nil {
		t.Fatal(err)
	}
	fakeRunner := &fakeWorkshopDockerRunner{imageExists: true, secure: true, verified: true}
	manager.runner = fakeRunner
	manager.steamAuth = &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{
		Supported: true, SteamCMDInstalled: true, LoggedIn: true, PasswordConfigured: true,
		CredentialsSecure: true, AccountName: "fixture_user", LastVerifiedAt: time.Now().UTC().Format(time.RFC3339),
	}}

	status, err := manager.WorkshopAuthStatus(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Supported || !status.SteamCMDInstalled || !status.LoggedIn || !status.CredentialsSecure || status.VerificationRequired || status.AccountName != "fixture_user" {
		t.Fatalf("Docker/Wine auth status = %#v", status)
	}
	if len(fakeRunner.verifyCalls) != 0 {
		t.Fatalf("legacy cache probe calls = %#v", fakeRunner.verifyCalls)
	}
	if _, err := manager.RequireWorkshopLogin(t.Context()); err != nil {
		t.Fatalf("RequireWorkshopLogin returned error: %v", err)
	}
	if _, err := manager.VerifyWorkshopLogin(t.Context(), "", ""); err != nil {
		t.Fatalf("VerifyWorkshopLogin returned error: %v", err)
	}
}

func TestDockerWorkshopAuthRequiresConfiguredPasswordInsteadOfLegacyCache(t *testing.T) {
	manager, store := newImportTestManager(t)
	if err := store.SetKV(t.Context(), "runtime_mode", server.RuntimeWineDocker); err != nil {
		t.Fatal(err)
	}
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "fixture_user"); err != nil {
		t.Fatal(err)
	}
	manager.runner = &fakeWorkshopDockerRunner{imageExists: true, secure: true, verified: false}
	manager.steamAuth = &fakeWorkshopAuthenticator{
		status:     steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true, AccountName: "fixture_user", VerificationRequired: true},
		requireErr: steamcmd.ErrLoginRequired,
	}

	status, err := manager.WorkshopAuthStatus(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if status.LoggedIn || !status.VerificationRequired || status.PasswordConfigured {
		t.Fatalf("unconfigured Docker/Wine auth status = %#v", status)
	}
	if _, err := manager.RequireWorkshopLogin(t.Context()); !errors.Is(err, steamcmd.ErrLoginRequired) {
		t.Fatalf("RequireWorkshopLogin error = %v", err)
	}
	if _, err := manager.StartWorkshopLogin(t.Context(), steamcmd.LoginRequest{AccountName: "fixture_user", Password: "fixture password"}); err != nil {
		t.Fatalf("StartWorkshopLogin error = %v", err)
	}
}

func useNativeWorkshopAuth(t *testing.T, store *db.Store) {
	t.Helper()
	if err := store.SetKV(t.Context(), "runtime_mode", server.RuntimeWindowsSteamCMD); err != nil {
		t.Fatal(err)
	}
}
