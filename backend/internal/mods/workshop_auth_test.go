package mods

import (
	"context"
	"errors"
	"testing"
	"time"

	"palpanel/internal/steamcmd"
)

type fakeWorkshopAuthenticator struct {
	status       steamcmd.LoginStatus
	startCalls   []string
	verifyCalls  []string
	requireCalls []string
	requireErr   error
}

func (f *fakeWorkshopAuthenticator) LoginStatus(account string) steamcmd.LoginStatus {
	status := f.status
	status.AccountName = account
	return status
}

func (f *fakeWorkshopAuthenticator) StartInteractiveLogin(_ context.Context, account string) (steamcmd.LoginStatus, error) {
	f.startCalls = append(f.startCalls, account)
	status := f.LoginStatus(account)
	status.LoginInProgress = true
	return status, nil
}

func (f *fakeWorkshopAuthenticator) VerifyLogin(_ context.Context, account string) (steamcmd.LoginStatus, error) {
	f.verifyCalls = append(f.verifyCalls, account)
	status := f.LoginStatus(account)
	status.LoggedIn = true
	status.VerificationRequired = false
	status.LastVerifiedAt = time.Now().UTC().Format(time.RFC3339)
	return status, nil
}

func (f *fakeWorkshopAuthenticator) RequireLogin(_ context.Context, account string) (steamcmd.LoginStatus, error) {
	f.requireCalls = append(f.requireCalls, account)
	if f.requireErr != nil {
		return f.LoginStatus(account), f.requireErr
	}
	return f.VerifyLogin(context.Background(), account)
}

func TestWorkshopAuthStatusReloadsPersistedAccountAndProbesCachedSession(t *testing.T) {
	manager, store := newImportTestManager(t)
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "persisted_user"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true, CredentialsSecure: true, VerificationRequired: true}}
	manager.steamAuth = fake

	status, err := manager.WorkshopAuthStatus(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !status.LoggedIn || status.AccountName != "persisted_user" || len(fake.verifyCalls) != 1 || fake.verifyCalls[0] != "persisted_user" {
		t.Fatalf("status = %#v, verify calls = %#v", status, fake.verifyCalls)
	}
}

func TestWorkshopAuthStatusDoesNotInstallSteamCMD(t *testing.T) {
	manager, store := newImportTestManager(t)
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
	fake := &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true}}
	manager.steamAuth = fake
	if _, err := manager.StartWorkshopLogin(t.Context(), " fixture_user "); err != nil {
		t.Fatal(err)
	}
	stored, configured, err := store.GetKV(t.Context(), workshopSteamAccountKey)
	if err != nil || !configured || stored != "fixture_user" || len(fake.startCalls) != 1 || fake.startCalls[0] != "fixture_user" {
		t.Fatalf("stored = %q, configured = %v, calls = %#v, error = %v", stored, configured, fake.startCalls, err)
	}
}

func TestVerifyWorkshopLoginReusesPersistedAccount(t *testing.T) {
	manager, store := newImportTestManager(t)
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "persisted_user"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeWorkshopAuthenticator{status: steamcmd.LoginStatus{Supported: true, SteamCMDInstalled: true}}
	manager.steamAuth = fake
	if _, err := manager.VerifyWorkshopLogin(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if len(fake.verifyCalls) != 1 || fake.verifyCalls[0] != "persisted_user" {
		t.Fatalf("verify calls = %#v", fake.verifyCalls)
	}
}

func TestWorkshopImportRejectsMissingLoginBeforeSubmittingJobAndReleasesClaim(t *testing.T) {
	manager, store := newImportTestManager(t)
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
