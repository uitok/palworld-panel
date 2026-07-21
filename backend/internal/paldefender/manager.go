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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/id"
	"palpanel/internal/jobs"
)

const (
	releasesURL     = "https://api.github.com/repos/Ultimeit/PalDefender/releases"
	latestURL       = "https://api.github.com/repos/Ultimeit/PalDefender/releases/latest"
	kvVersion       = "paldefender_version"
	kvRESTToken     = "paldefender_rest_token"
	kvRuntimeMode   = "runtime_mode"
	panelRESTOrigin = "http://palpanel.local"

	runtimeWineDocker      = "wine_docker"
	runtimeWindowsSteamCMD = "windows_steamcmd"
	runtimeLinuxSteamCMD   = "linux_steamcmd"
)

var panelRESTPermissions = []string{
	"REST.Version.Read",
	"REST.Players.Read",
	"REST.Pals.Read",
	"REST.Pals.Give",
	"REST.PalTemplates.Give",
	"REST.Items.Read",
	"REST.Items.Give",
	"REST.Techs.Read",
	"REST.Techs.Learn",
	"REST.Techs.Forget",
	"REST.Progression.Read",
	"REST.Progression.Give",
	"REST.Messages.Send.PlayerChat",
	"REST.Messages.Send.GlobalChat",
	"REST.Messages.Send.GuildChat",
	"REST.Messages.Send.Log.Normal",
	"REST.Messages.Send.Log.Important",
	"REST.Messages.Send.Log.VeryImportant",
	"REST.Messages.Broadcast",
	"REST.Messages.Alert",
	"REST.Punishments.Kick",
	"REST.Punishments.Ban",
	"REST.Punishments.Unban",
	"REST.Reload.Config",
}

func PanelRESTPermissions() []string {
	return append([]string(nil), panelRESTPermissions...)
}

type Manager struct {
	cfg         appconfig.Config
	store       *db.Store
	client      *http.Client
	restClient  *http.Client
	restBaseURL string
	jobs        *jobs.Executor
	ue4ss       *dependencyTracker
	serverState ServerState
}

type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	PublishedAt string  `json:"published_at"`
	Draft       bool    `json:"draft"`
	Prerelease  bool    `json:"prerelease"`
	Assets      []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest,omitempty"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Status struct {
	Installed       bool                  `json:"installed"`
	Version         string                `json:"version,omitempty"`
	ReleaseSource   string                `json:"release_source"`
	NeedsFirstStart bool                  `json:"needs_first_start"`
	Files           map[string]bool       `json:"files"`
	Paths           map[string]string     `json:"paths"`
	RESTAPIEnabled  bool                  `json:"rest_api_enabled"`
	Warnings        []string              `json:"warnings"`
	UE4SS           UE4SSDependencyStatus `json:"ue4ss"`
	LoadVerified    bool                  `json:"load_verified"`
	LoadEvidence    string                `json:"load_evidence,omitempty"`
}

type TokenResult struct {
	Name        string   `json:"name"`
	Token       string   `json:"token"`
	Permissions []string `json:"permissions"`
	Path        string   `json:"path"`
}

func NewManager(cfg appconfig.Config, store *db.Store, executors ...*jobs.Executor) Manager {
	executor := jobs.New(store, 4)
	if len(executors) > 0 && executors[0] != nil {
		executor = executors[0]
	}
	return Manager{cfg: cfg, store: store, client: &http.Client{Timeout: 60 * time.Second}, restClient: newRESTHTTPClient(), restBaseURL: cfg.EffectivePalDefenderRESTBaseURL(), jobs: executor, ue4ss: newDependencyTracker()}
}

func (m Manager) Releases(ctx context.Context) ([]Release, error) {
	var releases []Release
	if err := m.getJSON(ctx, releasesURL, &releases); err != nil {
		return nil, err
	}
	stable := releases[:0]
	for _, release := range releases {
		if !release.Draft && !release.Prerelease {
			stable = append(stable, release)
		}
	}
	releases = stable
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
	if m.isLinuxRuntime() {
		warnings = append(warnings, "Native Linux mode supports UE4SS Lua mods and Linux .so plugins; PalDefender Windows DLLs are unavailable.")
	}
	if installed && !dirExists(m.cfg.PalDefenderDir()) {
		warnings = append(warnings, "PalDefender directory has not been generated yet; start the server once")
	}
	ue4ssStatus := m.UE4SSStatus()
	if !ue4ssStatus.Compatible {
		warnings = append(warnings, ue4ssStatus.Message)
	}
	loadVerified, loadEvidence := m.palDefenderLoadEvidence()
	if !loadVerified && installed {
		warnings = append(warnings, "PalDefender files are installed but startup-log loading has not been verified yet")
	}
	return Status{
		Installed:       installed,
		Version:         version,
		ReleaseSource:   "github_latest",
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
		UE4SS:          ue4ssStatus,
		LoadVerified:   loadVerified,
		LoadEvidence:   loadEvidence,
	}, nil
}

