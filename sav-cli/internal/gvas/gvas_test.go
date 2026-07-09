package gvas

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadMinimalGVAS(t *testing.T) {
	var b bytes.Buffer
	u32(&b, 0x53415647)
	i32(&b, 3)
	i32(&b, 0x20a)
	i32(&b, 0x3f0)
	u16(&b, 5)
	u16(&b, 1)
	u16(&b, 1)
	u32(&b, 0)
	fstring(&b, "++UE5+Release-5.1")
	i32(&b, 3)
	u32(&b, 0)
	fstring(&b, "/Script/Pal.PalWorldSaveGame")
	fstring(&b, "None")
	u32(&b, 0)

	file, err := Read(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if file.Header.SaveGameVersion != 3 {
		t.Fatalf("unexpected save game version: %d", file.Header.SaveGameVersion)
	}
	if len(file.Properties) != 0 {
		t.Fatalf("expected no properties, got %d", len(file.Properties))
	}
}

func i32(b *bytes.Buffer, v int32)  { binary.Write(b, binary.LittleEndian, v) }
func u16(b *bytes.Buffer, v uint16) { binary.Write(b, binary.LittleEndian, v) }
func u32(b *bytes.Buffer, v uint32) { binary.Write(b, binary.LittleEndian, v) }

func fstring(b *bytes.Buffer, s string) {
	i32(b, int32(len(s)+1))
	b.WriteString(s)
	b.WriteByte(0)
}
