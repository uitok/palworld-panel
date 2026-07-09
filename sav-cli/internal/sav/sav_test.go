package sav

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"testing"
)

func TestDecodePlZ1(t *testing.T) {
	gvas := append([]byte("GVAS"), bytes.Repeat([]byte{1}, 32)...)
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(gvas); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	header := make([]byte, 12)
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(gvas)))
	binary.LittleEndian.PutUint32(header[4:8], uint32(compressed.Len()))
	copy(header[8:11], []byte("PlZ"))
	header[11] = '1'

	got, info, err := DecodeToGVAS(append(header, compressed.Bytes()...))
	if err != nil {
		t.Fatal(err)
	}
	if info.Magic != "PlZ" || info.SaveType != '1' {
		t.Fatalf("unexpected info: %+v", info)
	}
	if !bytes.Equal(got, gvas) {
		t.Fatalf("decoded payload mismatch")
	}
}

func TestDecodePlMReturnsIncompatible(t *testing.T) {
	data := make([]byte, 24)
	binary.LittleEndian.PutUint32(data[0:4], 100)
	binary.LittleEndian.PutUint32(data[4:8], 12)
	copy(data[8:12], []byte("PlM1"))
	copy(data[20:24], []byte("GVAS"))
	_, _, err := DecodeToGVAS(data)
	var incompatible *IncompatibleError
	if !errors.As(err, &incompatible) {
		t.Fatalf("expected IncompatibleError, got %T %v", err, err)
	}
}
