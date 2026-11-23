package main

import (
	"encoding/binary"
	"testing"
)

func smbiosRecord(typ byte, length int, fill func([]byte), strs ...string) []byte {
	r := make([]byte, length)
	r[0], r[1] = typ, byte(length)
	if fill != nil {
		fill(r)
	}
	for _, s := range strs {
		r = append(r, []byte(s)...)
		r = append(r, 0)
	}
	if len(strs) == 0 {
		r = append(r, 0)
	}
	return append(r, 0)
}

func TestParseSMBIOSMemory(t *testing.T) {
	stick := func(speed, conf uint16) []byte {
		return smbiosRecord(17, 0x28, func(r []byte) {
			binary.LittleEndian.PutUint16(r[0x0C:], 16384)
			r[0x12] = 0x22 // DDR5
			binary.LittleEndian.PutUint16(r[0x15:], speed)
			binary.LittleEndian.PutUint16(r[0x20:], conf)
		}, "DIMM_A1", "Kingston")
	}
	empty := smbiosRecord(17, 0x28, nil, "DIMM_B1")
	bios := smbiosRecord(0, 0x18, nil, "American Megatrends", "F30")
	var table []byte
	table = append(table, bios...)
	table = append(table, stick(6000, 4800)...)
	table = append(table, empty...)
	table = append(table, stick(6000, 4800)...)
	table = append(table, smbiosRecord(127, 4, nil)...)
	raw := make([]byte, 8)
	binary.LittleEndian.PutUint32(raw[4:], uint32(len(table)))
	raw = append(raw, table...)

	m := parseSMBIOSMemory(raw)
	if m.Sticks != 2 || m.Speed != 6000 || m.Configured != 4800 || m.Type != "DDR5" {
		t.Fatalf("got %+v", m)
	}
	if xmpVerdict(m) != "likely_off" {
		t.Fatalf("verdict %s", xmpVerdict(m))
	}
	if xmpVerdict(memInfo{Type: "DDR5", Speed: 6000, Configured: 6000}) != "ok" {
		t.Fatal("ok verdict")
	}
	// truncated / garbage input must never panic
	for n := 0; n < len(raw); n += 7 {
		_ = parseSMBIOSMemory(raw[:n])
	}
	_ = parseSMBIOSMemory([]byte{0, 0, 0, 0, 255, 255, 255, 255, 17, 2})
}
