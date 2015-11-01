package bpprof

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

var (
	units = []string{"B", "KB", "MB", "GB", "TB"}
)

func init() {
	http.Handle("/debug/bpprof/heap", http.HandlerFunc(Heap))
}

func formatSize(size int64) string {
	s := float64(size)
	for i := 0; i < len(units); i++ {
		if s < 1000 {
			return fmt.Sprintf("%.2f", s) + units[i]
		}
		s /= 1024
	}
	return fmt.Sprintf("%.0fTB", s)
}

type byInUseBytes []runtime.MemProfileRecord

func (x byInUseBytes) Len() int           { return len(x) }
func (x byInUseBytes) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byInUseBytes) Less(i, j int) bool { return x[i].InUseBytes() > x[j].InUseBytes() }

type byAllocBytes []runtime.MemProfileRecord

func (x byAllocBytes) Len() int           { return len(x) }
func (x byAllocBytes) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byAllocBytes) Less(i, j int) bool { return x[i].AllocBytes > x[j].AllocBytes }

type byAllocObjects []runtime.MemProfileRecord

func (x byAllocObjects) Len() int           { return len(x) }
func (x byAllocObjects) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byAllocObjects) Less(i, j int) bool { return x[i].AllocObjects > x[j].AllocObjects }

type byInUseObjects []runtime.MemProfileRecord

func (x byInUseObjects) Len() int           { return len(x) }
func (x byInUseObjects) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byInUseObjects) Less(i, j int) bool { return x[i].InUseObjects() > x[j].InUseObjects() }

// From: https://github.com/golang/go/blob/6b8762104a90c93ebd51149e7a031738832c5cdc/src/runtime/pprof/pprof.go#L326
// printStackRecord prints the function + source line information
// for a single stack trace.
func printStackRecord(w io.Writer, stk []uintptr, allFrames bool) {
	show := allFrames
	wasPanic := false
	for i, pc := range stk {
		f := runtime.FuncForPC(pc)
		if f == nil {
			show = true
			fmt.Fprintf(w, "#\t%#x\n", pc)
			wasPanic = false
		} else {
			tracepc := pc
			// Back up to call instruction.
			if i > 0 && pc > f.Entry() && !wasPanic {
				if runtime.GOARCH == "386" || runtime.GOARCH == "amd64" {
					tracepc--
				} else {
					tracepc -= 4 // arm, etc
				}
			}
			file, line := f.FileLine(tracepc)
			name := f.Name()
			// Hide runtime.goexit and any runtime functions at the beginning.
			// This is useful mainly for allocation traces.
			wasPanic = name == "runtime.panic"
			if name == "runtime.goexit" || !show && strings.HasPrefix(name, "runtime.") {
				continue
			}
			show = true
			fmt.Fprintf(w, "#\t%#x\t%s+%#x\t%s:%d\n", pc, name, pc-f.Entry(), file, line)
		}
	}
	if !show {
		// We didn't print anything; do it again,
		// and this time include runtime functions.
		printStackRecord(w, stk, true)
		return
	}
	fmt.Fprintf(w, "\n")
}

