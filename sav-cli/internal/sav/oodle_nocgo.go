//go:build !cgo

package sav

import "fmt"

func decompressOodle(_ []byte, _ []byte) error {
	return fmt.Errorf("Oodle decompression is unavailable in this no-cgo build")
}
