//go:build cgo

package sav

import "github.com/oriath-net/gooz"

func decompressOodle(in []byte, out []byte) error {
	_, err := gooz.Decompress(in, out)
	return err
}

// OodleAvailable reports whether this build can decompress Oodle-compressed
// (PlM) Palworld saves. The cgo build links the real decompressor.
func OodleAvailable() bool {
	return true
}
