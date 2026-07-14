//go:build embed_webui

package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:embedded
var embedded embed.FS

func embeddedAssets() fs.FS {
	assets, err := fs.Sub(embedded, "embedded")
	if err != nil {
		return nil
	}
	return assets
}
