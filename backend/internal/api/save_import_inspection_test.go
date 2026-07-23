package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"palpanel/internal/appconfig"
)

func TestInspectSaveImportCandidatesSortsParsesAndRedacts(t *testing.T) {
	root := t.TempDir()
	writeCandidateLevel(t, root, "z-world", "valid-z")
	writeCandidateLevel(t, root, "A-world", "valid-a")
	writeCandidateLevel(t, root, "empty", "")
	writeCandidateLevel(t, root, "broken", "broken")
	writeCandidateLevel(t, root, filepath.Join("nested", "history", "ignored"), "valid-history")

	var parsed []string
	indexer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			SaveDir string `json:"save_dir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, filepath.ToSlash(strings.TrimPrefix(request.SaveDir, root+string(filepath.Separator))))
		w.Header().Set("Content-Type", "application/json")
		if filepath.Base(request.SaveDir) == "broken" {
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"parse_failed","message":"private ` + filepath.ToSlash(request.SaveDir) + `"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"version":1,"parser":"test-parser","warnings":["read ` + filepath.ToSlash(request.SaveDir) + `"],"players":[{"player_uid":"one"}],"guilds":[],"bases":[],"pals":[],"containers":[],"map_entities":[]}}`))
	}))
	defer indexer.Close()
	cfg := appconfig.Config{
		SaveIndexerEnabled:      true,
		SaveIndexerURL:          indexer.URL,
		SaveIndexTimeoutSeconds: 2,
		SaveIndexCacheDir:       filepath.Join(root, "private-cache"),
	}

	candidates, err := inspectSaveImportCandidates(context.Background(), cfg, root)
	if err != nil {
		t.Fatalf("inspectSaveImportCandidates returned error: %v", err)
	}
	paths := make([]string, len(candidates))
	for i, candidate := range candidates {
		paths[i] = candidate.RelativePath
		body, marshalErr := json.Marshal(candidate)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if strings.Contains(string(body), root) || strings.Contains(string(body), filepath.ToSlash(root)) {
			t.Fatalf("candidate leaked private root: %s", body)
		}
		if candidate.ID == "" || candidate.LevelSHA256 == "" && candidate.LevelSize > 0 {
			t.Fatalf("candidate metadata incomplete: %#v", candidate)
		}
	}
	if want := []string{"A-world", "broken", "empty", "z-world"}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("candidate paths = %#v, want %#v", paths, want)
	}
	if want := []string{"A-world", "broken", "z-world"}; !reflect.DeepEqual(parsed, want) {
		t.Fatalf("parsed candidates = %#v, want %#v", parsed, want)
	}
	if !candidates[0].Valid || candidates[0].PlayerCount != 1 || candidates[0].WorldID != "A-world" {
		t.Fatalf("valid candidate = %#v", candidates[0])
	}
	if candidates[1].Valid || len(candidates[1].Errors) == 0 {
		t.Fatalf("unparseable candidate = %#v", candidates[1])
	}
	if candidates[2].Valid || len(candidates[2].Errors) == 0 || candidates[2].LevelSize != 0 {
		t.Fatalf("empty candidate = %#v", candidates[2])
	}
}

func TestSaveImportInspectionSelectionExpiryAndDuplicateClaim(t *testing.T) {
	clock := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	store := newSaveImportInspectionStore(time.Minute)
	store.now = func() time.Time { return clock }
	inspection := &saveImportInspection{
		ID:   "inspect_one",
		Root: root,
		Candidates: []saveImportCandidate{
			{ID: "invalid", RelativePath: "bad", Valid: false},
			{ID: "valid", RelativePath: "world", Valid: true},
		},
	}
	store.put(inspection)

	if _, err := store.selectCandidate(inspection.ID, "invalid"); err != errSaveImportCandidateInvalid {
		t.Fatalf("invalid selection error = %v", err)
	}
	selected, err := store.selectCandidate(inspection.ID, "valid")
	if err != nil || selected.SelectedCandidateID != "valid" {
		t.Fatalf("selected inspection = %#v, %v", selected, err)
	}
	claimed, err := store.claim(inspection.ID)
	if err != nil || claimed.SelectedCandidateID != "valid" {
		t.Fatalf("claim = %#v, %v", claimed, err)
	}
	if _, err := store.claim(inspection.ID); err != errSaveImportAlreadyClaimed {
		t.Fatalf("duplicate claim error = %v", err)
	}

	expiredRoot := filepath.Join(t.TempDir(), "expired")
	if err := os.MkdirAll(expiredRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	store.put(&saveImportInspection{ID: "inspect_expired", Root: expiredRoot})
	clock = clock.Add(2 * time.Minute)
	if _, err := store.get("inspect_expired"); err != errSaveImportInspectionExpired {
		t.Fatalf("expired inspection error = %v", err)
	}
	if _, err := os.Stat(expiredRoot); !os.IsNotExist(err) {
		t.Fatalf("expired private root was not removed: %v", err)
	}
}

func TestClaimedSaveImportIsNotExpiredDuringFinalization(t *testing.T) {
	clock := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	store := newSaveImportInspectionStore(time.Minute)
	store.now = func() time.Time { return clock }
	store.put(&saveImportInspection{
		ID: "inspect_claimed", Root: root, SelectedCandidateID: "world",
		Candidates: []saveImportCandidate{{ID: "world", RelativePath: "world", Valid: true}},
	})
	if _, err := store.claim("inspect_claimed"); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * time.Minute)
	store.expire("inspect_claimed")
	if _, err := store.get("inspect_claimed"); err != nil {
		t.Fatalf("concurrent request expired a claimed inspection: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("claimed inspection root was removed: %v", err)
	}
}

func TestCleanupSaveImportInspectionsRemovesRestartOrphans(t *testing.T) {
	root := t.TempDir()
	cfg := appconfig.Config{SaveSourcesDir: filepath.Join(root, "save-sources")}
	orphan := filepath.Join(cfg.SaveSourcesDir, ".inspections", "inspect_orphan", "files")
	if err := os.MkdirAll(orphan, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cleanupSaveImportInspections(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.SaveSourcesDir, ".inspections")); !os.IsNotExist(err) {
		t.Fatalf("orphan inspection root was not removed: %v", err)
	}
}

func writeCandidateLevel(t *testing.T, root, relative, content string) {
	t.Helper()
	directory := filepath.Join(root, relative)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "Level.sav"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
