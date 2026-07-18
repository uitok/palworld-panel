package saveindex

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"palpanel/internal/appconfig"
)

func TestRebuildWritesCacheAndCurrentReadsIt(t *testing.T) {
	root, cfg := testConfig(t)
	writeWorld(t, root, "level-one")
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"version":      1,
				"source_path":  "ignored",
				"generated_at": "2026-07-09T00:00:00Z",
				"duration_ms":  7,
				"parser":       "test",
				"warnings":     []string{},
				"players":      []map[string]any{{"player_uid": "p1", "nickname": "Tester", "level": 2}},
				"guilds":       []map[string]any{},
				"bases":        []map[string]any{},
				"pals":         []map[string]any{},
				"containers":   []map[string]any{},
				"map_entities": []map[string]any{},
			},
		})
	}))
	defer sidecar.Close()
	cfg.SaveIndexerURL = sidecar.URL

	m := NewManager(cfg)
	index, status, err := m.Rebuild(t.Context())
	if err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	if status.State != "ready" || status.Counts.Players != 1 || index.Players[0].Nickname != "Tester" {
		t.Fatalf("unexpected rebuild result: status=%#v index=%#v", status, index)
	}

	index, status, err = m.Current(t.Context())
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if status.State != "ready" || status.Stale || len(index.Players) != 1 {
		t.Fatalf("unexpected current result: status=%#v index=%#v", status, index)
	}
}

func TestRebuildRetriesWhenSaveChangesDuringIndexing(t *testing.T) {
	root, cfg := testConfig(t)
	writeWorld(t, root, "level-one")
	calls := 0
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			writeWorld(t, root, "level-two")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"version": 1, "generated_at": "2026-07-18T00:00:00Z", "parser": "test", "warnings": []string{},
				"players": []map[string]any{}, "guilds": []map[string]any{}, "bases": []map[string]any{},
				"pals": []map[string]any{}, "containers": []map[string]any{}, "map_entities": []map[string]any{},
			},
		})
	}))
	defer sidecar.Close()
	cfg.SaveIndexerURL = sidecar.URL

	m := NewManager(cfg)
	_, status, err := m.Rebuild(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || status.State != "ready" || status.Stale {
		t.Fatalf("rebuild did not settle after retry: calls=%d status=%#v", calls, status)
	}
	_, currentStatus, err := m.Current(t.Context())
	if err != nil || currentStatus.State != "ready" {
		t.Fatalf("settled cache is not current: %#v, %v", currentStatus, err)
	}
}

func TestCurrentBeforeFirstIndexIsNotAnError(t *testing.T) {
	root, cfg := testConfig(t)
	writeWorld(t, root, "level-one")
	sidecarCalled := false
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sidecarCalled = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer sidecar.Close()
	cfg.SaveIndexerURL = sidecar.URL

	m := NewManager(cfg)
	index, status, err := m.Current(t.Context())
	if err != nil {
		t.Fatalf("Current returned an error before the first index: %v", err)
	}
	if status.State != "not_indexed" || status.Error != "" || len(index.Players) != 0 {
		t.Fatalf("unexpected current result: status=%#v index=%#v", status, index)
	}
	status = m.Status(t.Context())
	if status.State != "not_indexed" || status.Error != "" {
		t.Fatalf("unexpected status before the first index: %#v", status)
	}
	if sidecarCalled {
		t.Fatal("Current should not call the sidecar indexer on cache miss")
	}
}

func TestFingerprintUsesMetadataOnly(t *testing.T) {
	root, _ := testConfig(t)
	writeWorld(t, root, "same-size")
	world := filepath.Join(root, "server", "Pal", "Saved", "SaveGames", "0", "world")
	level := filepath.Join(world, "Level.sav")
	st, err := os.Stat(level)
	if err != nil {
		t.Fatalf("stat Level.sav: %v", err)
	}
	before, err := fingerprintWorld(world)
	if err != nil {
		t.Fatalf("fingerprintWorld returned error: %v", err)
	}
	if err := os.WriteFile(level, []byte("diff-size"), 0o644); err != nil {
		t.Fatalf("rewrite Level.sav: %v", err)
	}
	if err := os.Chtimes(level, st.ModTime(), st.ModTime()); err != nil {
		t.Fatalf("restore mtime: %v", err)
	}
	after, err := fingerprintWorld(world)
	if err != nil {
		t.Fatalf("fingerprintWorld returned error: %v", err)
	}
	if before != after {
		t.Fatalf("metadata-only fingerprint changed after same-size same-mtime content edit: before=%s after=%s", before, after)
	}
}

func TestFingerprintIgnoresUnusedLevelMetaChanges(t *testing.T) {
	root, _ := testConfig(t)
	writeWorld(t, root, "level")
	world := filepath.Join(root, "server", "Pal", "Saved", "SaveGames", "0", "world")
	before, err := fingerprintWorld(world)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(world, "LevelMeta.sav"), []byte("updated metadata"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := fingerprintWorld(world)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("unused LevelMeta.sav changed the index fingerprint: %s -> %s", before, after)
	}
}

