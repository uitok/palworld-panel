package mods

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
