package mods

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/jobs"
	"palpanel/internal/steamcmd"
)

var workshopIDPattern = regexp.MustCompile(`^\d{5,20}$`)

type Manager struct {
	cfg       appconfig.Config
	store     *db.Store
	runner    docker.Runner
	native    nativeWorkshopDownloader
	steamAuth workshopAuthenticator
	workshop  *WorkshopService
	jobs      *jobs.Executor
	imports   *importRegistry
	local     *localActionState
}

func NewManager(cfg appconfig.Config, store *db.Store, runner docker.Runner, executors ...*jobs.Executor) Manager {
	executor := jobs.New(store, 4)
	if len(executors) > 0 && executors[0] != nil {
		executor = executors[0]
	}
	nativeClient := steamcmd.New(cfg)
	return Manager{
		cfg: cfg, store: store, runner: runner, native: nativeClient, steamAuth: nativeClient,
		workshop: NewWorkshopService(cfg), jobs: executor, imports: newImportRegistry(cfg), local: &localActionState{},
	}
}

func (m Manager) List(ctx context.Context) ([]db.Mod, error) {
	return m.store.ListMods(ctx)
}

func (m Manager) UploadZip(ctx context.Context, r io.Reader, filename string, enable bool) (db.Mod, error) {
	record, err := m.imports.newRecord(filename)
	if err != nil {
		return db.Mod{}, err
	}
	defer func() { _ = m.removeManagedDirectory(record.directory) }()
	workDir := record.directory
	zipPath := filepath.Join(workDir, cleanFilename(filename))
	if _, err := writeLimitedFile(r, zipPath, m.imports.maxBytes); err != nil {
		return db.Mod{}, err
	}
	extractDir := filepath.Join(workDir, "extracted")
	if err := extractArchive(zipPath, extractDir); err != nil {
		return db.Mod{}, err
	}
	modRoot, _, err := inspectModDirectory(extractDir)
	if err != nil {
		return db.Mod{}, err
	}
	return m.installPrepared(ctx, modRoot, "upload", "", WorkshopItem{}, enable)
}

func (m Manager) SearchWorkshop(ctx context.Context, params WorkshopSearchParams) (WorkshopSearchResult, error) {
	result, err := m.workshop.Search(ctx, params)
	if err != nil {
		return WorkshopSearchResult{}, err
	}
	return m.mergeWorkshopState(ctx, result)
}

func (m Manager) WorkshopDetail(ctx context.Context, itemID string) (WorkshopItem, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return WorkshopItem{}, fmt.Errorf("workshop item id is required")
	}
	if !workshopIDPattern.MatchString(itemID) {
		return WorkshopItem{}, fmt.Errorf("workshop item id must be numeric")
	}
	item, err := m.workshop.Detail(ctx, itemID)
	if err != nil {
		return WorkshopItem{}, err
	}
	items, err := m.mergeWorkshopItems(ctx, []WorkshopItem{item})
	if err != nil {
		return WorkshopItem{}, err
	}
	return items[0], nil
}

func (m Manager) DownloadWorkshop(ctx context.Context, itemID string, enable bool) (db.Job, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return db.Job{}, fmt.Errorf("workshop item id is required")
	}
	if !workshopIDPattern.MatchString(itemID) {
		return db.Job{}, fmt.Errorf("workshop item id must be numeric")
	}
	if _, err := m.RequireWorkshopLogin(ctx); err != nil {
		return db.Job{}, err
	}
	record, err := m.imports.newRecord(itemID)
	if err != nil {
		return db.Job{}, err
	}
	job, err := m.jobs.Submit(ctx, jobs.ClassLifecycle, "workshop_download", "queued workshop download", func(jobCtx context.Context, jobID string) {
		defer func() { _ = m.removeManagedDirectory(record.directory) }()
		var meta WorkshopItem
		if item, err := m.workshop.Detail(jobCtx, itemID); err == nil {
			meta = item
		}
		m.runWorkshopImport(jobCtx, jobID, itemID, enable, meta, record.directory)
	})
	if err != nil {
		_ = m.removeManagedDirectory(record.directory)
		return db.Job{}, err
	}
	return job, nil
}

