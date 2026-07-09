package paldefender

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/id"
)

const (
	releasesURL = "https://api.github.com/repos/Ultimeit/PalDefender/releases"
	latestURL   = "https://api.github.com/repos/Ultimeit/PalDefender/releases/latest"
	kvVersion   = "paldefender_version"
	kvRESTToken = "paldefender_rest_token"
)

type Manager struct {
	cfg         appconfig.Config
	store       *db.Store
	client      *http.Client
	restBaseURL string
}

type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest,omitempty"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Status struct {
	Installed       bool              `json:"installed"`
	Version         string            `json:"version,omitempty"`
	NeedsFirstStart bool              `json:"needs_first_start"`
	Files           map[string]bool   `json:"files"`
	Paths           map[string]string `json:"paths"`
	RESTAPIEnabled  bool              `json:"rest_api_enabled"`
	Warnings        []string          `json:"warnings"`
}

type TokenResult struct {
	Name        string   `json:"name"`
	Token       string   `json:"token"`
	Permissions []string `json:"permissions"`
	Path        string   `json:"path"`
}

func NewManager(cfg appconfig.Config, store *db.Store) Manager {
	return Manager{cfg: cfg, store: store, client: &http.Client{Timeout: 60 * time.Second}, restBaseURL: cfg.EffectivePalDefenderRESTBaseURL()}
}

func (m Manager) Releases(ctx context.Context) ([]Release, error) {
	var releases []Release
	if err := m.getJSON(ctx, releasesURL, &releases); err != nil {
		return nil, err
	}
	if len(releases) > 5 {
		releases = releases[:5]
	}
	return releases, nil
}

func (m Manager) Latest(ctx context.Context) (Release, error) {
	var release Release
	err := m.getJSON(ctx, latestURL, &release)
	return release, err
}

func (m Manager) Status(ctx context.Context) (Status, error) {
	version, _, err := m.store.GetKV(ctx, kvVersion)
	if err != nil {
		return Status{}, err
	}
	files := map[string]bool{
		"PalDefender.dll": fileExists(filepath.Join(m.cfg.Win64Dir(), "PalDefender.dll")),
		"d3d9.dll":        fileExists(filepath.Join(m.cfg.Win64Dir(), "d3d9.dll")),
		"Config.json":     fileExists(m.configPath()),
		"RESTConfig.json": fileExists(m.restConfigPath()),
	}
	installed := files["PalDefender.dll"] && files["d3d9.dll"]
	var warnings []string
	if installed && !dirExists(m.cfg.PalDefenderDir()) {
		warnings = append(warnings, "PalDefender directory has not been generated yet; start the server once")
	}
	return Status{
		Installed:       installed,
		Version:         version,
		NeedsFirstStart: installed && !dirExists(m.cfg.PalDefenderDir()),
		Files:           files,
		Paths: map[string]string{
			"win64":       m.cfg.Win64Dir(),
			"paldefender": m.cfg.PalDefenderDir(),
			"config":      m.configPath(),
			"rest_config": m.restConfigPath(),
			"tokens":      m.tokensDir(),
			"rest_api":    m.restBaseURL,
		},
		RESTAPIEnabled: m.restEnabled(),
		Warnings:       warnings,
	}, nil
}

func (m Manager) Install(ctx context.Context) (db.Job, error) {
	return m.installJob(ctx, "paldefender_install", "queued PalDefender install")
}

func (m Manager) Update(ctx context.Context) (db.Job, error) {
	return m.installJob(ctx, "paldefender_update", "queued PalDefender update")
}

func (m Manager) Rollback(ctx context.Context) (Status, error) {
	backup, err := m.latestBackup()
	if err != nil {
		return Status{}, err
	}
	for _, name := range []string{"PalDefender.dll", "d3d9.dll"} {
		src := filepath.Join(backup, name)
		if fileExists(src) {
			if err := copyFile(src, filepath.Join(m.cfg.Win64Dir(), name)); err != nil {
				return Status{}, err
			}
		}
	}
	for _, rel := range []string{
		filepath.Join("PalDefender", "Config.json"),
		filepath.Join("PalDefender", "RESTAPI", "RESTConfig.json"),
	} {
		src := filepath.Join(backup, rel)
		if fileExists(src) {
			if err := copyFile(src, filepath.Join(m.cfg.Win64Dir(), rel)); err != nil {
				return Status{}, err
			}
		}
	}
	return m.Status(ctx)
}