func (m Manager) palDefenderLoadEvidence() (bool, string) {
	type logCandidate struct {
		path    string
		modTime time.Time
	}
	logDir := filepath.Join(m.cfg.PalDefenderDir(), "Logs")
	entries, err := os.ReadDir(logDir)
	if err == nil {
		candidates := make([]logCandidate, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".log") {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil || !info.Mode().IsRegular() {
				continue
			}
			candidates = append(candidates, logCandidate{path: filepath.Join(logDir, entry.Name()), modTime: info.ModTime()})
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].modTime.After(candidates[j].modTime) })
		if len(candidates) > 20 {
			candidates = candidates[:20]
		}
		for _, candidate := range candidates {
			if logContainsMarkers(candidate.path, "starting paldefender anti cheat") {
				return true, "paldefender_log"
			}
		}
	}
	if logContainsMarkers(m.cfg.ServerLogPath(), "paldefender", "load") || logContainsMarkers(m.cfg.ServerLogPath(), "paldefender", "version") {
		return true, "palserver_log"
	}
	return false, ""
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
		permissions = PanelRESTPermissions()
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
	if err := m.enableRESTConfig(ctx); err != nil {
		return TokenResult{}, err
	}
	_ = m.store.SetKV(ctx, kvRESTToken, token)
	return TokenResult{Name: name, Token: token, Permissions: permissions, Path: path}, nil
}

