//go:build !windows

package networkproxy

import "os"

func replaceFileAtomic(source, target string) error { return os.Rename(source, target) }