func (m Manager) ReadConfig() (map[string]any, error) {
	if !fileExists(m.configPath()) {
		return BalancedPreset(), nil
	}
	b, err := os.ReadFile(m.configPath())
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (m Manager) WriteConfig(cfg map[string]any) (map[string]any, error) {
	if cfg == nil {
		cfg = map[string]any{}
	}
	if err := os.MkdirAll(filepath.Dir(m.configPath()), 0o755); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(m.configPath(), append(b, '\n'), 0o644); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (m Manager) ApplyPreset(name string) (map[string]any, error) {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "", "balanced":
		return m.WriteConfig(BalancedPreset())
	default:
		return nil, fmt.Errorf("unknown PalDefender preset: %s", name)
	}
}

func (m Manager) CreateRESTToken(ctx context.Context, name string, permissions []string) (TokenResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "AdminPanel"
	}
	if len(permissions) == 0 {
		permissions = []string{"REST.*"}
	}
	token, err := randomToken()
	if err != nil {
		return TokenResult{}, err
	}
	if err := os.MkdirAll(m.tokensDir(), 0o755); err != nil {
		return TokenResult{}, err
	}
	payload := map[string]any{"Name": name, "Token": token, "Permissions": permissions}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return TokenResult{}, err
	}
	path := filepath.Join(m.tokensDir(), safeName(name)+".json")
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return TokenResult{}, err
	}
	if err := m.enableRESTConfig(); err != nil {
		return TokenResult{}, err
	}
	_ = m.store.SetKV(ctx, kvRESTToken, token)
	return TokenResult{Name: name, Token: token, Permissions: permissions, Path: path}, nil
}