func TestRebuildFailureKeepsStaleCache(t *testing.T) {
	root, cfg := testConfig(t)
	writeWorld(t, root, "level-one")
	fail := false
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": map[string]any{"code": "parser_failed", "message": "boom"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"version":      1,
				"generated_at": "2026-07-09T00:00:00Z",
				"parser":       "test",
				"warnings":     []string{},
				"players":      []map[string]any{{"player_uid": "p1", "nickname": "Tester"}},
				"guilds":       []map[string]any{},
				"bases":        []map[string]any{},
				"pals":         []map[string]any{},
				"containers":   []map[string]any{},
				"map_entities": []map[string]any{},
			},
		})
	}))
	defer sidecar.Close()
	cfg.SaveIndexerURL = sidecar.URL

	m := NewManager(cfg)
	if _, _, err := m.Rebuild(t.Context()); err != nil {
		t.Fatalf("initial Rebuild returned error: %v", err)
	}
	writeWorld(t, root, "level-two")
	fail = true
	index, status, err := m.Rebuild(t.Context())
	if err == nil {
		t.Fatal("expected rebuild error")
	}
	if !status.Stale || status.State != "error" || len(index.Players) != 1 {
		t.Fatalf("expected stale cached index after failure, got status=%#v index=%#v", status, index)
	}
}

func TestRebuildNeverReturnsRawBinarySidecarPayload(t *testing.T) {
	root, cfg := testConfig(t)
	writeWorld(t, root, "level-one")
	rawSave := append([]byte("GVAS"), bytes.Repeat([]byte{0x00, 0xff, 0x81, 0x10}, 256)...)
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(rawSave)
	}))
	defer sidecar.Close()
	cfg.SaveIndexerURL = sidecar.URL

	_, status, err := NewManager(cfg).Rebuild(t.Context())
	if err == nil {
		t.Fatal("expected a non-JSON sidecar response to fail")
	}
	for _, message := range []string{err.Error(), status.Error} {
		if strings.ContainsRune(message, '\x00') || strings.Contains(message, "GVAS") || strings.Contains(message, "\\x00") {
			t.Fatalf("binary save content leaked into diagnostic text: %q", message)
		}
		if !strings.Contains(message, "non-JSON response") {
			t.Fatalf("unexpected safe diagnostic: %q", message)
		}
	}
}

func TestRebuildNeverReturnsBinaryTextFromStructuredSidecarError(t *testing.T) {
	root, cfg := testConfig(t)
	writeWorld(t, root, "level-one")
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": false,
			"error": map[string]any{
				"code":    "parse_failed",
				"message": "GVAS \\x00\\xff raw payload",
			},
		})
	}))
	defer sidecar.Close()
	cfg.SaveIndexerURL = sidecar.URL

	_, status, err := NewManager(cfg).Rebuild(t.Context())
	if err == nil {
		t.Fatal("expected structured sidecar error to fail")
	}
	for _, message := range []string{err.Error(), status.Error} {
		if strings.Contains(message, "GVAS") || strings.Contains(message, "\\x") || strings.ContainsRune(message, '\x00') {
			t.Fatalf("sidecar error payload leaked into diagnostic text: %q", message)
		}
		if !strings.Contains(message, "inspect the sav-cli text logs") {
			t.Fatalf("unexpected safe diagnostic: %q", message)
		}
	}
}

func TestLoadCacheUsesMemoryUntilFileMTimeChanges(t *testing.T) {
	root, cfg := testConfig(t)
	writeWorld(t, root, "level-one")
	sidecar := successfulSidecar(t, "Tester")
	defer sidecar.Close()
	cfg.SaveIndexerURL = sidecar.URL
	m := NewManager(cfg)
	if _, _, err := m.Rebuild(t.Context()); err != nil {
		t.Fatalf("Rebuild returned error: %v", err)
	}
	cachePath := filepath.Join(cfg.SaveIndexCacheDir, "index-cache.json")
	st, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("stat cache: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("corrupt cache: %v", err)
	}
	if err := os.Chtimes(cachePath, st.ModTime(), st.ModTime()); err != nil {
		t.Fatalf("restore cache mtime: %v", err)
	}
	index, _, err := m.Current(t.Context())
	if err != nil || index.Players[0].Nickname != "Tester" {
		t.Fatalf("expected memory cache despite on-disk corruption, err=%v index=%#v", err, index)
	}
	later := st.ModTime().Add(2 * time.Second)
	if err := os.Chtimes(cachePath, later, later); err != nil {
		t.Fatalf("advance cache mtime: %v", err)
	}
	_, _, err = m.Current(t.Context())
	if err == nil || errors.Is(err, ErrDisabled) {
		t.Fatalf("expected changed cache mtime to invalidate memory cache, got %v", err)
	}
}

func successfulSidecar(t *testing.T, nickname string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"data": map[string]any{
				"version":      1,
				"generated_at": "2026-07-09T00:00:00Z",
				"parser":       "test",
				"warnings":     []string{},
				"players":      []map[string]any{{"player_uid": "p1", "nickname": nickname}},
				"guilds":       []map[string]any{},
				"bases":        []map[string]any{},
				"pals":         []map[string]any{},
				"containers":   []map[string]any{},
				"map_entities": []map[string]any{},
			},
		})
	}))
}

func testConfig(t *testing.T) (string, appconfig.Config) {
	t.Helper()
	root := t.TempDir()
	return root, appconfig.Config{
		ServerDir:               filepath.Join(root, "server"),
		SaveIndexerEnabled:      true,
		SaveIndexCacheDir:       filepath.Join(root, "cache"),
		SaveIndexTimeoutSeconds: 5,
	}
}

func writeWorld(t *testing.T, root, content string) {
	t.Helper()
	world := filepath.Join(root, "server", "Pal", "Saved", "SaveGames", "0", "world")
	if err := os.MkdirAll(world, 0o755); err != nil {
		t.Fatalf("MkdirAll world: %v", err)
	}
	if err := os.WriteFile(filepath.Join(world, "Level.sav"), []byte(content), 0o644); err != nil {
		t.Fatalf("write Level.sav: %v", err)
	}
	if err := os.WriteFile(filepath.Join(world, "LevelMeta.sav"), []byte("meta"), 0o644); err != nil {
		t.Fatalf("write LevelMeta.sav: %v", err)
	}
}
