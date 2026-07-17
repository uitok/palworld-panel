package sav

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

type Info struct {
	UncompressedLen int    `json:"uncompressed_len"`
	CompressedLen   int    `json:"compressed_len"`
	Magic           string `json:"magic"`
	SaveType        byte   `json:"save_type"`
	DataOffset      int    `json:"data_offset"`
	GVASOffset      int    `json:"gvas_offset,omitempty"`
}

type IncompatibleError struct {
	Message string
}

func (e *IncompatibleError) Error() string {
	return e.Message
}

func incompatible(format string, args ...any) *IncompatibleError {
	return &IncompatibleError{Message: fmt.Sprintf(format, args...)}
}

func Inspect(data []byte) (Info, error) {
	info, _, err := parseHeader(data)
	if idx := bytes.Index(data, []byte("GVAS")); idx >= 0 {
		info.GVASOffset = idx
	}
	if err == nil && info.Magic != "PlZ" && info.Magic != "PlM" {
		err = incompatible("unsupported or invalid Palworld .sav header")
	}
	return info, err
}

func DecodeToGVAS(data []byte) ([]byte, Info, error) {
	info, payload, err := parseHeader(data)
	if err != nil {
		return nil, info, err
	}

	switch info.Magic {
	case "PlZ":
		out, err := inflate(payload)
		if err != nil {
			return nil, info, incompatible("PlZ zlib decompression failed: %v", err)
		}
		if info.SaveType == '2' {
			if info.CompressedLen != len(out) {
				return nil, info, incompatible("PlZ2 inner compressed length mismatch: header=%d actual=%d", info.CompressedLen, len(out))
			}
			out, err = inflate(out)
			if err != nil {
				return nil, info, incompatible("PlZ double zlib decompression failed: %v", err)
			}
		}
		if info.UncompressedLen != len(out) {
			return nil, info, incompatible("uncompressed length mismatch: header=%d actual=%d", info.UncompressedLen, len(out))
		}
		if !bytes.HasPrefix(out, []byte("GVAS")) {
			return nil, info, incompatible("decompressed payload does not start with GVAS")
		}
		return out, info, nil
	case "PlM":
		if info.SaveType != '1' {
			return nil, info, incompatible("unsupported PlM save type %q", info.SaveType)
		}
		if info.CompressedLen != len(payload) {
			return nil, info, incompatible("PlM compressed length mismatch: header=%d actual=%d", info.CompressedLen, len(payload))
		}
		out := make([]byte, info.UncompressedLen)
		if err := decompressOodle(payload, out); err != nil {
			gvasOffset := bytes.Index(data, []byte("GVAS"))
			if gvasOffset >= 0 {
				info.GVASOffset = gvasOffset
				return nil, info, incompatible("PlM Oodle decompression failed: %v; observed inline GVAS marker at offset %d", err, gvasOffset)
			}
			return nil, info, incompatible("PlM Oodle decompression failed: %v", err)
		}
		if !bytes.HasPrefix(out, []byte("GVAS")) {
			gvasOffset := bytes.Index(out, []byte("GVAS"))
			if gvasOffset >= 0 {
				info.GVASOffset = gvasOffset
				return nil, info, incompatible("PlM decompressed payload has GVAS marker at offset %d instead of start", gvasOffset)
			}
			return nil, info, incompatible("PlM decompressed payload does not start with GVAS")
		}
		return out, info, nil
	default:
		return nil, info, incompatible("unsupported save magic %q", info.Magic)
	}
}

func parseHeader(data []byte) (Info, []byte, error) {
	var info Info
	if len(data) < 12 {
		return info, nil, incompatible("file is too small for a Palworld .sav header")
	}
	info.UncompressedLen = int(binary.LittleEndian.Uint32(data[0:4]))
	info.CompressedLen = int(binary.LittleEndian.Uint32(data[4:8]))
	info.Magic = string(data[8:11])
	info.SaveType = data[11]
	info.DataOffset = 12

	if info.Magic == "CNK" {
		if len(data) < 24 {
			return info, nil, incompatible("CNK wrapped save is too small")
		}
		info.UncompressedLen = int(binary.LittleEndian.Uint32(data[12:16]))
		info.CompressedLen = int(binary.LittleEndian.Uint32(data[16:20]))
		info.Magic = string(data[20:23])
		info.SaveType = data[23]
		info.DataOffset = 24
	}

	if info.Magic == "PlZ" {
		if info.SaveType != '1' && info.SaveType != '2' {
			return info, nil, incompatible("unsupported PlZ save type %q", info.SaveType)
		}
		if info.SaveType == '1' && info.CompressedLen != len(data)-info.DataOffset {
			return info, nil, incompatible("compressed length mismatch: header=%d actual=%d", info.CompressedLen, len(data)-info.DataOffset)
		}
	}

	return info, data[info.DataOffset:], nil
}

func inflate(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
