package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreJobsModsAndKV(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	job, err := store.CreateJob(ctx, "job_1", "install", "queued")
	if err != nil {
		t.Fatalf("CreateJob returned error: %v", err)
	}
	if job.Status != "queued" {
		t.Fatalf("job status = %q", job.Status)
	}
	if err := store.UpdateJob(ctx, "job_1", "completed", 100, "done", ""); err != nil {
		t.Fatalf("UpdateJob returned error: %v", err)
	}
	gotJob, err := store.GetJob(ctx, "job_1")
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if gotJob.Status != "completed" || gotJob.Progress != 100 {
		t.Fatalf("unexpected job: %#v", gotJob)
	}

	mod := Mod{ID: "mod_1", Name: "Test", Source: "upload", PackageName: "TestPackage", Path: "x", Enabled: true}
	if err := store.UpsertMod(ctx, mod); err != nil {
		t.Fatalf("UpsertMod returned error: %v", err)
	}
	mods, err := store.ListMods(ctx)
	if err != nil {
		t.Fatalf("ListMods returned error: %v", err)
	}
	if len(mods) != 1 || !mods[0].Enabled {
		t.Fatalf("unexpected mods: %#v", mods)
	}

	if err := store.SetKV(ctx, "pending_restart", "true"); err != nil {
		t.Fatalf("SetKV returned error: %v", err)
	}
	value, ok, err := store.GetKV(ctx, "pending_restart")
	if err != nil {
		t.Fatalf("GetKV returned error: %v", err)
	}
	if !ok || value != "true" {
		t.Fatalf("unexpected kv: %v %q", ok, value)
	}
}