func (m Manager) SetEnabled(ctx context.Context, modID string, enabled bool) (db.Mod, error) {
	mod, err := m.store.GetMod(ctx, modID)
	if err != nil {
		return db.Mod{}, err
	}
	if err := m.store.SetModEnabled(ctx, modID, enabled); err != nil {
		return db.Mod{}, err
	}
	if err := m.rewriteActiveMods(ctx); err != nil {
		return db.Mod{}, err
	}
	_ = m.store.SetKV(ctx, "pending_restart", "true")
	mod.Enabled = enabled
	return mod, nil
}

func (m Manager) Delete(ctx context.Context, modID string) error {
	mod, err := m.store.GetMod(ctx, modID)
	if err != nil {
		return err
	}
	target, err := palPanelOwnedModTarget(m.cfg.WorkshopModsDir(), mod.ID, mod.Path)
	if err != nil {
		return err
	}
	if err := m.removeManagedDirectory(target); err != nil {
		return err
	}
	if err := m.store.DeleteMod(ctx, modID); err != nil {
		return err
	}
	if err := m.rewriteActiveMods(ctx); err != nil {
		return err
	}
	_ = m.store.SetKV(ctx, "pending_restart", "true")
	return nil
}

func (m Manager) removeManagedDirectory(path string) error {
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return err
	}
	// Revalidate immediately before the destructive operation so a path changed
	// into a junction or symlink after an earlier check cannot escape the root.
	if err := m.cfg.ValidateManagedPath(path, false); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func (m Manager) mergeWorkshopState(ctx context.Context, result WorkshopSearchResult) (WorkshopSearchResult, error) {
	items, err := m.mergeWorkshopItems(ctx, result.Items)
	if err != nil {
		return WorkshopSearchResult{}, err
	}
	result.Items = items
	return result, nil
}

func (m Manager) mergeWorkshopItems(ctx context.Context, items []WorkshopItem) ([]WorkshopItem, error) {
	local, err := m.store.ListMods(ctx)
	if err != nil {
		return nil, err
	}
	byWorkshopID := map[string]db.Mod{}
	for _, mod := range local {
		workshopID := strings.TrimSpace(mod.WorkshopID)
		if workshopID == "" && mod.Source == "workshop" {
			workshopID = mod.ID
		}
		if workshopID != "" {
			byWorkshopID[workshopID] = mod
		}
	}
	out := make([]WorkshopItem, len(items))
	copy(out, items)
	for i := range out {
		mod, ok := byWorkshopID[out[i].ID]
		if !ok {
			continue
		}
		out[i].Installed = true
		out[i].Enabled = mod.Enabled
		out[i].ModID = mod.ID
		out[i].UpdateAvailable = mod.TimeUpdated > 0 && out[i].TimeUpdated > mod.TimeUpdated
	}
	return out, nil
}

func applyWorkshopMetadata(mod *db.Mod, meta WorkshopItem) {
	if meta.ID != "" {
		mod.WorkshopID = meta.ID
		mod.SteamURL = meta.SteamURL
		if mod.SteamURL == "" {
			mod.SteamURL = steamWorkshopPageURL + meta.ID
		}
		mod.PreviewURL = meta.PreviewURL
		mod.Summary = meta.Summary
		mod.Tags = meta.Tags
		mod.FileSize = meta.FileSize
		mod.Subscriptions = meta.Subscriptions
		mod.TimeUpdated = meta.TimeUpdated
		mod.LastCheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if mod.WorkshopID != "" && mod.SteamURL == "" {
		mod.SteamURL = steamWorkshopPageURL + mod.WorkshopID
	}
}

func (m Manager) rewriteActiveMods(ctx context.Context) error {
	list, err := m.store.ListMods(ctx)
	if err != nil {
		return err
	}
	settings := ModSettings{GlobalEnabled: true}
	for _, mod := range list {
		if mod.Enabled {
			settings = EnablePackage(settings, mod.PackageName, true)
		}
	}
	return WriteModSettings(m.cfg.PalModSettingsPath(), settings)
}

func (m Manager) update(jobID, status string, progress int, message, errText string) {
	if err := m.jobs.Update(jobID, status, progress, message, errText); err != nil {
		log.Printf("job %s update failed: %v", jobID, err)
	}
}

func cleanFilename(name string) string {
	name = filepath.Base(name)
	if name == "." || name == string(os.PathSeparator) || strings.TrimSpace(name) == "" {
		return "upload.zip"
	}
	return name
}
