package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
)

// TestLinkTargets_AllResolvable 是 render.LinkTargets 与 registry 之间的一致性守门。
// 这张表被两处消费：convert 拿它做协议收窄，registry 拿它做 target 合法性判断。
// 历史上 registry 认不全 link target，导致 `/sub?target=ssr` 能转而 CLI `-format ssr`
// 报"不支持的输出格式"。任何新加的 target 都必须同时能通过这道 Lookup。
func TestLinkTargets_AllResolvable(t *testing.T) {
	for name := range render.LinkTargets {
		r, err := Lookup(name, clashmeta.Options{})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		want, ok := render.NormalizeTarget(name)
		if !ok {
			t.Errorf("target %q 不在 render.LinkTargets 里", name)
			continue
		}
		// 别名归一到同一个规范渲染器名：sub / subscription 都落到 list。
		if r.Name() != want.Renderer {
			t.Errorf("target %q 的渲染器名 = %q，应为 %q", name, r.Name(), want.Renderer)
		}
	}
}

// TestLinkTargets_NarrowingIsPreserved 保证收窄语义在归一化时不丢：
// ss 系含 ss 与 ssr，v2ray 只含 vmess，这与 subconverter 实机输出对齐。
func TestLinkTargets_NarrowingIsPreserved(t *testing.T) {
	cases := map[string]map[string]bool{
		"ss":     {"ss": true, "ssr": true},
		"ssd":    {"ss": true, "ssr": true},
		"ssr":    {"ssr": true},
		"trojan": {"trojan": true},
		"v2ray":  {"vmess": true},
	}
	for name, want := range cases {
		lt, ok := render.NormalizeTarget(name)
		if !ok {
			t.Errorf("target %q 不在表里", name)
			continue
		}
		if lt.Renderer != "mixed" {
			t.Errorf("%q 的渲染器 = %q，应为 mixed", name, lt.Renderer)
		}
		got := make(map[string]bool, len(lt.Types))
		for _, ty := range lt.Types {
			got[ty] = true
		}
		if len(got) != len(want) {
			t.Errorf("%q 收窄 %v，期望 %v", name, got, want)
			continue
		}
		for ty := range want {
			if !got[ty] {
				t.Errorf("%q 缺收窄 %q：%v", name, ty, lt.Types)
			}
		}
	}

	// 不收窄的 target
	for _, name := range []string{"list", "mixed"} {
		lt, ok := render.NormalizeTarget(name)
		if !ok {
			t.Errorf("target %q 不在表里", name)
			continue
		}
		if len(lt.Types) != 0 {
			t.Errorf("%q 不该有协议收窄：%v", name, lt.Types)
		}
	}
}

// TestLineFormTargets_AllAliasesResolvable 是 render.LineFormTargets 别名与
// registry 之间的一致性守门。Available() 会把别名一并列出，所以每个别名都必须
// 能真的通过 Lookup——否则用户照抄错误提示反而踩空（历史上 surge4 就是这么漏掉的）。
func TestLineFormTargets_AllAliasesResolvable(t *testing.T) {
	for target := range render.LineFormTargets {
		for _, name := range append([]string{target}, render.LineFormTargets[target].Aliases...) {
			want, ok := render.LineFormRenderer(name)
			if !ok {
				t.Errorf("target %q 不在 render.LineFormTargets 的别名表里", name)
				continue
			}
			r, err := Lookup(name, clashmeta.Options{})
			if err != nil {
				t.Errorf("target %q 无法取得渲染器: %v", name, err)
				continue
			}
			// 别名归一到规范渲染器名：产物头与文件名都从它派生，必须逐字节一致。
			if r.Name() != want {
				t.Errorf("target %q 的渲染器名 = %q，应为 %q", name, r.Name(), want)
				continue
			}
			wantR, err := Lookup(want, clashmeta.Options{})
			if err != nil {
				t.Fatalf("target %q 无法取得渲染器: %v", want, err)
			}
			wantBody, err := wantR.Render(sampleRequest())
			if err != nil {
				t.Fatalf("target %q 渲染失败: %v", want, err)
			}
			gotBody, err := r.Render(sampleRequest())
			if err != nil {
				t.Fatalf("target %q 渲染失败: %v", name, err)
			}
			if !bytes.Equal(gotBody, wantBody) {
				t.Errorf("target %q 与规范名 %q 的产物不一致（别名不该改变输出）", name, want)
			}
		}
	}
}

