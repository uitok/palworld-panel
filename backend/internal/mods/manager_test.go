package mods

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
)

func TestSearchWorkshopMergesInstalledState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"result":1,"total":1,"publishedfiledetails":[{"publishedfileid":"123456","result":1,"title":"Installed Mod","time_updated":200}]}}`))
	}))
	defer server.Close()

	root := t.TempDir()
	cfg := appconfig.Config{
		DataDir:        root,
		ServerDir:      filepath.Join(root, "server"),
		UploadsDir:     filepath.Join(root, "uploads"),
		DBPath:         filepath.Join(root, "test.db"),
		SteamWebAPIKey: "key",
		WorkshopAppID:  "1623730",
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	defer store.Close()
	if err := store.UpsertMod(context.Background(), db.Mod{
		ID:          "123456",
		Name:        "Installed Mod",
		Source:      "workshop",
		PackageName: "InstalledPkg",
		Path:        filepath.Join(root, "server", "Mods", "Workshop", "123456"),
		Enabled:     true,
		WorkshopID:  "123456",
		TimeUpdated: 100,
	}); err != nil {
		t.Fatalf("UpsertMod returned error: %v", err)
	}

	manager := NewManager(cfg, store, docker.NewRunner(cfg))
	manager.workshop.client.baseURL = server.URL

	result, err := manager.SearchWorkshop(context.Background(), WorkshopSearchParams{PageSize: 10})
	if err != nil {
		t.Fatalf("SearchWorkshop returned error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("unexpected items: %#v", result.Items)
	}
	item := result.Items[0]
	if !item.Installed || !item.Enabled || !item.UpdateAvailable || item.ModID != "123456" {
		t.Fatalf("workshop state was not merged: %#v", item)
	}
}

func TestDeleteRejectsModPathOutsideManagedRoot(t *testing.T) {
	manager, store := newImportTestManager(t)
	outside := t.TempDir()
	marker := filepath.Join(outside, "must-survive.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	mod := db.Mod{ID: "mod_untrusted", Name: "Untrusted", PackageName: "Untrusted", Path: outside}
	if err := store.UpsertMod(t.Context(), mod); err != nil {
		t.Fatal(err)
	}

	err := manager.Delete(t.Context(), mod.ID)
	if err == nil || (!strings.Contains(err.Error(), "mod target") && !strings.Contains(err.Error(), "runtime root")) {
		t.Fatalf("Delete error = %v", err)
	}
	if body, readErr := os.ReadFile(marker); readErr != nil || string(body) != "keep" {
		t.Fatalf("outside marker was modified: %q, %v", body, readErr)
	}
	if _, getErr := store.GetMod(t.Context(), mod.ID); getErr != nil {
		t.Fatalf("database record should remain after refused deletion: %v", getErr)
	}
}
