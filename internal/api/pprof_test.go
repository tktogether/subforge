// Package api 的 pprof 测试：覆盖 pprof.go 的 13 个零覆盖函数。
//
// 设计思路：
//   - isDebugMode 三个分支用表驱动直接调未导出函数；
//   - 11 个 pprof 处理器结构一致（!debug → 404，debug → 转发 net/http/pprof），
//     分两组表驱动：404 分支（不带 ?debug=1）与成功分支（带 ?debug=1）；
//   - handleMetrics 用 /api/metrics（无 debug 门）断言 JSON 结构。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestIsDebugMode 覆盖 isDebugMode 的 3 个分支 + query 优先级 + debug=0。
// 注意：用 t.Setenv 的子测试不能 t.Parallel（会 panic），故全串行。
func TestIsDebugMode(t *testing.T) {
	// 进入测试前先确保 SUBFORGE_DEBUG 没被外部污染。
	t.Setenv("SUBFORGE_DEBUG", "")

	cases := []struct {
		name   string
		query  string // RawQuery
		envVal string // SUBFORGE_DEBUG 值（"" 表示未设）
		want   bool
	}{
		// 分支①：query debug=1 → true（即便环境变量未设）
		{"query debug=1 env空", "debug=1", "", true},
		// query 优先级证明：query=1 且 env=1 仍 true
		{"query debug=1 env=1", "debug=1", "1", true},
		// debug=0 → false（不等于 "1"）
		{"query debug=0", "debug=0", "", false},
		// 分支②：query 空 + env=1 → true
		{"env=1 query空", "", "1", true},
		// 分支③：query 空 + env 未设 → false
		{"env未设 query空", "", "", false},
		// env 为非 "1" 值 → false（== "1" 才 true）
		{"env=0 query空", "", "0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("SUBFORGE_DEBUG", c.envVal)
			req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
			req.URL.RawQuery = c.query
			if got := isDebugMode(req); got != c.want {
				t.Fatalf("isDebugMode = %v，期望 %v (query=%q env=%q)", got, c.want, c.query, c.envVal)
			}
		})
	}
}

// TestIsDebugModeEnvUnset 覆盖 os.Unsetenv("SUBFORGE_DEBUG") 的分支③。
// 单独成函数：t.Setenv 只能设值，无法 unset，用 os.Unsetenv 后立即调。
func TestIsDebugModeEnvUnset(t *testing.T) {
	// 先确保真的 unset（避免上个子测试的残留）
	os.Unsetenv("SUBFORGE_DEBUG")
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	// query 空 + env 未设 → false
	if isDebugMode(req) {
		t.Fatal("isDebugMode(空 query + env 未设) = true，期望 false")
	}
	// 顺带验证 query=1 在 env 未设时仍 true（分支①）
	req2 := httptest.NewRequest(http.MethodGet, "/debug/pprof/?debug=1", nil)
	if !isDebugMode(req2) {
		t.Fatal("isDebugMode(debug=1 + env 未设) = false，期望 true")
	}
}

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

// TestPprofEndpointsRequireDebugMode 覆盖每个处理器的 404 分支（!isDebugMode）。
func TestPprofEndpointsRequireDebugMode(t *testing.T) {
	// 确保环境变量清空，不被污染
	t.Setenv("SUBFORGE_DEBUG", "")
	ts := newTestServer(t, nil)
	for _, path := range pprofPaths {
		t.Run(path, func(t *testing.T) {
			status, _, _ := doGet(t, ts.URL+path, "", "")
			if status != http.StatusNotFound {
				t.Fatalf("%s 无 debug → 状态 %d，期望 404", path, status)
			}
		})
	}
}

// TestPprofEndpointsInDebugMode 覆盖每个处理器的成功分支（isDebugMode → 转发 net/http/pprof）。
// profile/trace 默认跑 30s/1s，必须显式传 seconds=1，否则卡死。
func TestPprofEndpointsInDebugMode(t *testing.T) {
	ts := newTestServer(t, nil)

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
