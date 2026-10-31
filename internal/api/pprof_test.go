// Package api 的 pprof 测试：覆盖 pprof.go 的 13 个零覆盖函数。
//
// 设计思路：
//   - 默认配置下 11 个 pprof 处理器都返回 404（带 ?debug=1 也不开放）；
//   - 显式 PProf=true 的启动配置下转发 net/http/pprof；
//   - handleMetrics 用 /api/metrics（无 debug 门）断言 JSON 结构。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// pprof 路径表：11 条端点。
var pprofPaths = []string{
	"/debug/pprof/",
	"/debug/pprof/cmdline",
	"/debug/pprof/profile",
	"/debug/pprof/symbol",
	"/debug/pprof/trace",
	"/debug/pprof/heap",
	"/debug/pprof/goroutine",
	"/debug/pprof/allocs",
	"/debug/pprof/block",
	"/debug/pprof/mutex",
	"/debug/pprof/threadcreate",
}

// TestPprofDisabledByDefault pprof 是启动期开关：默认 404，带 ?debug=1 也不开放。
func TestPprofDisabledByDefault(t *testing.T) {
	ts := newTestServer(t, nil)
	for _, path := range pprofPaths {
		t.Run(path, func(t *testing.T) {
			status, _, _ := doGet(t, ts.URL+path, "", "")
			if status != http.StatusNotFound {
				t.Fatalf("%s 默认 → 状态 %d，期望 404", path, status)
			}
			status, _, _ = doGet(t, ts.URL+path+"?debug=1", "", "")
			if status != http.StatusNotFound {
				t.Fatalf("%s?debug=1 默认 → 状态 %d，期望 404", path, status)
			}
		})
	}
}

// TestPprofEndpointsWithStartupFlag 覆盖启动期开启后的成功分支。
// profile/trace 默认跑 30s/1s，必须显式传 seconds=1，否则卡死。
func TestPprofEndpointsWithStartupFlag(t *testing.T) {
	ts, _ := uiSubServerCfg(t, func(c *Config) { c.PProf = true })

	// 不带 seconds 的端点（索引/静态 profile）
	type exp struct {
		path     string
		contains string // 正文必须包含的子串；"" 表示只验状态码
		ctPrefix string // Content-Type 前缀；"" 表示不验
	}
	ends := []exp{
		{"/debug/pprof/?debug=1", "goroutine", ""},
		{"/debug/pprof/cmdline?debug=1", "", "text/plain"},
		{"/debug/pprof/symbol?debug=1", "num_symbols:", ""},
		{"/debug/pprof/heap?debug=1", "heap profile", ""},
		{"/debug/pprof/goroutine?debug=1", "goroutine profile", ""},
		{"/debug/pprof/allocs?debug=1", "heap profile", ""},
		// 采样率为 0，正文可能空，只断言状态码
		{"/debug/pprof/block?debug=1", "", ""},
		{"/debug/pprof/mutex?debug=1", "", ""},
		{"/debug/pprof/threadcreate?debug=1", "", ""},
	}
	for _, e := range ends {
		status, hdr, body := doGet(t, ts.URL+e.path, "", "")
		if status != http.StatusOK {
			t.Fatalf("%s → 状态 %d，期望 200；body=%s", e.path, status, body)
		}
		if e.ctPrefix != "" {
			if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, e.ctPrefix) {
				t.Fatalf("%s → Content-Type=%q，期望以 %q 开头", e.path, ct, e.ctPrefix)
			}
		}
		if e.contains != "" && !strings.Contains(body, e.contains) {
			t.Fatalf("%s → 正文不含 %q；body=%s", e.path, e.contains, body)
		}
	}

	// profile & trace：必须 seconds=1，各约 1 秒
	for _, e := range []exp{
		{"/debug/pprof/profile?debug=1&seconds=1", "", ""},
		{"/debug/pprof/trace?debug=1&seconds=1", "", ""},
	} {
		status, _, body := doGet(t, ts.URL+e.path, "", "")
		if status != http.StatusOK {
			t.Fatalf("%s → 状态 %d，期望 200；body=%s", e.path, status, body)
		}
		if len(body) == 0 {
			t.Fatalf("%s → 正文为空，期望非空", e.path)
		}
	}
}

// TestMetrics 覆盖 handleMetrics 的 JSON 输出。
// newTestServer 不设 Config.StartedAt，startedAt 零值 → time.Since(零值) 仍是非空 string。
func TestMetrics(t *testing.T) {
	ts := newTestServer(t, nil)
	status, hdr, body := doGet(t, ts.URL+"/api/metrics", "", "")
	if status != http.StatusOK {
		t.Fatalf("/api/metrics → 状态 %d，期望 200；body=%s", status, body)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("/api/metrics → Content-Type=%q，期望以 application/json 开头", ct)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("/api/metrics → JSON 解析失败: %v；body=%s", err, body)
	}

	// 顶层 4 个键
	for _, k := range []string{"uptime", "goroutines", "memory", "cpu"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("/api/metrics → 缺少顶层键 %q", k)
		}
	}

	// uptime：非空 string
	uptime, ok := m["uptime"].(string)
	if !ok {
		t.Fatalf("/api/metrics → uptime 类型 %T，期望 string", m["uptime"])
	}
	if uptime == "" {
		t.Fatal("/api/metrics → uptime 为空字符串，期望非空")
	}

	// goroutines：float64 且 >= 1
	goros, ok := m["goroutines"].(float64)
	if !ok {
		t.Fatalf("/api/metrics → goroutines 类型 %T，期望 float64", m["goroutines"])
	}
	if goros < 1 {
		t.Fatalf("/api/metrics → goroutines = %v，期望 >= 1", goros)
	}

	// memory：object，11 个键
	mem, ok := m["memory"].(map[string]any)
	if !ok {
		t.Fatalf("/api/metrics → memory 类型 %T，期望 object", m["memory"])
	}
	memKeys := []string{
		"heap_alloc", "heap_sys", "heap_idle", "heap_inuse",
		"stack_sys", "stack_inuse", "sys", "num_gc",
		"total_alloc", "alloc", "gc_cpu_frac",
	}
	for _, k := range memKeys {
		if _, ok := mem[k]; !ok {
			t.Fatalf("/api/metrics → memory 缺少键 %q", k)
		}
	}

	// cpu：object，gomaxprocs 与 num_cpu 存在且 >= 1
	cpu, ok := m["cpu"].(map[string]any)
	if !ok {
		t.Fatalf("/api/metrics → cpu 类型 %T，期望 object", m["cpu"])
	}
	for _, k := range []string{"gomaxprocs", "num_cpu"} {
		v, ok := cpu[k].(float64)
		if !ok {
			t.Fatalf("/api/metrics → cpu.%s 类型 %T，期望 float64", k, cpu[k])
		}
		if v < 1 {
			t.Fatalf("/api/metrics → cpu.%s = %v，期望 >= 1", k, v)
		}
	}
}
