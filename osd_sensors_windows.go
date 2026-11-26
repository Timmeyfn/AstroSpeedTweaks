//go:build windows

package main

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Extra readings for the overlay, all from Windows' own performance counters (no driver, nothing
// that touches a game): CPU temperature where the PC reports one, and video memory for any GPU.

type pdhMulti struct {
	q, c uintptr
	ok   bool
	once sync.Once
}

func (p *pdhMulti) init(path string) bool {
	p.once.Do(func() {
		if procPdhOpenQueryW.Find() != nil {
			return
		}
		var q, c uintptr
		if r, _, _ := procPdhOpenQueryW.Call(0, 0, uintptr(unsafe.Pointer(&q))); r != 0 {
			return
		}
		pp, _ := windows.UTF16PtrFromString(path)
		if r, _, _ := procPdhAddEnglishCounterW.Call(q, uintptr(unsafe.Pointer(pp)), 0, uintptr(unsafe.Pointer(&c))); r != 0 {
			return
		}
		procPdhCollectQueryData.Call(q)
		p.q, p.c, p.ok = q, c, true
	})
	return p.ok
}

// values returns every instance's current value.
func (p *pdhMulti) values() []float64 {
	if r, _, _ := procPdhCollectQueryData.Call(p.q); r != 0 {
		return nil
	}
	var size, count uint32
	r, _, _ := procPdhGetFormattedCounterArray.Call(p.c, pdhFmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if uint32(r) != pdhMoreData || size == 0 {
		return nil
	}
	buf := make([]byte, size)
	r, _, _ = procPdhGetFormattedCounterArray.Call(p.c, pdhFmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 || count == 0 {
		return nil
	}
	items := unsafe.Slice((*pdhFmtItem)(unsafe.Pointer(&buf[0])), count)
	out := make([]float64, 0, count)
	for _, it := range items {
		if it.CStatus == pdhCstatusOK || it.CStatus == pdhCstatusNew {
			out = append(out, it.Value)
		}
	}
	return out
}

var (
	osdMu      sync.Mutex
	thermalHP  pdhMulti // tenths of a kelvin
	thermalK   pdhMulti // kelvin
	vramPdh    pdhMulti // bytes, per adapter
	vramTotPdh pdhMulti
)

// cpuTempC returns the hottest thermal zone Windows reports, or -1. Many desktops don't report a
// real CPU sensor here (they show a fixed ~28°C), so anything that looks fake is hidden.
func cpuTempC() float64 {
	defer func() { _ = recover() }()
	osdMu.Lock()
	defer osdMu.Unlock()
	best := -1.0
	if thermalHP.init(`\Thermal Zone Information(*)\High Precision Temperature`) {
		for _, v := range thermalHP.values() {
			if c := v/10 - 273.15; c > best {
				best = c
			}
		}
	}
	if best < 0 && thermalK.init(`\Thermal Zone Information(*)\Temperature`) {
		for _, v := range thermalK.values() {
			if c := v - 273.15; c > best {
				best = c
			}
		}
	}
	if !plausibleCPUTemp(best) {
		return -1
	}
	return best
}

// vramGB returns dedicated video memory in use (and total, if known) for any GPU brand.
func vramGB() (used, total float64) {
	defer func() { _ = recover() }()
	osdMu.Lock()
	defer osdMu.Unlock()
	used, total = -1, -1
	if vramPdh.init(`\GPU Adapter Memory(*)\Dedicated Usage`) {
		max := -1.0
		for _, v := range vramPdh.values() {
			if v > max {
				max = v
			}
		}
		if max >= 0 {
			used = max / (1 << 30)
		}
	}
	return used, total
}
