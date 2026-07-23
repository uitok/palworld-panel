//go:build !cgo

package sav

import "fmt"

func decompressOodle(_ []byte, _ []byte) error {
	return fmt.Errorf("Oodle decompression is unavailable in this no-cgo build")
}

// OodleAvailable reports whether this build can decompress Oodle-compressed
// (PlM) Palworld saves. The no-cgo build cannot, so PlM saves fail to parse.
func OodleAvailable() bool {
	return false
}
