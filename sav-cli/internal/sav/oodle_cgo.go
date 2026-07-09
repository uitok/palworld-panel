//go:build cgo

package sav

import "github.com/oriath-net/gooz"

func decompressOodle(in []byte, out []byte) error {
	_, err := gooz.Decompress(in, out)
	return err
}
