//go:build !embed_webui

package webui

import "io/fs"

func embeddedAssets() fs.FS {
	return nil
}