// Based on: https://github.com/golang/go/blob/6b8762104a90c93ebd51149e7a031738832c5cdc/src/runtime/pprof/pprof.go#L387
func Heap(w http.ResponseWriter, r *http.Request) {
	var p []runtime.MemProfileRecord
	n, ok := runtime.MemProfile(nil, true)
	for {
		// Allocate room for a slightly bigger profile,
		// in case a few more entries have been added
		// since the call to MemProfile.
		p = make([]runtime.MemProfileRecord, n+50)
		n, ok = runtime.MemProfile(p, true)
		if ok {
			p = p[0:n]
			break
		}
		// Profile grew; try again.
	}

	pm := make(map[uintptr]runtime.MemProfileRecord, len(p))

	for _, r := range p {
		// Based on: https://github.com/golang/go/blob/f9ed2f75c43cb8745a1593ec3e4208c46419216a/src/runtime/mprof.go#L150
		var h uintptr
		for _, pc := range r.Stack0 {
			h += pc
			h += h << 10
			h ^= h >> 6
		}
		h += h << 3
		h ^= h >> 11

		if _, ok := pm[h]; ok {
			r.AllocBytes += pm[h].AllocBytes
			r.FreeBytes += pm[h].FreeBytes
			r.AllocObjects += pm[h].AllocObjects
			r.FreeObjects += pm[h].FreeObjects
		}
		pm[h] = r
	}

	p = make([]runtime.MemProfileRecord, 0, len(pm))

	for _, r := range pm {
		p = append(p, r)
	}

	switch r.FormValue("sort") {
	default:
		sort.Sort(byInUseBytes(p))
	case "allocbytes":
		sort.Sort(byAllocBytes(p))
	case "allocobjects":
		sort.Sort(byAllocObjects(p))
	case "inuseobjects":
		sort.Sort(byInUseObjects(p))
	}

	tw := tabwriter.NewWriter(w, 1, 8, 1, '\t', 0)

	var total runtime.MemProfileRecord
	for _, r := range p {
		total.AllocBytes += r.AllocBytes
		total.AllocObjects += r.AllocObjects
		total.FreeBytes += r.FreeBytes
		total.FreeObjects += r.FreeObjects
	}

	// Technically the rate is MemProfileRate not 2*MemProfileRate,
	// but early versions of the C++ heap profiler reported 2*MemProfileRate,
	// so that's what pprof has come to expect.
	fmt.Fprintf(tw, "heap profile: %d: %d [%d: %d] @ heap/%d\n",
		total.InUseObjects(), total.InUseBytes(),
		total.AllocObjects, total.AllocBytes,
		2*runtime.MemProfileRate)

	fmt.Fprintf(tw, "# heap profile: %d: %s [%d: %s] @ heap/%d\n\n",
		total.InUseObjects(), formatSize(total.InUseBytes()),
		total.AllocObjects, formatSize(total.AllocBytes),
		2*runtime.MemProfileRate)

	for _, r := range p {
		fmt.Fprintf(tw, "%d: %d [%d: %d] @",
			r.InUseObjects(), r.InUseBytes(),
			r.AllocObjects, r.AllocBytes)
		for _, pc := range r.Stack() {
			fmt.Fprintf(tw, " %#x", pc)
		}
		fmt.Fprintf(tw, "\n# %d: %s [%d: %s]\n",
			r.InUseObjects(), formatSize(r.InUseBytes()),
			r.AllocObjects, formatSize(r.AllocBytes))
		printStackRecord(tw, r.Stack(), false)
	}

	// Print memstats information too.
	// Pprof will ignore, but useful for people
	s := new(runtime.MemStats)
	runtime.ReadMemStats(s)

	// Sort pauseNs in newer first,
	// and make it a nice to print duration.
	pauseNs := make([]time.Duration, 0, len(s.PauseNs))
	var pauseNsLongest time.Duration
	for i := (s.NumGC + 255) % 256; i > 0; i-- {
		d := time.Duration(int64(s.PauseNs[i]))
		if d > pauseNsLongest {
			pauseNsLongest = d
		}
		pauseNs = append(pauseNs, d)
	}
	for i := uint32(255); i > (s.NumGC+255)%256; i-- {
		d := time.Duration(int64(s.PauseNs[i]))
		if d > pauseNsLongest {
			pauseNsLongest = d
		}
		pauseNs = append(pauseNs, d)
	}

	fmt.Fprintf(tw, "\n# runtime.MemStats\n")
	fmt.Fprintf(tw, "# Alloc = %d (%s)\n", s.Alloc, formatSize(int64(s.Alloc)))
	fmt.Fprintf(tw, "# TotalAlloc = %d (%s)\n", s.TotalAlloc, formatSize(int64(s.TotalAlloc)))
	fmt.Fprintf(tw, "# Sys = %d (%s)\n", s.Sys, formatSize(int64(s.Sys)))
	fmt.Fprintf(tw, "# Lookups = %d\n", s.Lookups)
	fmt.Fprintf(tw, "# Mallocs = %d\n", s.Mallocs)
	fmt.Fprintf(tw, "# Frees = %d\n", s.Frees)

	fmt.Fprintf(tw, "# HeapAlloc = %d (%s)\n", s.HeapAlloc, formatSize(int64(s.HeapAlloc)))
	fmt.Fprintf(tw, "# HeapSys = %d (%s)\n", s.HeapSys, formatSize(int64(s.HeapSys)))
	fmt.Fprintf(tw, "# HeapIdle = %d (%s)\n", s.HeapIdle, formatSize(int64(s.HeapIdle)))
	fmt.Fprintf(tw, "# HeapInuse = %d (%s)\n", s.HeapInuse, formatSize(int64(s.HeapInuse)))
	fmt.Fprintf(tw, "# HeapReleased = %d (%s)\n", s.HeapReleased, formatSize(int64(s.HeapReleased)))
	fmt.Fprintf(tw, "# HeapObjects = %d (%s)\n", s.HeapObjects, formatSize(int64(s.HeapObjects)))

	fmt.Fprintf(tw, "# Stack = %d (%s) / %d (%s)\n", s.StackInuse, formatSize(int64(s.StackInuse)), s.StackSys, formatSize(int64(s.StackSys)))
	fmt.Fprintf(tw, "# MSpan = %d (%s) / %d (%s)\n", s.MSpanInuse, formatSize(int64(s.MSpanInuse)), s.MSpanSys, formatSize(int64(s.MSpanSys)))
	fmt.Fprintf(tw, "# MCache = %d (%s) / %d (%s)\n", s.MCacheInuse, formatSize(int64(s.MCacheInuse)), s.MCacheSys, formatSize(int64(s.MCacheSys)))
	fmt.Fprintf(tw, "# BuckHashSys = %d\n", s.BuckHashSys)

	fmt.Fprintf(tw, "# NextGC = %d\n", s.NextGC)
	fmt.Fprintf(tw, "# PauseNs = %v\n", pauseNs)
	fmt.Fprintf(tw, "# PauseNsLongest = %v\n", pauseNsLongest)
	fmt.Fprintf(tw, "# NumGC = %d\n", s.NumGC)
	fmt.Fprintf(tw, "# EnableGC = %v\n", s.EnableGC)
	fmt.Fprintf(tw, "# DebugGC = %v\n", s.DebugGC)

	if tw != nil {
		tw.Flush()
	}
}