// TestTargetAliases_AllResolvable 与 LinkFormTargets 的守门对称：
// TargetAliases 里的每个别名都必须真的能过 Lookup，且渲染器名归一到同一个。
func TestTargetAliases_AllResolvable(t *testing.T) {
	for name := range render.TargetAliases {
		want, ok := render.TargetAliases[name]
		if !ok {
			t.Errorf("target %q 不在 TargetAliases 里", name)
			continue
		}
		r, err := Lookup(name, clashmeta.Options{})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != want {
			t.Errorf("target %q 的渲染器名 = %q，应为 %q", name, r.Name(), want)
		}
	}
}

// TestClashJSONTargets_AllAliasesResolvable 与 LineFormTargets 的守门对称：
// Available() 会把 JSON 别名一并列出，每个都必须真能过 Lookup，
// 且渲染器名归一到 clash.meta.json（X-Convert-Format 与落盘名都从它派生）。
func TestClashJSONTargets_AllAliasesResolvable(t *testing.T) {
	for name := range render.ClashJSONTargets {
		r, err := Lookup(name, clashmeta.Options{})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != "clash.meta.json" {
			t.Errorf("target %q 的渲染器名 = %q，应为 clash.meta.json", name, r.Name())
		}
		body, err := r.Render(sampleRequest())
		if err != nil {
			t.Fatalf("target %q 渲染失败: %v", name, err)
		}
		// JSON 产物必须能被标准库解析回来，且没有 YAML 注释头。
		if !json.Valid(body) {
			t.Errorf("target %q 的产物不是合法 JSON：%s", name, body)
		}
		if bytes.HasPrefix(body, []byte("#")) {
			t.Errorf("target %q 的 JSON 产物不应带注释头", name)
		}
	}
}

// TestSingboxTargets_AllResolvable sing-box 是独立内核，target 写法都归一到同一个渲染器名。
func TestSingboxTargets_AllResolvable(t *testing.T) {
	for name := range render.SingboxTargets {
		r, err := Lookup(name, clashmeta.Options{})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != "sing-box" {
			t.Errorf("target %q 的渲染器名 = %q，应为 sing-box", name, r.Name())
			continue
		}
		body, err := r.Render(sampleRequest())
		if err != nil {
			t.Fatalf("target %q 渲染失败: %v", name, err)
			continue
		}
		if !json.Valid(body) {
			t.Errorf("target %q 的产物不是合法 JSON：%s", name, body)
		}
		if bytes.HasPrefix(body, []byte("#")) {
			t.Errorf("target %q 的 JSON 产物不应带注释头", name)
		}
		if !strings.Contains(string(body), "\"outbounds\":") {
			t.Errorf("target %q 的产物缺 outbounds 段：%s", name, body)
		}
	}
}

// TestClashYAMLOutputsYAML 反过守一面对称性：YAML 系的每个写法都必须走 YAML 输出，
// 且调用方传入的 JSON 标记不能污染输出（序列化格式由 target 决定）。
func TestClashYAMLOutputsYAML(t *testing.T) {
	for name := range render.ClashYAMLTargets {
		r, err := Lookup(name, clashmeta.Options{JSON: true})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != "clash.meta" {
			t.Errorf("target %q 的渲染器名 = %q，应为 clash.meta", name, r.Name())
			continue
		}
		body, err := r.Render(sampleRequest())
		if err != nil {
			t.Fatalf("target %q 渲染失败: %v", name, err)
			continue
		}
		if bytes.HasPrefix(body, []byte("{")) {
			t.Errorf("target %q 输出了 JSON，应为 YAML", name)
		}
		if !strings.Contains(string(body), "proxies:") {
			t.Errorf("target %q 的产物缺 proxies 段：%s", name, body)
		}
	}
}

