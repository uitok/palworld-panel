package db

import (
	"context"
	"database/sql"
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

	mod := Mod{
		ID:            "mod_1",
		Name:          "Test",
		Source:        "upload",
		PackageName:   "TestPackage",
		Path:          "x",
		Enabled:       true,
		WorkshopID:    "123456",
		PreviewURL:    "https://cdn.example/mod.jpg",
		SteamURL:      "https://steamcommunity.com/sharedfiles/filedetails/?id=123456",
		Summary:       "summary",
		Tags:          []string{"QoL", "Server"},
		FileSize:      123,
		Subscriptions: 456,
		TimeUpdated:   789,
		LastCheckedAt: "2026-01-02T03:04:05Z",
	}
	if err := store.UpsertMod(ctx, mod); err != nil {
		t.Fatalf("UpsertMod returned error: %v", err)
	}
	mods, err := store.ListMods(ctx)
	if err != nil {
		t.Fatalf("ListMods returned error: %v", err)
	}
	if len(mods) != 1 || !mods[0].Enabled || mods[0].WorkshopID != "123456" || len(mods[0].Tags) != 2 {
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

func TestStoreMigratesLegacyModsTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	_, err = d.Exec(`CREATE TABLE mods (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		source TEXT NOT NULL,
		package_name TEXT NOT NULL,
		path TEXT NOT NULL,
		version TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create legacy mods table returned error: %v", err)
	}
	_, err = d.Exec(`INSERT INTO mods (id,name,source,package_name,path,version,enabled,created_at,updated_at)
		VALUES ('mod_legacy','Legacy','workshop','LegacyPkg','/tmp/mod','','1','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert legacy mod returned error: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("close legacy db returned error: %v", err)
	}

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer store.Close()

	mods, err := store.ListMods(context.Background())
	if err != nil {
		t.Fatalf("ListMods returned error: %v", err)
	}
	if len(mods) != 1 || mods[0].ID != "mod_legacy" || !mods[0].Enabled {
		t.Fatalf("legacy mod was not preserved: %#v", mods)
	}
	if mods[0].WorkshopID != "" || mods[0].PreviewURL != "" || mods[0].FileSize != 0 {
		t.Fatalf("legacy mod defaults not applied: %#v", mods[0])
	}
}
