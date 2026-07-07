package mods

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"palpanel/internal/appconfig"
	"palpanel/internal/db"
	"palpanel/internal/docker"
	"palpanel/internal/id"
)

var workshopIDPattern = regexp.MustCompile(`^\d{5,20}$`)

type Manager struct {
	cfg    appconfig.Config
	store  *db.Store
	runner docker.Runner
}

func NewManager(cfg appconfig.Config, store *db.Store, runner docker.Runner) Manager {
	return Manager{cfg: cfg, store: store, runner: runner}
}

func (m Manager) List(ctx context.Context) ([]db.Mod, error) {
	return m.store.ListMods(ctx)
}

func (m Manager) UploadZip(ctx context.Context, r io.Reader, filename string, enable bool) (db.Mod, error) {
	modID := id.New("mod")
	workDir := filepath.Join(m.cfg.UploadsDir, modID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return db.Mod{}, err
	}
	zipPath := filepath.Join(workDir, cleanFilename(filename))
	f, err := os.Create(zipPath)
	if err != nil {
		return db.Mod{}, err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return db.Mod{}, err
	}
	if err := f.Close(); err != nil {
		return db.Mod{}, err
	}

	extractDir := filepath.Join(workDir, "extracted")
	if err := extractZip(zipPath, extractDir); err != nil {
		return db.Mod{}, err
	}
	return m.installFromDir(ctx, modID, extractDir, "upload", enable)
}

func (m Manager) DownloadWorkshop(ctx context.Context, itemID string) (db.Job, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return db.Job{}, fmt.Errorf("workshop item id is required")
	}
	if !workshopIDPattern.MatchString(itemID) {
		return db.Job{}, fmt.Errorf("workshop item id must be numeric")
	}
	j, err := m.store.CreateJob(ctx, id.New("job"), "workshop_download", "queued workshop download")
	if err != nil {
		return db.Job{}, err
	}
	go func() {
		m.update(j.ID, "running", 10, "building wine runner image", "")
		if err := m.runner.BuildImage(context.Background()); err != nil {
			m.update(j.ID, "failed", 10, "build failed", err.Error())
			return
		}
		m.update(j.ID, "running", 50, "downloading Steam Workshop item", "")
		if err := m.runner.DownloadWorkshop(context.Background(), itemID); err != nil {
			m.update(j.ID, "failed", 50, "workshop download failed", err.Error())
			return
		}
		m.update(j.ID, "running", 80, "reading mod metadata", "")
		if _, err := m.installFromDir(context.Background(), itemID, filepath.Join(m.cfg.WorkshopModsDir(), itemID), "workshop", false); err != nil {
			m.update(j.ID, "failed", 80, "metadata read failed", err.Error())
			return
		}
		m.update(j.ID, "completed", 100, "workshop download completed", "")
	}()
	return j, nil
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
	if err := os.RemoveAll(mod.Path); err != nil {
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

func (m Manager) installFromDir(ctx context.Context, modID, sourceDir, source string, enable bool) (db.Mod, error) {
	infoPath, err := findInfoJSON(sourceDir)
	if err != nil {
		return db.Mod{}, err
	}
	info, err := ReadInfo(infoPath)
	if err != nil {
		return db.Mod{}, err
	}
	target := filepath.Join(m.cfg.WorkshopModsDir(), modID)
	if filepath.Clean(sourceDir) != filepath.Clean(target) {
		if err := os.RemoveAll(target); err != nil {
			return db.Mod{}, err
		}
		if err := copyDir(filepath.Dir(infoPath), target); err != nil {
			return db.Mod{}, err
		}
	}
	mod := db.Mod{
		ID:          modID,
		Name:        info.Name,
		Source:      source,
		PackageName: info.PackageName,
		Path:        target,
		Version:     info.Version,
		Enabled:     enable,
	}
	if err := m.store.UpsertMod(ctx, mod); err != nil {
		return db.Mod{}, err
	}
	if enable {
		if err := m.rewriteActiveMods(ctx); err != nil {
			return db.Mod{}, err
		}
		_ = m.store.SetKV(ctx, "pending_restart", "true")
	}
	return mod, nil
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
	_ = m.store.UpdateJob(context.Background(), jobID, status, progress, message, errText)
}

func extractZip(zipPath, dst string) error {
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
		src, err := file.Open()
		if err != nil {
			return err
		}
		dstFile, err := os.OpenFile(targetAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = src.Close()
			return err
		}
		_, copyErr := io.Copy(dstFile, src)
		closeErr := dstFile.Close()
		_ = src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func findInfoJSON(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "Info.json") {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("Info.json not found")
	}
	return found, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func cleanFilename(name string) string {
	name = filepath.Base(name)
	if name == "." || name == string(os.PathSeparator) || strings.TrimSpace(name) == "" {
		return "upload.zip"
	}
	return name
}