// TestSsSubBase64Toggle base64 只改变外层编码，渲染器名与裸 JSON 内容都不该变。
func TestSsSubBase64Toggle(t *testing.T) {
	plainR, err := Lookup("ss-sub", clashmeta.Options{})
	if err != nil {
		t.Fatalf("ss-sub 无法取得渲染器: %v", err)
	}
	plain, err := plainR.Render(sampleRequest())
	if err != nil {
		t.Fatalf("ss-sub 渲染失败: %v", err)
	}
	if !bytes.HasSuffix(plain, []byte("]")) {
		t.Fatalf("裸 JSON 形态应以 ] 结尾，实际: %s", plain)
	}
	b64R, err := Lookup("ss-sub-b64", clashmeta.Options{})
	if err != nil {
		t.Fatalf("ss-sub-b64 无法取得渲染器: %v", err)
	}
	encoded, err := b64R.Render(sampleRequest())
	if err != nil {
		t.Fatalf("ss-sub-b64 渲染失败: %v", err)
	}
	if bytes.Contains(encoded, []byte("[")) || bytes.Contains(encoded, []byte("]")) {
		t.Errorf("base64 形态不应含裸 JSON 括号")
	}
}

// sampleRequest 构造一个最小的渲染输入，供别名一致性断言使用。
func sampleRequest() render.Request {
	return render.Request{
		Nodes: model.NodeList{
			model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
				"cipher": "aes-256-gcm", "password": "password123",
			}),
		},
		Profile: render.Profile{Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z"},
	}
}

// TestLookup_Unsupported 拼错的名字必须报未支持格式，不能静默回落成默认配置。
func TestLookup_Unsupported(t *testing.T) {
	for _, bad := range []string{"srr", "vles", "surge2-unknown", "ssr2"} {
		if _, err := Lookup(bad, clashmeta.Options{}); err == nil {
			t.Errorf("target %q 应当报未支持", bad)
		} else if !errors.Is(err, ErrUnsupportedFormat) {
			t.Errorf("target %q 的错误 %v 没带上 ErrUnsupportedFormat", bad, err)
		}
	}
}

// TestAvailable_ContainsEveryTarget 错误信息里列出的 target 必须覆盖真源，
// 否则用户照抄提示也踩空。五张真源表逐个过——历史上这里只查了 LinkTargets，
// lineform 与 clash JSON 的别名全靠人肉记得加。
func TestAvailable_ContainsEveryTarget(t *testing.T) {
	avail := make(map[string]bool, len(Available()))
	for _, n := range Available() {
		avail[n] = true
	}
	for name := range render.LinkTargets {
		if name == "sub" || name == "subscription" {
			continue
		}
		if !avail[name] {
			t.Errorf("Available() 缺 link target %q", name)
		}
	}
	for target, lt := range render.LineFormTargets {
		if !avail[target] {
			t.Errorf("Available() 缺 lineform target %q", target)
		}
		for _, a := range lt.Aliases {
			if !avail[a] {
				t.Errorf("Available() 缺 lineform 别名 %q", a)
			}
		}
	}
	for name := range render.ClashJSONTargets {
		if !avail[name] {
			t.Errorf("Available() 缺 clash JSON target %q", name)
		}
	}
	for name := range render.SingboxTargets {
		if !avail[name] {
			t.Errorf("Available() 缺 sing-box target %q", name)
		}
	}
	for name := range render.TargetAliases {
		if !avail[name] {
			t.Errorf("Available() 缺 target 别名 %q", name)
		}
	}
}
