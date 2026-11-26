//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// The benchmark: live FPS / frame time / CPU / GPU like MSI Afterburner, plus recorded runs
// (average FPS, 1% and 0.1% lows) to compare before and after optimizing.

type benchSample struct {
	FPS      float64    `json:"fps"`
	FrameMs  float64    `json:"frameMs"`
	Low1     float64    `json:"low1"`   // over the last 10 seconds
	Frames   []float64  `json:"frames"` // this second's frame times, for the graph (capped)
	CPU      float64    `json:"cpu"`
	GPU      float64    `json:"gpu"`
	GPUGame  float64    `json:"gpuGame"`
	RAMPct   float64    `json:"ramPct"`
	RAMUsed  float64    `json:"ramUsedGB"`
	Sensors  gpuSensors `json:"sensors"`
	PID      uint32     `json:"pid"`
	Process  string     `json:"process"`
	Game     string     `json:"game"`
	Counting bool       `json:"counting"`
	Error    string     `json:"error,omitempty"`
	Rec      *recState  `json:"rec,omitempty"`
}

type recState struct {
	Label    string    `json:"label"`
	Seconds  int       `json:"seconds"`
	Elapsed  int       `json:"elapsed"`
	frames   []float64 // all frame times
	graph    []float64 // FPS per second
	cpu, gpu []float64
	temps    []float64
	pid      uint32
	process  string
	game     string
}

var (
	benchMu      sync.Mutex
	benchOn      bool
	benchTarget  uint32 // 0 = pick automatically
	benchLast    benchSample
	benchRec     *recState
	benchStopped = make(chan struct{}, 1)
	benchIdle    time.Time
)

func nowFiletime() int64 {
	ft := windows.NsecToFiletime(time.Now().UnixNano())
	return int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
}

func processNames() map[uint32]string {
	m := map[uint32]string{}
	for _, p := range listProcesses() {
		m[p.PID] = p.Name
	}
	return m
}

var notGames = map[string]bool{"dwm.exe": true, "explorer.exe": true, "astro tweaks.exe": true, "astrospeed tweaks.exe": true,
	"msedgewebview2.exe": true, "applicationframehost.exe": true, "searchhost.exe": true, "startmenuexperiencehost.exe": true,
	"textinputhost.exe": true, "shellexperiencehost.exe": true, "lockapp.exe": true, "nvidia overlay.exe": true}

// pickTarget chooses what to measure: a known game if one is running, else whatever shows the most frames.
func pickTarget(names map[uint32]string, presenting map[uint32]float64) uint32 {
	if benchTarget != 0 {
		if _, ok := names[benchTarget]; ok {
			return benchTarget
		}
	}
	for pid := range presenting {
		for _, g := range Games {
			for _, e := range g.Exes {
				if strings.EqualFold(names[pid], e) {
					return pid
				}
			}
		}
	}
	best, bestN := uint32(0), 0.0
	self := uint32(os.Getpid())
	for pid, n := range presenting {
		nm := strings.ToLower(names[pid])
		if pid == self || nm == "" || notGames[nm] {
			continue
		}
		if n > bestN {
			best, bestN = pid, n
		}
	}
	return best
}

func gameNameFor(exe string) string {
	for _, g := range Games {
		for _, e := range g.Exes {
			if strings.EqualFold(exe, e) {
				return g.Name
			}
		}
	}
	return strings.TrimSuffix(exe, filepath.Ext(exe))
}

// benchStart turns the live monitor on. It stops by itself 30 s after the window stops asking.
func benchStart() error {
	if err := startFrameCounter(); err != nil {
		return err
	}
	benchMu.Lock()
	benchIdle = time.Now()
	if benchOn {
		benchMu.Unlock()
		return nil
	}
	benchOn = true
	benchMu.Unlock()
	go benchLoop()
	return nil
}

func benchLoop() {
	defer func() { _ = recover() }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	cpuPercent() // prime
	gpuUsage(0)
	var since int64 = nowFiletime()
	var window []float64 // last 10 s of frame times
	var windowSec []int
	lastPID := uint32(0)
	for range tick.C {
		benchMu.Lock()
		idle := time.Since(benchIdle) > 30*time.Second && benchRec == nil && !overlayOn()
		benchMu.Unlock()
		if idle {
			break
		}
		now := nowFiletime()
		names := processNames()
		presenting := presentingProcesses(now)
		benchMu.Lock()
		pid := pickTarget(names, presenting)
		benchMu.Unlock()
		if pid != lastPID {
			window, windowSec, lastPID = nil, nil, pid
		}
		ms, _ := frameTimesSince(pid, since)
		since = now
		window = append(window, ms...)
		windowSec = append(windowSec, len(ms))
		for len(windowSec) > 10 {
			window = window[windowSec[0]:]
			windowSec = windowSec[1:]
		}
		s := benchSample{PID: pid, Process: names[pid], Counting: true, CPU: cpuPercent()}
		if pid != 0 {
			s.Game = gameNameFor(names[pid])
		}
		if len(ms) > 0 {
			st := frameStats(ms)
			s.FPS, s.FrameMs = float64(len(ms)), st.AvgMs
			if len(ms) <= 400 {
				s.Frames = ms
			} else {
				s.Frames = ms[len(ms)-400:]
			}
		}
		if len(window) > 0 {
			s.Low1 = frameStats(window).Low1
		}
		s.GPU, s.GPUGame = gpuUsage(pid)
		m := memStatus()
		s.RAMPct, s.RAMUsed = float64(m.MemoryLoad), float64(m.TotalPhys-m.AvailPhys)/(1<<30)
		s.Sensors = readGPUSensors()
		framesMu.Lock()
		if etwErr != "" {
			s.Error, s.Counting = etwErr, false
		}
		// forget processes that stopped drawing a minute ago
		for p, fl := range framesBy {
			if len(fl.ts) > 0 && fl.ts[len(fl.ts)-1] < now-600_000_000 {
				delete(framesBy, p)
			}
		}
		framesMu.Unlock()

		benchMu.Lock()
		if r := benchRec; r != nil {
			if r.pid == 0 && pid != 0 {
				r.pid, r.process, r.game = pid, names[pid], s.Game
			}
			if pid == r.pid && r.pid != 0 {
				r.frames = append(r.frames, ms...)
				r.graph = append(r.graph, float64(len(ms)))
				r.cpu = append(r.cpu, s.CPU)
				r.gpu = append(r.gpu, s.GPU)
				if s.Sensors.TempC >= 0 {
					r.temps = append(r.temps, s.Sensors.TempC)
				}
				r.Elapsed++
			}
			cp := *r
			s.Rec = &cp
			if r.Elapsed >= r.Seconds {
				go finishRecording()
			}
		}
		benchLast = s
		benchMu.Unlock()
		overlayUpdate(s)
		emit("benchLive", s)
	}
	benchMu.Lock()
	benchOn = false
	benchMu.Unlock()
	stopFrameCounter()
}

