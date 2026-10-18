// spa.go — Vite 产物（internal/api/web/dist）的静态资源与 SPA fallback 路由。
//
// 交付形态是「单二进制内嵌前端」：/assets/*.js 这类真实文件必须按文件服务，
// 未知的「无扩展名」路径必须回落到 index.html 交给前端路由（createWebHistory 要求），
// 而带扩展名却找不到的资源必须 404 —— 一旦回落成 HTML，浏览器会把它当 JS 解析，
// 报 "Unexpected token '<'"，这类问题在浏览器里极难定位。
package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// spaExcludePrefixes 是不参与 SPA fallback 的路径前缀。
// 这些前缀要么是 JSON API，要么是转换入口（/sub /profile /render），要么是运维端点。
var spaExcludePrefixes = []string{
	"/api",
	"/sub",
	"/subscribe",
	"/profile",
	"/render",
	"/getruleset",
	"/inspect",
	"/version",
	"/healthz",
	"/debug/pprof",
	"/metrics",
}

// spaIndexFile 是 SPA 的入口文件（Vite 构建产物）。
const spaIndexFile = "index.html"

// handleSPAFallback 统一处理 GET/HEAD 的静态资源与前端路由。
//
// 判定顺序：排除前缀 → 真实文件 → 带扩展名则 404 → 否则 index.html。
func (s *Server) handleSPAFallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.handleNotFound(w, r)
		return
	}
	if spaExcluded(r.URL.Path) {
		s.handleNotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		s.serveIndex(w, r)
		return
	}
	// 路径穿越：嵌入文件系统本身也拦，但这里显式拒绝，错误更早更清楚。
	if strings.Contains(name, "..") {
		s.handleNotFound(w, r)
		return
	}
	if data, err := fs.ReadFile(webFS, name); err == nil {
		w.Header().Set("Content-Type", assetContentType(name))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}
	// 找不到文件：带扩展名的按资源缺失处理，绝不回落；其余交给前端路由。
	if strings.Contains(lastSegment(name), ".") {
		s.handleNotFound(w, r)
		return
	}
	s.serveIndex(w, r)
}

// spaExcluded 判断路径是否属于「不走前端路由」的前缀集合。
// 用「等于前缀本身或以 前缀/ 开头」匹配，避免 /apiary 这种同前缀不同路径被误伤。
func spaExcluded(p string) bool {
	for _, prefix := range spaExcludePrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

// lastSegment 返回路径最后一段（无扩展名判断只关心最后一段的 "."）。
func lastSegment(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// serveIndex 直出 index.html；产物缺失（例如没跑前端构建）时退化为 404。
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(webFS, spaIndexFile)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
