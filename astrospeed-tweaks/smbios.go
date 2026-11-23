package main

import "encoding/binary"

// memInfo summarises the installed RAM sticks from the SMBIOS table (type 17 records).
type memInfo struct {
	Sticks     int
	Speed      int // rated MT/s (highest stick)
	Configured int // MT/s it's actually running at
	Type       string
}

// parseSMBIOSMemory reads a raw SMBIOS table (as returned by GetSystemFirmwareTable 'RSMB',
// including its 8-byte header) without needing WMI or PowerShell.
func parseSMBIOSMemory(raw []byte) memInfo {
	var m memInfo
	if len(raw) < 8 {
		return m
	}
	length := int(binary.LittleEndian.Uint32(raw[4:8]))
	data := raw[8:]
	if length < len(data) {
		data = data[:length]
	}
	for i := 0; i+4 <= len(data); {
		typ := data[i]
		hl := int(data[i+1])
		if hl < 4 || i+hl > len(data) {
			break
		}
		rec := data[i : i+hl]
		if typ == 17 && hl >= 0x15 {
			size := binary.LittleEndian.Uint16(rec[0x0C:])
			if size != 0 && size != 0xFFFF {
				m.Sticks++
				if hl > 0x12 {
					switch rec[0x12] {
					case 0x18:
						m.Type = "DDR3"
					case 0x1A:
						m.Type = "DDR4"
					case 0x22, 0x23:
						m.Type = "DDR5"
					case 0x1E:
						m.Type = "LPDDR4"
					}
				}
				if hl >= 0x17 {
					sp := int(binary.LittleEndian.Uint16(rec[0x15:]))
					if sp == 0xFFFF && hl >= 0x58 {
						sp = int(binary.LittleEndian.Uint32(rec[0x54:]))
					}
					if sp > m.Speed && sp < 20000 {
						m.Speed = sp
					}
				}
				if hl >= 0x22 {
					cs := int(binary.LittleEndian.Uint16(rec[0x20:]))
					if cs == 0xFFFF && hl >= 0x5C {
						cs = int(binary.LittleEndian.Uint32(rec[0x58:]))
					}
					if cs > m.Configured && cs < 20000 {
						m.Configured = cs
					}
				}
			}
		}
		if typ == 127 {
			break
		}
		// skip the string set that follows the formatted area (ends with two NULs)
		j := i + hl
		for j+1 < len(data) && !(data[j] == 0 && data[j+1] == 0) {
			j++
		}
		i = j + 2
	}
	return m
}

// xmpVerdict turns memory speeds into the dashboard's memory-speed check.
func xmpVerdict(m memInfo) string {
	if m.Configured <= 0 {
		return "unknown"
	}
	if (m.Type == "DDR4" && m.Configured <= 2666) || (m.Type == "DDR5" && m.Configured <= 4800) {
		return "likely_off"
	}
	if m.Speed > m.Configured {
		return "below_rated"
	}
	return "ok"
}
