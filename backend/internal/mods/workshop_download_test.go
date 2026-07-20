package mods

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"palpanel/internal/db"
	"palpanel/internal/server"
)

type fakeNativeWorkshopDownloader struct {
	ensureCalls   int
	downloadCalls int
	ensureErr     error
	downloadErr   error
	download      func(appID, itemID, destination string) error
}

func (f *fakeNativeWorkshopDownloader) Ensure(context.Context) error {
	f.ensureCalls++
	return f.ensureErr
}

func (f *fakeNativeWorkshopDownloader) DownloadWorkshopTo(_ context.Context, appID, itemID, destination string) error {
	f.downloadCalls++
	if f.downloadErr != nil {
		return f.downloadErr
	}
	if f.download != nil {
		return f.download(appID, itemID, destination)
	}
	return nil
}

func TestRunWorkshopImportUsesNativeSteamCMDForWindowsRuntime(t *testing.T) {
	manager, store := newImportTestManager(t)
	if err := store.SetKV(t.Context(), "runtime_mode", server.RuntimeWindowsSteamCMD); err != nil {
		t.Fatal(err)
	}
	if err := store.SetKV(t.Context(), workshopSteamAccountKey, "fixture_user"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeNativeWorkshopDownloader{download: func(appID, itemID, destination string) error {
		if appID != manager.cfg.WorkshopAppID || itemID != "123456789" {
			t.Fatalf("native download IDs = %q, %q", appID, itemID)
		}
		item := filepath.Join(destination, itemID)
		if err := os.MkdirAll(item, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(item, "Info.json"), []byte(`{"Name":"Native Fixture","PackageName":"NativeFixture","Version":"1"}`), 0o644)
	}}
	manager.native = fake
	job, directory := newWorkshopImportJob(t, manager, store, "native-workshop")

	manager.runWorkshopImport(t.Context(), job.ID, "123456789", false, WorkshopItem{ID: "123456789"}, directory)
	completed, err := store.GetJob(t.Context(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != "completed" || completed.Progress != 100 {
		t.Fatalf("job = %#v", completed)
	}
	if fake.ensureCalls != 1 || fake.downloadCalls != 1 {
		t.Fatalf("native calls = ensure %d, download %d", fake.ensureCalls, fake.downloadCalls)
	}
	installed, err := store.ListMods(t.Context())
	if err != nil || len(installed) != 1 || installed[0].Source != "workshop" || installed[0].WorkshopID != "123456789" {
		t.Fatalf("installed mods = %#v, %v", installed, err)
	}
}

func TestRunWorkshopImportDoesNotReportIncompleteNativeDownloadAsSuccess(t *testing.T) {
	manager, store := newImportTestManager(t)
	if err := store.SetKV(t.Context(), "runtime_mode", server.RuntimeWindowsSteamCMD); err != nil {
		t.Fatal(err)
	}
	manager.native = &fakeNativeWorkshopDownloader{}
	job, directory := newWorkshopImportJob(t, manager, store, "incomplete-workshop")

	manager.runWorkshopImport(t.Context(), job.ID, "123456789", false, WorkshopItem{}, directory)
	failed, err := store.GetJob(t.Context(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || failed.Progress != 80 || !strings.Contains(failed.Error, "Workshop") {
		t.Fatalf("job = %#v", failed)
	}
	installed, err := store.ListMods(t.Context())
	if err != nil || len(installed) != 0 {
		t.Fatalf("incomplete download created records: %#v, %v", installed, err)
	}
}

func TestRunWorkshopImportKeepsDockerWineBranch(t *testing.T) {
	manager, store := newImportTestManager(t)
	if err := store.SetKV(t.Context(), "runtime_mode", server.RuntimeWineDocker); err != nil {
		t.Fatal(err)
	}
	fake := &fakeNativeWorkshopDownloader{ensureErr: errors.New("native path must not run")}
	manager.native = fake
	job, directory := newWorkshopImportJob(t, manager, store, "wine-workshop")

	manager.runWorkshopImport(t.Context(), job.ID, "123456789", false, WorkshopItem{}, directory)
	failed, err := store.GetJob(t.Context(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || !strings.Contains(failed.Error, "build wine runner image") {
		t.Fatalf("job = %#v", failed)
	}
	if fake.ensureCalls != 0 || fake.downloadCalls != 0 {
		t.Fatalf("native path ran in Wine mode: %#v", fake)
	}
}

func newWorkshopImportJob(t *testing.T, manager Manager, store *db.Store, id string) (db.Job, string) {
	t.Helper()
	record, err := manager.imports.newRecord(id)
	if err != nil {
		t.Fatal(err)
	}
	job, err := store.CreateJob(t.Context(), id, "workshop_download", "queued")
	if err != nil {
		t.Fatal(err)
	}
	return job, record.directory
}