func benchLive() benchSample {
	benchMu.Lock()
	defer benchMu.Unlock()
	benchIdle = time.Now()
	return benchLast
}

func benchSetTarget(pid uint32) {
	benchMu.Lock()
	benchTarget = pid
	benchMu.Unlock()
}

// benchTargets lists processes currently showing frames, for the "what to measure" picker.
func benchTargets() []map[string]interface{} {
	names := processNames()
	pres := presentingProcesses(nowFiletime())
	var out []map[string]interface{}
	for pid, n := range pres {
		nm := names[pid]
		if nm == "" || notGames[strings.ToLower(nm)] || pid == uint32(os.Getpid()) {
			continue
		}
		out = append(out, map[string]interface{}{"pid": pid, "process": nm, "name": gameNameFor(nm), "fps": n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["fps"].(float64) > out[j]["fps"].(float64) })
	return out
}

func benchRecord(seconds int, label string) error {
	if seconds < 10 || seconds > 600 {
		seconds = 60
	}
	label = strings.TrimSpace(label)
	if len(label) > 40 {
		label = label[:40]
	}
	if err := benchStart(); err != nil {
		return err
	}
	benchMu.Lock()
	defer benchMu.Unlock()
	if benchRec != nil {
		return errors.New("a benchmark is already recording")
	}
	benchRec = &recState{Label: label, Seconds: seconds}
	logf("Benchmark started: %d s (%s)", seconds, label)
	return nil
}

func benchCancel() {
	benchMu.Lock()
	benchRec = nil
	benchMu.Unlock()
}

func finishRecording() {
	benchMu.Lock()
	r := benchRec
	benchRec = nil
	benchMu.Unlock()
	if r == nil {
		return
	}
	st := frameStats(r.frames)
	if st.Frames < 30 {
		emit("benchDone", map[string]interface{}{"error": "No frames were seen. Make sure the game is running and in focus, then try again. (Vulkan and OpenGL games can't be measured.)"})
		return
	}
	avg := func(v []float64) float64 {
		n, s := 0, 0.0
		for _, x := range v {
			if x >= 0 {
				s += x
				n++
			}
		}
		if n == 0 {
			return -1
		}
		return float64(int(s/float64(n)*10)) / 10
	}
	res := benchResult{ID: fmt.Sprintf("b%d", time.Now().UnixNano()), When: time.Now().UnixMilli(), Game: r.game, Process: r.process,
		Label: r.Label, Stats: st, CPU: avg(r.cpu), GPU: avg(r.gpu), GPUTemp: avg(r.temps), Graph: r.graph}
	stateMu.Lock()
	appState.Benchmarks = append(appState.Benchmarks, res)
	if len(appState.Benchmarks) > 60 {
		appState.Benchmarks = appState.Benchmarks[len(appState.Benchmarks)-60:]
	}
	saveStateLocked()
	stateMu.Unlock()
	logf("Benchmark %s (%s): avg %.1f FPS, 1%% low %.1f, 0.1%% low %.1f", res.Game, res.Label, st.AvgFPS, st.Low1, st.Low01)
	emit("benchDone", res)
}

func benchList() []benchResult {
	stateMu.Lock()
	defer stateMu.Unlock()
	out := append([]benchResult(nil), appState.Benchmarks...)
	return out
}

func benchDelete(id string) {
	stateMu.Lock()
	defer stateMu.Unlock()
	keep := appState.Benchmarks[:0]
	for _, b := range appState.Benchmarks {
		if b.ID != id {
			keep = append(keep, b)
		}
	}
	appState.Benchmarks = keep
	saveStateLocked()
}

// saveBenchImage saves the share card (a PNG made by the window) to Pictures\Astro Tweaks.
func saveBenchImage(pngData []byte) (string, error) {
	if len(pngData) < 100 || len(pngData) > 8<<20 || string(pngData[1:4]) != "PNG" {
		return "", errors.New("that picture couldn't be saved")
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Pictures", "Astro Tweaks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, "Benchmark "+time.Now().Format("2006-01-02 15-04-05")+".png")
	if err := os.WriteFile(p, pngData, 0o644); err != nil {
		return "", err
	}
	return p, nil
}
