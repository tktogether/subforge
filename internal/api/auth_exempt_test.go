package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAuthExemptPath 覆盖豁免判定的全部分支。
//
// 豁免面必须收得很窄：只有 /subscribe、控制台外壳与前端路由深层链接可以绕过
// 访问令牌；凡是交给真实处理器（/sub、/getruleset、/healthz…）或 /api/* 的
// 路径，以及任何非 GET/HEAD 请求，都必须继续校验。
func TestAuthExemptPath(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		// 订阅入口：用订阅令牌自行鉴权
		{http.MethodGet, "/subscribe", true},
		// 控制台外壳
		{http.MethodGet, "/", true},
		{http.MethodHead, "/", true},
		{http.MethodGet, "/favicon.ico", true},
		{http.MethodGet, "/assets/index-abc123.js", true},
		{http.MethodGet, "/assets/index-abc123.css", true},
		// 前端路由深层链接（会回退到 index.html）
		{http.MethodGet, "/subscriptions", true},
		{http.MethodGet, "/rule-sources", true},
		{http.MethodGet, "/settings", true},
		// 数据接口与真实处理器：一律不豁免
		{http.MethodGet, "/api/version", false},
		{http.MethodGet, "/api/airports", false},
		{http.MethodGet, "/sub", false},
		{http.MethodGet, "/profile", false},
		{http.MethodGet, "/render", false},
		{http.MethodGet, "/getruleset", false},
		{http.MethodGet, "/inspect", false},
		{http.MethodGet, "/version", false},
		{http.MethodGet, "/healthz", false},
		{http.MethodGet, "/metrics", false},
		{http.MethodGet, "/debug/pprof/", false},
		// 带扩展名的未知路径不是前端路由，按 404 处理即可
		{http.MethodGet, "/robots.txt", false},
		// 非 GET/HEAD 不豁免
		{http.MethodPost, "/", false},
		{http.MethodPost, "/api/convert", false},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, c.path, nil)
		if got := authExemptPath(r); got != c.want {
			t.Errorf("authExemptPath(%s %s) = %v，want %v", c.method, c.path, got, c.want)
		}
	}
}

// TestAuth_ConsoleShellLoadsWithoutToken 是本次修复的核心契约：
// 设了访问令牌时，控制台外壳必须能匿名加载，否则浏览器只会看到一行 401 纯文本，
// 用户没有任何地方可以输入令牌。
func TestAuth_ConsoleShellLoadsWithoutToken(t *testing.T) {
	ts := newTestServer(t, func(c *Config) { c.Token = func() string { return "s3cr3t" } })

	for _, path := range []string{"/", "/subscriptions", "/favicon.ico"} {
		status, hdr, _ := doGet(t, ts.URL+path, "", "")
		if status == http.StatusUnauthorized {
			t.Errorf("GET %s 不应要求访问令牌，实际 401", path)
		}
		if hdr.Get("WWW-Authenticate") != "" {
			t.Errorf("GET %s 不应带 WWW-Authenticate", path)
		}
	}

	// 外壳能拿到真正的 index.html（前端构建产物存在时）。
	status, hdr, body := doGet(t, ts.URL+"/", "", "")
	if status != http.StatusOK {
		t.Fatalf("GET / = %d，want 200（internal/api/web/dist/index.html 应已构建）", status)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("GET / Content-Type = %q，want text/html", ct)
	}
	if !strings.Contains(body, "<div id=\"app\">") {
		t.Errorf("GET / 未返回控制台外壳，正文前 80 字节 = %q", firstBytes(body, 80))
	}

	// 静态资源同样豁免（用真实存在的产物文件名，避免落到 404 分支）。
	assets, err := fs.ReadDir(webFS, "assets")
	if err != nil || len(assets) == 0 {
		t.Fatalf("前端产物缺失：%v", err)
	}
	status, _, _ = doGet(t, ts.URL+"/assets/"+assets[0].Name(), "", "")
	if status == http.StatusUnauthorized {
		t.Errorf("GET /assets/%s 不应要求访问令牌，实际 401", assets[0].Name())
	}
}

// TestAuth_DataEndpointsStillGuarded 确认豁免没有削弱数据面：
// 同一个服务器上，/api/*、/sub 与其它真实处理器仍然必须校验访问令牌。
func TestAuth_DataEndpointsStillGuarded(t *testing.T) {
	ts := newTestServer(t, func(c *Config) { c.Token = func() string { return "s3cr3t" } })

	guarded := []string{"/api/version", "/api/targets", "/healthz", "/version", "/sub", "/getruleset"}
	for _, path := range guarded {
		if status, _, _ := doGet(t, ts.URL+path, "", ""); status != http.StatusUnauthorized {
			t.Errorf("GET %s 无令牌 = %d，应为 401", path, status)
		}
		if status, _, _ := doGet(t, ts.URL+path, "", "s3cr3t"); status == http.StatusUnauthorized {
			t.Errorf("GET %s 带正确令牌仍为 401", path)
		}
	}
}

// firstBytes 截断正文，便于在失败信息里看清实际返回了什么。
func firstBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
