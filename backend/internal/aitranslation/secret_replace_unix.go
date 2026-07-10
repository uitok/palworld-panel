//go:build !windows

package aitranslation

import "os"

func replaceFileAtomic(source, target string) error {
	return os.Rename(source, target)
}