func (m Manager) ReloadConfig(ctx context.Context) error {
	token, _, err := m.store.GetKV(ctx, kvRESTToken)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("PalDefender REST token has not been generated")
	}
	paths := []string{"/ReloadConfig", "/v1/pdapi/ReloadConfig"}
	var lastErr error
	for _, path := range paths {
		if err := m.postREST(ctx, path, token); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (m Manager) postREST(ctx context.Context, path, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(m.restBaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("PalDefender REST API %s returned status %d", path, resp.StatusCode)
	}
	return nil
}

func BalancedPreset() map[string]any {
	return map[string]any{
		"version":                      "1.0.0",
		"MOTD":                         []string{"Welcome to {ServerName}", "PvP: {IsPvP} | Death penalty: {DeathPenalty}"},
		"exitServerOnStartupFailure":   true,
		"preventAdminPasswordInChat":   true,
		"shouldWarnCheaters":           true,
		"shouldWarnCheatersReason":     true,
		"shouldKickCheaters":           true,
		"shouldBanCheaters":            false,
		"shouldIPBanCheaters":          false,
		"logChat":                      true,
		"logRCON":                      true,
		"logPlayerUID":                 true,
		"logPlayerIP":                  true,
		"logPlayerDeaths":              true,
		"logPlayerLogins":              true,
		"logPlayerBuildings":           true,
		"logPlayerSummons":             true,
		"logPlayerCaptures":            true,
		"logCraftings":                 true,
		"logTechUnlocks":               true,
		"useAdminWhitelist":            false,
		"adminAutoLogin":               false,
		"adminIPs":                     []string{},
		"bannedChatWords":              []string{},
		"bannedNames":                  []string{},
		"announceConnections":          true,
		"dontAnnounceAdminConnections": true,
		"announcePunishments":          true,
		"useWhitelist":                 false,
		"whitelistMessage":             "This server uses a whitelist.",
		"steamidProtection":            true,
		"blockTowerBossCapture":        true,
		"disableIllegalItemProtection": false,
		"disableButchering":            false,
		"disableRenaming":              false,
		"disablePalRenaming":           false,
		"doActionUponIllegalPalStats":  true,
		"palStatsMaxRank":              -1,
		"bannedTechnologies":           []string{},
	}
}

func (m Manager) installJob(ctx context.Context, typ, message string) (db.Job, error) {
	j, err := m.store.CreateJob(ctx, id.New("job"), typ, message)
	if err != nil {
		return db.Job{}, err
	}
	go func() {
		m.update(j.ID, "running", 10, "fetching latest PalDefender release", "")
		release, err := m.Latest(context.Background())
		if err != nil {
			m.update(j.ID, "failed", 10, "release lookup failed", err.Error())
			return
		}
		m.update(j.ID, "running", 30, "downloading PalDefender assets", "")
		if err := m.installRelease(context.Background(), release); err != nil {
			m.update(j.ID, "failed", 30, "install failed", err.Error())
			return
		}
		_ = m.store.SetKV(context.Background(), kvVersion, release.TagName)
		m.update(j.ID, "completed", 100, "PalDefender "+release.TagName+" installed", "")
	}()
	return j, nil
}

func (m Manager) installRelease(ctx context.Context, release Release) error {
	if err := os.MkdirAll(m.cfg.Win64Dir(), 0o755); err != nil {
		return err
	}
	if err := m.backupCurrent(); err != nil {
		return err
	}
	zipAsset := findAsset(release.Assets, "PalDefender.zip")
	if zipAsset.BrowserDownloadURL != "" {
		zipPath := filepath.Join(m.cfg.ToolsDir, "PalDefender-"+release.TagName+".zip")
		if err := m.downloadAsset(ctx, zipAsset, zipPath); err != nil {
			return err
		}
		tmp := filepath.Join(m.cfg.ToolsDir, "paldefender-"+release.TagName)
		_ = os.RemoveAll(tmp)
		if err := extractZipSafe(zipPath, tmp); err != nil {
			return err
		}
		return m.copyInstalledFiles(tmp)
	}
	for _, name := range []string{"PalDefender.dll", "d3d9.dll"} {
		asset := findAsset(release.Assets, name)
		if asset.BrowserDownloadURL == "" {
			return fmt.Errorf("release asset %s not found", name)
		}
		if err := m.downloadAsset(ctx, asset, filepath.Join(m.cfg.Win64Dir(), name)); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) copyInstalledFiles(root string) error {
	for _, name := range []string{"PalDefender.dll", "d3d9.dll"} {
		path, err := findFile(root, name)
		if err != nil {
			return err
		}
		if err := copyFile(path, filepath.Join(m.cfg.Win64Dir(), name)); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) downloadAsset(ctx context.Context, asset Asset, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download %s returned status %d", asset.Name, resp.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return err
	}
	if err := verifyDigest(asset, buf.Bytes()); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}

func verifyDigest(asset Asset, b []byte) error {
	if asset.Digest == "" {
		return nil
	}
	want, ok := strings.CutPrefix(asset.Digest, "sha256:")
	if !ok {
		return nil
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s sha256 mismatch", asset.Name)
	}
	return nil
}

func (m Manager) backupCurrent() error {
	dir := filepath.Join(m.cfg.BackupsDir, "paldefender-"+time.Now().UTC().Format("20060102T150405Z"))
	for _, rel := range []string{
		"PalDefender.dll",
		"d3d9.dll",
		filepath.Join("PalDefender", "Config.json"),
		filepath.Join("PalDefender", "RESTAPI", "RESTConfig.json"),
	} {
		src := filepath.Join(m.cfg.Win64Dir(), rel)
		if fileExists(src) {
			if err := copyFile(src, filepath.Join(dir, rel)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m Manager) latestBackup() (string, error) {
	entries, err := os.ReadDir(m.cfg.BackupsDir)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "paldefender-") {
			dirs = append(dirs, filepath.Join(m.cfg.BackupsDir, entry.Name()))
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no PalDefender backup found")
	}
	sort.Strings(dirs)
	return dirs[len(dirs)-1], nil
}

func (m Manager) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s returned status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (m Manager) update(jobID, status string, progress int, message, errText string) {
	_ = m.store.UpdateJob(context.Background(), jobID, status, progress, message, errText)
}

func (m Manager) configPath() string {
	return filepath.Join(m.cfg.PalDefenderDir(), "Config.json")
}

func (m Manager) restConfigPath() string {
	return filepath.Join(m.cfg.PalDefenderDir(), "RESTAPI", "RESTConfig.json")
}

func (m Manager) tokensDir() string {
	return filepath.Join(m.cfg.PalDefenderDir(), "RESTAPI", "Tokens")
}

func (m Manager) restEnabled() bool {
	b, err := os.ReadFile(m.restConfigPath())
	if err != nil {
		return false
	}
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		return false
	}
	enabled, _ := cfg["Enabled"].(bool)
	return enabled
}

func (m Manager) enableRESTConfig() error {
	path := m.restConfigPath()
	restPort := m.cfg.EffectivePalDefenderRESTPort()
	cfg := map[string]any{"Enabled": true, "Port": restPort}
	if fileExists(path) {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_ = json.Unmarshal(b, &cfg)
		cfg["Enabled"] = true
		if _, ok := cfg["Port"]; !ok {
			cfg["Port"] = restPort
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func findAsset(assets []Asset, name string) Asset {
	for _, asset := range assets {
		if strings.EqualFold(asset.Name, name) {
			return asset
		}
	}
	return Asset{}
}

func findFile(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), name) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found", name)
	}
	return found, nil
}

func extractZipSafe(zipPath, dst string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	dst, err = filepath.Abs(dst)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		target := filepath.Join(dst, file.Name)
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if targetAbs != dst && !strings.HasPrefix(targetAbs, dst+string(os.PathSeparator)) {
			return fmt.Errorf("zip contains unsafe path: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func randomToken() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func safeName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "AdminPanel"
	}
	return b.String()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
