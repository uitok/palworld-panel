package webui

import (
	"io/fs"
	"os"
	"strings"
)

// Load returns release-embedded assets when available, otherwise it falls
// back to the configured development directory.
func Load(externalDirectory string) (fs.FS, bool) {
	var external fs.FS
	if strings.TrimSpace(externalDirectory) != "" {
		external = os.DirFS(externalDirectory)
	}
	return Select(embeddedAssets(), external)
}

// Select exposes the deterministic priority rule for focused filesystem tests.
func Select(preferred, fallback fs.FS) (fs.FS, bool) {
	if Ready(preferred) {
		return preferred, true
	}
	if Ready(fallback) {
		return fallback, true
	}
	return nil, false
}

func Ready(files fs.FS) bool {
	if files == nil {
		return false
	}
	index, err := fs.Stat(files, "index.html")
	if err != nil || index.IsDir() {
		return false
	}
	assets, err := fs.Stat(files, "assets")
	return err == nil && assets.IsDir()
}
