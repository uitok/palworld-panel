package gvas

import (
	"encoding/binary"
	"fmt"
	"math"
)

type Header struct {
	Magic                 int32     `json:"magic"`
	SaveGameVersion       int32     `json:"save_game_version"`
	PackageFileVersionUE4 int32     `json:"package_file_version_ue4"`
	PackageFileVersionUE5 int32     `json:"package_file_version_ue5"`
	EngineVersionMajor    uint16    `json:"engine_version_major"`
	EngineVersionMinor    uint16    `json:"engine_version_minor"`
	EngineVersionPatch    uint16    `json:"engine_version_patch"`
	EngineVersionChange   uint32    `json:"engine_version_changelist"`
	EngineVersionBranch   string    `json:"engine_version_branch"`
	CustomVersionFormat   int32     `json:"custom_version_format"`
	CustomVersions        []Version `json:"custom_versions"`
	SaveGameClassName     string    `json:"save_game_class_name"`
}

type Version struct {
	GUID    string `json:"guid"`
	Version int32  `json:"version"`
}

type File struct {
	Header     Header         `json:"header"`
	Properties map[string]any `json:"properties"`
	TrailerLen int            `json:"trailer_len"`
}

func Read(data []byte) (*File, error) {
	r := NewReader(data)
	header, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	props, err := r.PropertiesUntilEnd("")
	if err != nil {
		return nil, err
	}
	return &File{
		Header:     header,
		Properties: props,
		TrailerLen: r.Remaining(),
	}, nil
}

func readHeader(r *Reader) (Header, error) {
	var h Header
	magic, err := r.I32()
	if err != nil {
		return h, err
	}
	h.Magic = magic
	if h.Magic != 0x53415647 {
		return h, fmt.Errorf("invalid GVAS magic: 0x%x", uint32(h.Magic))
	}
	if h.SaveGameVersion, err = r.I32(); err != nil {
		return h, err
	}
	if h.SaveGameVersion != 3 {
		return h, fmt.Errorf("expected save game version 3, got %d", h.SaveGameVersion)
	}
	if h.PackageFileVersionUE4, err = r.I32(); err != nil {
		return h, err
	}
	if h.PackageFileVersionUE5, err = r.I32(); err != nil {
		return h, err
	}
	if h.EngineVersionMajor, err = r.U16(); err != nil {
		return h, err
	}
	if h.EngineVersionMinor, err = r.U16(); err != nil {
		return h, err
	}
	if h.EngineVersionPatch, err = r.U16(); err != nil {
		return h, err
	}
	if h.EngineVersionChange, err = r.U32(); err != nil {
		return h, err
	}
	if h.EngineVersionBranch, err = r.FString(); err != nil {
		return h, err
	}
	if h.CustomVersionFormat, err = r.I32(); err != nil {
		return h, err
	}
	if h.CustomVersionFormat != 3 {
		return h, fmt.Errorf("expected custom version format 3, got %d", h.CustomVersionFormat)
	}
	count, err := r.U32()
	if err != nil {
		return h, err
	}
	if count > 100000 {
		return h, fmt.Errorf("custom version count is unreasonable: %d", count)
	}
	h.CustomVersions = make([]Version, 0, count)
	for range count {
		guid, err := r.GUID()
		if err != nil {
			return h, err
		}
		version, err := r.I32()
		if err != nil {
			return h, err
		}
		h.CustomVersions = append(h.CustomVersions, Version{GUID: guid, Version: version})
	}
	if h.SaveGameClassName, err = r.FString(); err != nil {
		return h, err
	}
	return h, nil
}

type Reader struct {
	data []byte
	pos  int
}

func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

func (r *Reader) EOF() bool {
	return r.pos >= len(r.data)
}

func (r *Reader) Remaining() int {
	if r.pos >= len(r.data) {
		return 0
	}
	return len(r.data) - r.pos
}

func (r *Reader) Read(n int) ([]byte, error) {
	if n < 0 || r.pos+n > len(r.data) {
		return nil, fmt.Errorf("read past end at offset %d size %d remaining %d", r.pos, n, r.Remaining())
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *Reader) Skip(n int) error {
	_, err := r.Read(n)
	return err
}

func (r *Reader) Byte() (byte, error) {
	b, err := r.Read(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (r *Reader) Bool() (bool, error) {
	b, err := r.Byte()
	return b != 0, err
}

func (r *Reader) I16() (int16, error) {
	b, err := r.Read(2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(b)), nil
}

func (r *Reader) U16() (uint16, error) {
	b, err := r.Read(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (r *Reader) I32() (int32, error) {
	b, err := r.Read(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(b)), nil
}

func (r *Reader) U32() (uint32, error) {
	b, err := r.Read(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *Reader) I64() (int64, error) {
	b, err := r.Read(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b)), nil
}

func (r *Reader) U64() (uint64, error) {
	b, err := r.Read(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

func (r *Reader) Float() (float64, error) {
	b, err := r.Read(4)
	if err != nil {
		return 0, err
	}
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(b))), nil
}

func (r *Reader) Double() (float64, error) {
	b, err := r.Read(8)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b)), nil
}

func (r *Reader) FString() (string, error) {
	n, err := r.I32()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	if n > 0 {
		if n > 16*1024*1024 {
			return "", fmt.Errorf("ascii fstring is too large: %d", n)
		}
		b, err := r.Read(int(n))
		if err != nil {
			return "", err
		}
		if len(b) == 0 {
			return "", nil
		}
		return string(b[:len(b)-1]), nil
	}
	chars := -n
	if chars > 8*1024*1024 {
		return "", fmt.Errorf("utf16 fstring is too large: %d", chars)
	}
	b, err := r.Read(int(chars) * 2)
	if err != nil {
		return "", err
	}
	if len(b) < 2 {
		return "", nil
	}
	runes := make([]rune, 0, int(chars)-1)
	for i := 0; i+1 < len(b)-2; i += 2 {
		runes = append(runes, rune(binary.LittleEndian.Uint16(b[i:i+2])))
	}
	return string(runes), nil
}

func (r *Reader) GUID() (string, error) {
	b, err := r.Read(16)
	if err != nil {
		return "", err
	}
	return formatGUID(b), nil
}

func (r *Reader) OptionalGUID() (any, error) {
	present, err := r.Byte()
	if err != nil {
		return nil, err
	}
	if present == 0 {
		return nil, nil
	}
	return r.GUID()
}

func (r *Reader) TArray(read func(*Reader) (any, error)) ([]any, error) {
	count, err := r.U32()
	if err != nil {
		return nil, err
	}
	if count > 10_000_000 {
		return nil, fmt.Errorf("array count is unreasonable: %d", count)
	}
	out := make([]any, 0, count)
	for range count {
		v, err := read(r)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func formatGUID(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[3], b[2], b[1], b[0],
		b[7], b[6],
		b[5], b[4],
		b[11], b[10],
		b[9], b[8], b[15], b[14], b[13], b[12],
	)
}