func (m Manager) ReloadConfig(ctx context.Context) error {
	paths := []string{"/v1/pdapi/ReloadConfig", "/ReloadConfig"}
	var lastErr error
	for _, path := range paths {
		if err := m.doREST(ctx, http.MethodPost, path, nil, nil); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func BalancedPreset() map[string]any {
	return map[string]any{
		"version":                      "1.0.0",
		"MOTD":                         []string{"Welcome to {ServerName}", "PvP: {IsPvP} | Death penalty: {DeathPenalty}"},
		"exitServerOnStartupFailure":   false,
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
	if m.isLinuxRuntime() {
		return db.Job{}, fmt.Errorf("PalDefender is a Windows DLL and cannot be installed into the native Linux server; install native Linux UE4SS instead")
	}
	return m.jobs.Submit(ctx, jobs.ClassLifecycle, typ, message, func(jobCtx context.Context, jobID string) {
		m.update(jobID, "running", 5, "checking Palworld state and UE4SS dependency", "")
		if _, err := m.ensureUE4SS(jobCtx); err != nil {
			status := m.UE4SSStatus()
			m.update(jobID, "failed", 5, "UE4SS dependency stage failed: "+status.State, err.Error())
			return
		}
		m.update(jobID, "running", 35, "UE4SS verified; fetching latest PalDefender release", "")
		release, err := m.Latest(jobCtx)
		if err != nil {
			m.update(jobID, "failed", 35, "PalDefender release lookup failed", err.Error())
			return
		}
		installedVersion := releaseVersion(release)
		if installedVersion == "" {
			m.update(jobID, "failed", 35, "PalDefender release lookup failed", "latest GitHub release has no tag_name")
			return
		}
		m.update(jobID, "running", 55, "downloading and transactionally installing PalDefender assets", "")
		if err := m.installRelease(jobCtx, release); err != nil {
			m.update(jobID, "failed", 55, "PalDefender install stage failed", err.Error())
			return
		}
		if err := m.store.SetKV(jobCtx, kvVersion, installedVersion); err != nil {
			m.update(jobID, "failed", 90, "installed version persistence failed", err.Error())
			return
		}
		m.update(jobID, "completed", 100, "PalDefender "+installedVersion+" installed", "")
	})
}

func (m Manager) installRelease(ctx context.Context, release Release) error {
	for _, path := range []string{m.cfg.ToolsDir, m.cfg.Win64Dir(), m.cfg.BackupsDir} {
		if err := m.cfg.ValidateManagedPath(path, false); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(m.cfg.ToolsDir, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(m.cfg.ToolsDir, "paldefender-stage-")
	if err != nil {
		return err
	}
	defer func() { _ = m.removeManagedDirectory(stage) }()
	loaderAsset := findAsset(release.Assets, "d3d9.dll")
	palDefenderAsset := findAsset(release.Assets, "PalDefender.dll")
	var loaderPath, palDefenderPath string
	if loaderAsset.BrowserDownloadURL != "" && palDefenderAsset.BrowserDownloadURL != "" {
		loaderPath = filepath.Join(stage, "d3d9.dll")
		if err := m.downloadAsset(ctx, loaderAsset, loaderPath); err != nil {
			return err
		}
		palDefenderPath = filepath.Join(stage, "PalDefender.dll")
		if err := m.downloadAsset(ctx, palDefenderAsset, palDefenderPath); err != nil {
			return err
		}
	} else {
		zipAsset := findAsset(release.Assets, "PalDefender.zip")
		if zipAsset.BrowserDownloadURL == "" {
			return fmt.Errorf("latest release must provide d3d9.dll and PalDefender.dll, or a PalDefender.zip fallback")
		}
		zipPath := filepath.Join(stage, "PalDefender.zip")
		if err := m.downloadAsset(ctx, zipAsset, zipPath); err != nil {
			return err
		}
		extracted := filepath.Join(stage, "extracted")
		validate := func(path string) error { return m.cfg.ValidateManagedPath(path, false) }
		if err := extractZipSafeValidated(zipPath, extracted, 256<<20, validate); err != nil {
			return err
		}
		loaderPath, err = findFile(extracted, "d3d9.dll")
		if err != nil {
			return err
		}
		palDefenderPath, err = findFile(extracted, "PalDefender.dll")
		if err != nil {
			return err
		}
	}
	backupDir := filepath.Join(m.cfg.BackupsDir, "paldefender-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+id.New("backup"))
	mutations := make([]ue4ssMutation, 0, 2)
	for _, item := range []struct {
		source string
		name   string
	}{
		{source: loaderPath, name: "d3d9.dll"},
		{source: palDefenderPath, name: "PalDefender.dll"},
	} {
		mutation, err := m.replaceUE4SSFile(item.source, filepath.Join(m.cfg.Win64Dir(), item.name), filepath.Join(backupDir, item.name))
		if err != nil {
			if rollbackErr := rollbackUE4SSMutations(m, mutations); rollbackErr != nil {
				return fmt.Errorf("PalDefender install failed: %v; rollback failed: %v", err, rollbackErr)
			}
			return fmt.Errorf("PalDefender install failed and previous files were restored: %w", err)
		}
		mutations = append(mutations, mutation)
	}
	for _, mutation := range mutations {
		if mutation.oldPath != "" {
			_ = os.Remove(mutation.oldPath)
		}
	}
	return nil
}

func releaseVersion(release Release) string {
	version := strings.TrimSpace(release.TagName)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	return strings.TrimSpace(version)
}

func (m Manager) downloadAsset(ctx context.Context, asset Asset, dst string) error {
	if err := m.cfg.ValidateManagedPath(dst, false); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s returned status %d", asset.Name, resp.StatusCode)
	}
	const maxAssetBytes int64 = 64 << 20
	if resp.ContentLength > maxAssetBytes || asset.Size > maxAssetBytes {
		return fmt.Errorf("download %s exceeds the size limit", asset.Name)
	}
	var buf bytes.Buffer
	written, err := io.Copy(&buf, io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		return err
	}
	if written > maxAssetBytes {
		return fmt.Errorf("download %s exceeds the size limit", asset.Name)
	}
	if err := verifyDigest(asset, buf.Bytes()); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(dst), ".paldefender-download-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	complete := false
	defer func() {
		_ = temporary.Close()
		if !complete {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(buf.Bytes()); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, dst); err != nil {
		return err
	}
	complete = true
	return nil
}

func verifyDigest(asset Asset, b []byte) error {
	if asset.Digest == "" {
		return fmt.Errorf("%s release asset has no SHA-256 digest", asset.Name)
	}
	want, ok := strings.CutPrefix(asset.Digest, "sha256:")
	if !ok {
		return fmt.Errorf("%s release asset uses an unsupported digest", asset.Name)
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
	if err := m.jobs.Update(jobID, status, progress, message, errText); err != nil {
		log.Printf("job %s update failed: %v", jobID, err)
	}
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

func (m Manager) enableRESTConfig(ctx context.Context) error {
	path := m.restConfigPath()
	restPort := m.cfg.EffectivePalDefenderRESTPort()
	address, err := m.restAddress(ctx)
	if err != nil {
		return err
	}
	cfg := map[string]any{"Enabled": true, "Address": address, "Port": restPort}
	if fileExists(path) {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_ = json.Unmarshal(b, &cfg)
		cfg["Enabled"] = true
		cfg["Address"] = address
		if _, ok := cfg["Port"]; !ok {
			cfg["Port"] = restPort
		}
	}
	cors, _ := cfg["Cors"].(map[string]any)
	if cors == nil {
		cors = map[string]any{}
	}
	cors["Allowed-Origins"] = appendPanelRESTOrigin(cors["Allowed-Origins"])
	if _, ok := cors["Max-Age"]; !ok {
		cors["Max-Age"] = 86400
	}
	cfg["Cors"] = cors
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func appendPanelRESTOrigin(value any) []string {
	origins := make([]string, 0, 2)
	add := func(origin string) {
		origin = strings.TrimSpace(origin)
		if origin == "" || origin == "*" {
			return
		}
		for _, existing := range origins {
			if existing == origin {
				return
			}
		}
		origins = append(origins, origin)
	}
	switch configured := value.(type) {
	case string:
		add(configured)
	case []string:
		for _, origin := range configured {
			add(origin)
		}
	case []any:
		for _, raw := range configured {
			if origin, ok := raw.(string); ok {
				add(origin)
			}
		}
	}
	add(panelRESTOrigin)
	return origins
}

func (m Manager) restAddress(ctx context.Context) (string, error) {
	mode, _, err := m.store.GetKV(ctx, kvRuntimeMode)
	if err != nil {
		return "", err
	}
	switch strings.TrimSpace(mode) {
	case runtimeWindowsSteamCMD:
		return "127.0.0.1", nil
	case runtimeWineDocker:
		return "0.0.0.0", nil
	default:
		if runtime.GOOS == "windows" {
			return "127.0.0.1", nil
		}
		return "0.0.0.0", nil
	}
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
	return extractZipSafeValidated(zipPath, dst, 1<<30, nil)
}

func extractZipSafeValidated(zipPath, dst string, maxExtractedBytes int64, validate func(string) error) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	dst, err = filepath.Abs(dst)
	if err != nil {
		return err
	}
	if maxExtractedBytes <= 0 {
		return fmt.Errorf("invalid extracted size limit")
	}
	if len(reader.File) > 20_000 {
		return fmt.Errorf("zip contains too many entries")
	}
	if validate != nil {
		if err := validate(dst); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(reader.File))
	var declaredBytes int64
	var extractedBytes int64
	for _, file := range reader.File {
		normalized := strings.ReplaceAll(strings.TrimSuffix(file.Name, "/"), "\\", "/")
		if unsafeArchiveEntryName(normalized) {
			return fmt.Errorf("zip contains unsafe path: %s", file.Name)
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("zip contains duplicate path: %s", file.Name)
		}
		seen[key] = struct{}{}
		if file.UncompressedSize64 > uint64(maxExtractedBytes) || declaredBytes > maxExtractedBytes-int64(file.UncompressedSize64) {
			return fmt.Errorf("zip exceeds the extracted size limit")
		}
		declaredBytes += int64(file.UncompressedSize64)
		mode := file.Mode()
		isDirectory := file.FileInfo().IsDir()
		if mode&os.ModeSymlink != 0 || (!isDirectory && mode.Type() != 0) {
			return fmt.Errorf("zip contains unsupported file type: %s", file.Name)
		}
		target := filepath.Join(dst, filepath.FromSlash(normalized))
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dst, targetAbs)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("zip contains unsafe path: %s", file.Name)
		}
		if validate != nil {
			if err := validate(targetAbs); err != nil {
				return err
			}
		}
		if isDirectory {
			if err := os.MkdirAll(targetAbs, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return err
		}
		if validate != nil {
			if err := validate(targetAbs); err != nil {
				return err
			}
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		permissions := mode.Perm()
		if permissions == 0 {
			permissions = 0o644
		}
		out, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, permissions)
		if err != nil {
			_ = in.Close()
			return err
		}
		remaining := maxExtractedBytes - extractedBytes
		written, copyErr := io.Copy(out, io.LimitReader(in, remaining+1))
		extractedBytes += written
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return copyErr
		}
		if extractedBytes > maxExtractedBytes {
			return fmt.Errorf("zip exceeds the extracted size limit")
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func unsafeArchiveEntryName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return true
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if clean != name || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(filepath.FromSlash(clean)) {
		return true
	}
	for _, component := range strings.Split(clean, "/") {
		if component == "" || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") || strings.ContainsAny(component, `<>"|?*`) {
			return true
		}
		base := component
		if index := strings.IndexByte(base, '.'); index >= 0 {
			base = base[:index]
		}
		base = strings.ToUpper(base)
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CLOCK$" ||
			(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return true
		}
	}
	return false
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
