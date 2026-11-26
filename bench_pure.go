package main

import (
	"math"
	"sort"
)

// Frame-time statistics, the same way benchmark tools report them:
//   avg FPS      = frames / time
//   1% low FPS   = 1000 / (average of the slowest 1% of frame times)
//   0.1% low FPS = 1000 / (average of the slowest 0.1% of frame times)
// Frame times are in milliseconds.

type benchStats struct {
	Frames   int     `json:"frames"`
	Seconds  float64 `json:"seconds"`
	AvgFPS   float64 `json:"avgFps"`
	Low1     float64 `json:"low1"`
	Low01    float64 `json:"low01"`
	MinFPS   float64 `json:"minFps"`
	MaxFPS   float64 `json:"maxFps"`
	AvgMs    float64 `json:"avgMs"`
	P99Ms    float64 `json:"p99Ms"`
	StutterN int     `json:"stutters"` // frames more than 2.5x the median frame time
}

func frameStats(ms []float64) benchStats {
	var s benchStats
	var clean []float64
	for _, v := range ms {
		if v > 0 && v < 2000 { // ignore pauses (alt-tab, loading screens)
			clean = append(clean, v)
		}
	}
	if len(clean) == 0 {
		return s
	}
	total := 0.0
	for _, v := range clean {
		total += v
	}
	s.Frames = len(clean)
	s.Seconds = total / 1000
	s.AvgMs = total / float64(len(clean))
	s.AvgFPS = 1000 / s.AvgMs
	sorted := append([]float64(nil), clean...)
	sort.Float64s(sorted) // fastest first
	slowAvg := func(frac float64) float64 {
		n := int(math.Ceil(float64(len(sorted)) * frac))
		if n < 1 {
			n = 1
		}
		sum := 0.0
		for _, v := range sorted[len(sorted)-n:] {
			sum += v
		}
		return sum / float64(n)
	}
	s.Low1 = 1000 / slowAvg(0.01)
	s.Low01 = 1000 / slowAvg(0.001)
	s.MaxFPS = 1000 / sorted[0]
	s.MinFPS = 1000 / sorted[len(sorted)-1]
	s.P99Ms = sorted[int(math.Min(float64(len(sorted)-1), math.Floor(float64(len(sorted))*0.99)))]
	med := sorted[len(sorted)/2]
	for _, v := range clean {
		if v > med*2.5 && v > 8 {
			s.StutterN++
		}
	}
	round := func(v float64) float64 { return math.Round(v*10) / 10 }
	s.AvgFPS, s.Low1, s.Low01, s.MinFPS, s.MaxFPS = round(s.AvgFPS), round(s.Low1), round(s.Low01), round(s.MinFPS), round(s.MaxFPS)
	s.AvgMs, s.P99Ms, s.Seconds = math.Round(s.AvgMs*100)/100, math.Round(s.P99Ms*100)/100, round(s.Seconds)
	return s
}

// benchResult is one saved benchmark run.
type benchResult struct {
	ID      string     `json:"id"`
	When    int64      `json:"when"` // unix ms
	Game    string     `json:"game"`
	Process string     `json:"process"`
	Label   string     `json:"label"` // "before", "after" or anything typed
	Stats   benchStats `json:"stats"`
	CPU     float64    `json:"cpu"`
	GPU     float64    `json:"gpu"`
	GPUTemp float64    `json:"gpuTemp"`
	Graph   []float64  `json:"graph"` // FPS per second, for the chart
}
