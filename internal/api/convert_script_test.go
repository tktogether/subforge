package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/subforge"
)

// convertScriptServer 起一个注入了订阅管理器的 Server。
// 脚本库（config.ScriptResolver）正是靠 SetSubManager 注入引擎的，
// 所以这条用例必须走这条路，否则测的是另一条链路。
func convertScriptServer(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	mgr, err := subforge.NewManager(filepath.Join(t.TempDir(), "convert-script.db"))
	if err != nil {
		t.Fatalf("打开临时 SQLite: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test", Timeout: 5 * time.Second,
		Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	srv := New(Config{
		Engine: &convert.Engine{
			Fetcher: fetcher,
			RendererFor: func(target string) (render.Renderer, error) {
				return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	srv.SetSubManager(mgr)
	return srv, mgr
}

// ssScriptLink 生成一条 ss 分享链接（与 internal/convert 的测试 fixture 同形）。
func ssScriptLink(host string, port int, name string) string {
	userinfo := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:pass"))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, host, port, name)
}

// apiConvertOut 是 POST /api/convert 成功响应里我们关心的字段。
type apiConvertOut struct {
	Body     string `json:"body"`
	Nodes    int    `json:"nodes"`
	RawNodes int    `json:"rawNodes"`
}

// postAPIConvert 打一次 POST /api/convert，返回状态码与原始响应体。
func postAPIConvert(t *testing.T, srv *Server, body string) (int, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/convert", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIConvert(rr, req)
	return rr.Code, rr.Body.String()
}

// seedScriptPipeline 在库里建一条脚本 + 一条引用它的命名管线，并设置 script.enabled。
// 形状与控制台「脚本」页 + 「管线」页做出来的东西一致：
// 脚本名 e2e-hk-filter，管线步骤 op=filter.script、args.ref=e2e-hk-filter。
func seedScriptPipeline(t *testing.T, mgr *subforge.Manager, scriptsEnabled bool) {
	t.Helper()
	ctx := context.Background()
	if _, err := mgr.DB().CreateScript(ctx, store.Script{
		Name:    "e2e-hk-filter",
		Kind:    "filter",
		Code:    "node.name.indexOf('HK') >= 0",
		Enabled: true,
	}); err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	steps := `[{"op":"filter.script","args":{"ref":"e2e-hk-filter"}}]`
	if _, err := mgr.CreatePipeline(ctx, "e2e-hk-pipeline", "e2e", steps, true); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	v := "false"
	if scriptsEnabled {
		v = "true"
	}
	if err := mgr.SetSettings(ctx, map[string]string{subforge.ScriptEnabledKey: v}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
}

// TestAPIConvert_ScriptRefEnabled 端到端：网页那条链路（脚本 + 管线 ref= + 转换）
// 在 script.enabled 打开时必须 200，并且产物里只剩脚本放行的节点。
// 修复前这里固定 502「脚本步骤需要脚本库支持」。
func TestAPIConvert_ScriptRefEnabled(t *testing.T) {
	srv, mgr := convertScriptServer(t)
	seedScriptPipeline(t, mgr, true)

	text := strings.Join([]string{
		ssScriptLink("hk1.example.com", 8388, "HK-01"),
		ssScriptLink("jp1.example.com", 8388, "JP-01"),
		ssScriptLink("us1.example.com", 8388, "US-01"),
	}, "\n")
	body := fmt.Sprintf(`{"target":"clash.meta","pipeline":"e2e-hk-pipeline","text":%q}`, text)

	code, raw := postAPIConvert(t, srv, body)
	if code != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", code, raw)
	}
	var out apiConvertOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("解析响应: %v，body=%s", err, raw)
	}
	if out.RawNodes != 3 {
		t.Errorf("rawNodes = %d，want 3", out.RawNodes)
	}
	if out.Nodes != 1 {
		t.Errorf("nodes = %d，want 1（ref 脚本只放行 HK）", out.Nodes)
	}
	t.Logf("探针 API 端到端(开启): status=%d rawNodes=%d nodes=%d 产物含HK-01=%v 含JP-01=%v",
		code, out.RawNodes, out.Nodes, strings.Contains(out.Body, "HK-01"), strings.Contains(out.Body, "JP-01"))
	// 断言产物节点名单，不能只看状态码。
	if !strings.Contains(out.Body, "HK-01") {
		t.Errorf("产物应含 HK-01:\n%s", out.Body)
	}
	for _, gone := range []string{"JP-01", "US-01"} {
		if strings.Contains(out.Body, gone) {
			t.Errorf("产物不应含 %s（应被 ref 脚本过滤掉）:\n%s", gone, out.Body)
		}
	}
}

// TestAPIConvert_ScriptRefDisabled 关闭 script.enabled 后同一条链路必须失败，
// 且文案是「脚本步骤已禁用」——安全开关要真的管住 ref 脚本。
func TestAPIConvert_ScriptRefDisabled(t *testing.T) {
	srv, mgr := convertScriptServer(t)
	seedScriptPipeline(t, mgr, false)

	text := ssScriptLink("hk1.example.com", 8388, "HK-01")
	body := fmt.Sprintf(`{"target":"clash.meta","pipeline":"e2e-hk-pipeline","text":%q}`, text)

	code, raw := postAPIConvert(t, srv, body)
	if code == http.StatusOK {
		t.Fatalf("script.enabled=false 时不应 200，body=%s", raw)
	}
	// statusFor 对编译期错误没有特例，走默认的 502。
	if code != http.StatusBadGateway {
		t.Errorf("状态 = %d，want 502", code)
	}
	if !strings.Contains(raw, "脚本步骤已禁用") {
		t.Errorf("响应应含 脚本步骤已禁用: %s", raw)
	}
	t.Logf("探针 API 端到端(关闭): status=%d body=%s", code, strings.TrimSpace(raw))
}

// TestAPIConvert_ScriptRefNoResolver 锁定修复前的真实故障形态：管线能选中、脚本库没注入时，
// /api/convert 必须报「脚本步骤需要脚本库支持」——网页上看到的正是这条 502。
// 用例把 ScriptResolver 显式置回 nil，等价于 SetSubManager 只注入 ExtraPipelines 的旧行为。
func TestAPIConvert_ScriptRefNoResolver(t *testing.T) {
	srv, mgr := convertScriptServer(t)
	seedScriptPipeline(t, mgr, true)
	srv.engine.ScriptResolver = nil

	text := ssScriptLink("hk1.example.com", 8388, "HK-01")
	body := fmt.Sprintf(`{"target":"clash.meta","pipeline":"e2e-hk-pipeline","text":%q}`, text)

	code, raw := postAPIConvert(t, srv, body)
	if code != http.StatusBadGateway {
		t.Errorf("状态 = %d，want 502", code)
	}
	if !strings.Contains(raw, "脚本步骤需要脚本库支持") {
		t.Errorf("响应应含 脚本步骤需要脚本库支持: %s", raw)
	}
	t.Logf("探针 API 端到端(无 resolver): status=%d body=%s", code, strings.TrimSpace(raw))
}
