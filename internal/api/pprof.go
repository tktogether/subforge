// Package api 提供 HTTP 入口：路由、中间件与处理器。
//
// pprof 端点只在启动期显式开启（serve -pprof / SUBFORGE_PPROF=1）时可用；
// 生产默认关闭，即使请求带 ?debug=1 也返回 404。
package api

import (
	"net/http"
	"net/http/pprof"
	"runtime"
	"time"
)

// handlePprofIndex 是 pprof 索引页。
func (s *Server) handlePprofIndex(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Index(w, r)
}

// handlePprofCmdline 是 pprof cmdline。
func (s *Server) handlePprofCmdline(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Cmdline(w, r)
}

// handlePprofProfile 是 pprof profile。
func (s *Server) handlePprofProfile(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Profile(w, r)
}

// handlePprofSymbol 是 pprof symbol。
func (s *Server) handlePprofSymbol(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Symbol(w, r)
}

// handlePprofTrace 是 pprof trace。
func (s *Server) handlePprofTrace(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Trace(w, r)
}

// handlePprofHeap 是 pprof heap。
func (s *Server) handlePprofHeap(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Handler("heap").ServeHTTP(w, r)
}

// handlePprofGoroutine 是 pprof goroutine。
func (s *Server) handlePprofGoroutine(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Handler("goroutine").ServeHTTP(w, r)
}

// handlePprofAllocs 是 pprof allocs。
func (s *Server) handlePprofAllocs(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Handler("allocs").ServeHTTP(w, r)
}

// handlePprofBlock 是 pprof block。
func (s *Server) handlePprofBlock(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Handler("block").ServeHTTP(w, r)
}

// handlePprofMutex 是 pprof mutex。
func (s *Server) handlePprofMutex(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Handler("mutex").ServeHTTP(w, r)
}

// handlePprofThreadcreate 是 pprof threadcreate。
func (s *Server) handlePprofThreadcreate(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.PProf {
		http.NotFound(w, r)
		return
	}
	pprof.Handler("threadcreate").ServeHTTP(w, r)
}

// handleMetrics 返回运行时指标（内存、GC、协程数等）。
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	metrics := map[string]any{
		"uptime":     time.Since(s.startedAt).String(),
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]any{
			"heap_alloc":  ms.HeapAlloc,
			"heap_sys":    ms.HeapSys,
			"heap_idle":   ms.HeapIdle,
			"heap_inuse":  ms.HeapInuse,
			"stack_sys":   ms.StackSys,
			"stack_inuse": ms.StackInuse,
			"sys":         ms.Sys,
			"num_gc":      ms.NumGC,
			"total_alloc": ms.TotalAlloc,
			"alloc":       ms.Alloc,
			"gc_cpu_frac": ms.GCCPUFraction,
		},
		"cpu": map[string]any{
			"gomaxprocs": runtime.GOMAXPROCS(0),
			"num_cpu":    runtime.NumCPU(),
		},
	}

	s.writeJSON(w, http.StatusOK, metrics)
}
