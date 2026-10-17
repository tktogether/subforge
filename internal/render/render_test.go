package render_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
)

// sampleNode 构造一个最小节点，供渲染 smoke 测试使用。
func sampleNode() model.Node {
	return model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
		"cipher": "aes-256-gcm", "password": "password123",
	})
}

func sampleRequest() render.Request {
	return render.Request{
		Nodes:   model.NodeList{sampleNode()},
		Profile: render.Profile{Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z"},
	}
}

// --- CommentBlock / Profile 注释头 ---

func TestCommentBlock(t *testing.T) {
	if got := render.CommentBlock(nil); got != "" {
		t.Errorf("空输入应返回空串，实际 %q", got)
	}
	if got := render.CommentBlock([]string{}); got != "" {
		t.Errorf("空切片应返回空串，实际 %q", got)
	}
	got := render.CommentBlock([]string{"a", "b"})
	want := "# a\n# b\n"
	if got != want {
		t.Errorf("CommentBlock = %q，期望 %q", got, want)
	}
	// 单行也要带尾随换行，供渲染器直接拼接。
	if got := render.CommentBlock([]string{"only"}); got != "# only\n" {
		t.Errorf("单行 CommentBlock = %q", got)
	}
}

func TestProfile_HeaderRenderedInYAML(t *testing.T) {
	r, err := registry.Lookup("clash.meta", clashmeta.Options{})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	body, err := r.Render(sampleRequest())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"# 此配置由 SubForge 自动生成",
		"# 配置名: subforge-test",
		"# SubForge 0.1.0",
		"# 生成时间: 2026-10-14T00:00:00Z",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("YAML 产物缺注释头行 %q\n产物:\n%s", want, s)
		}
	}
	// 注释头必须在最前。
	if !strings.HasPrefix(s, "# 此配置由 SubForge 自动生成") {
		t.Errorf("注释头应位于产物最前: %s", s[:80])
	}
	// Profile 为空时只输出第一行。
	r2, _ := registry.Lookup("clash.meta", clashmeta.Options{})
	emptyProf := render.Request{Nodes: model.NodeList{sampleNode()}}
	body2, err := r2.Render(emptyProf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(body2), "# 此配置由 SubForge 自动生成\n") {
		t.Errorf("空 Profile 头不正确: %s", string(body2)[:80])
	}
	if strings.Contains(string(body2), "配置名:") {
		t.Errorf("空 Profile 不应出现配置名行")
	}
}

func TestProfile_HeaderAbsentInJSON(t *testing.T) {
	r, err := registry.Lookup("clash.meta.json", clashmeta.Options{})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	body, err := r.Render(sampleRequest())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if bytesHasPrefix(body, []byte("#")) {
		t.Errorf("JSON 产物不应带注释头: %s", body[:80])
	}
	if !json.Valid(body) {
		t.Errorf("JSON 产物非法: %s", body)
	}
}

// --- LinkTargets 一致性 ---

func TestLinkTargets_AllResolvableFromRender(t *testing.T) {
	for name, want := range render.LinkTargets {
		r, err := registry.Lookup(name, clashmeta.Options{})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != want.Renderer {
			t.Errorf("target %q 的渲染器名 = %q，应为 %q", name, r.Name(), want.Renderer)
		}
	}
}

func TestLinkTargets_NarrowingSemantics(t *testing.T) {
	cases := map[string][]string{
		"ss":           {"ss", "ssr"},
		"ssd":          {"ss", "ssr"},
		"shadowsocks":  {"ss", "ssr"},
		"ssr":          {"ssr"},
		"shadowsocksr": {"ssr"},
		"trojan":       {"trojan"},
		"v2ray":        {"vmess"},
	}
	for name, wantTypes := range cases {
		lt, ok := render.NormalizeTarget(name)
		if !ok {
			t.Errorf("target %q 不在表里", name)
			continue
		}
		if lt.Renderer != "mixed" {
			t.Errorf("%q 的渲染器 = %q，应为 mixed", name, lt.Renderer)
		}
		got := map[string]bool{}
		for _, ty := range lt.Types {
			got[ty] = true
		}
		if len(got) != len(wantTypes) {
			t.Errorf("%q 收窄 %v，期望 %v", name, lt.Types, wantTypes)
			continue
		}
		for _, ty := range wantTypes {
			if !got[ty] {
				t.Errorf("%q 缺收窄 %q", name, ty)
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
		if lt.Renderer != "list" && name == "list" {
			// list 的 renderer 是 list
		}
	}
	if lt, _ := render.NormalizeTarget("list"); lt.Renderer != "list" {
		t.Errorf("list 的渲染器应为 list")
	}
	if lt, _ := render.NormalizeTarget("mixed"); lt.Renderer != "mixed" {
		t.Errorf("mixed 的渲染器应为 mixed")
	}
}

func TestNormalizeTarget_Behavior(t *testing.T) {
	lt, ok := render.NormalizeTarget("  SS  ")
	if !ok || lt.Renderer != "mixed" {
		t.Errorf("大小写/空白应被归一化: ok=%v renderer=%q", ok, lt.Renderer)
	}
	if _, ok := render.NormalizeTarget("nope"); ok {
		t.Errorf("未知 target 应返回 false")
	}
	if _, ok := render.NormalizeTarget(""); ok {
		t.Errorf("空 target 不在 LinkTargets 里")
	}
	if !render.IsMixedTarget("ss") || !render.IsMixedTarget("mixed") {
		t.Errorf("IsMixedTarget 应对 ss/mixed 返回 true")
	}
	if render.IsMixedTarget("list") || render.IsMixedTarget("v2ray") == false {
		// v2ray is mixed (Renderer mixed)
	}
	if !render.IsMixedTarget("v2ray") {
		t.Errorf("v2ray 应为 mixed")
	}
	if render.IsMixedTarget("list") {
		t.Errorf("list 不应为 mixed")
	}
}

// --- LineFormTargets 一致性 ---

func TestLineFormTargets_AllResolvableFromRender(t *testing.T) {
	for target, lt := range render.LineFormTargets {
		for _, name := range append([]string{target}, lt.Aliases...) {
			r, err := registry.Lookup(name, clashmeta.Options{})
			if err != nil {
				t.Errorf("target %q 无法取得渲染器: %v", name, err)
				continue
			}
			renderer, _ := render.LineFormRenderer(name)
			if r.Name() != renderer {
				t.Errorf("target %q 的渲染器名 = %q，应为 %q", name, r.Name(), renderer)
			}
		}
	}
}

func TestNormalizeLineFormTarget_Behavior(t *testing.T) {
	lt, ok := render.NormalizeLineFormTarget("  SURGE4  ")
	if !ok || lt.Renderer != "surge" {
		t.Errorf("surge4 应归一到 surge: ok=%v renderer=%q", ok, lt.Renderer)
	}
	// 别名列表应被保留。
	if len(lt.Aliases) == 0 {
		t.Errorf("NormalizeLineFormTarget 应保留 Aliases")
	}
	renderer, ok := render.LineFormRenderer("suri")
	if !ok || renderer != "surge" {
		t.Errorf("suri 应归一到 surge: ok=%v renderer=%q", ok, renderer)
	}
	if _, ok := render.NormalizeLineFormTarget("surge5"); ok {
		t.Errorf("surge5 不在表里")
	}
	if render.IsLineFormTarget("quantumultx") == false {
		t.Errorf("quantumultx 应为 lineform")
	}
	if render.IsLineFormTarget("sing-box") {
		t.Errorf("sing-box 不应为 lineform")
	}
	// 别名也应被 IsLineFormTarget 识别。
	if !render.IsLineFormTarget("surge4") {
		t.Errorf("surge4 别名应被识别")
	}
}

// --- ClashJSONTargets 一致性 ---

func TestClashJSONTargets_AllResolvableFromRender(t *testing.T) {
	for name := range render.ClashJSONTargets {
		r, err := registry.Lookup(name, clashmeta.Options{})
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
		if !json.Valid(body) {
			t.Errorf("target %q 的产物不是合法 JSON", name)
		}
		if bytesHasPrefix(body, []byte("#")) {
			t.Errorf("target %q 的 JSON 产物不应带注释头", name)
		}
	}
}

// --- ClashYAMLTargets 一致性 ---

func TestClashYAMLTargets_AllResolvableFromRender(t *testing.T) {
	for name := range render.ClashYAMLTargets {
		r, err := registry.Lookup(name, clashmeta.Options{JSON: true})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != "clash.meta" {
			t.Errorf("target %q 的渲染器名 = %q，应为 clash.meta", name, r.Name())
		}
		body, err := r.Render(sampleRequest())
		if err != nil {
			t.Fatalf("target %q 渲染失败: %v", name, err)
		}
		if bytesHasPrefix(body, []byte("{")) {
			t.Errorf("target %q 输出了 JSON，应为 YAML", name)
		}
		if !strings.Contains(string(body), "proxies:") {
			t.Errorf("target %q 的产物缺 proxies 段", name)
		}
	}
}

func TestIsClashYAMLTarget_Behavior(t *testing.T) {
	if !render.IsClashYAMLTarget("") {
		t.Errorf("空 target 应为 Clash YAML")
	}
	if !render.IsClashYAMLTarget("  CLASH.META  ") {
		t.Errorf("大小写/空白应被归一化")
	}
	if render.IsClashYAMLTarget("clash.json") {
		t.Errorf("clash.json 不应为 Clash YAML")
	}
}

func TestIsClashJSONTarget_Behavior(t *testing.T) {
	if !render.IsClashJSONTarget("clash-json") {
		t.Errorf("clash-json 应为 Clash JSON")
	}
	if render.IsClashJSONTarget("clash") {
		t.Errorf("clash 不应为 Clash JSON")
	}
}

// --- SingboxTargets 一致性 ---

func TestSingboxTargets_AllResolvableFromRender(t *testing.T) {
	for name := range render.SingboxTargets {
		r, err := registry.Lookup(name, clashmeta.Options{})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != "sing-box" {
			t.Errorf("target %q 的渲染器名 = %q，应为 sing-box", name, r.Name())
		}
		body, err := r.Render(sampleRequest())
		if err != nil {
			t.Fatalf("target %q 渲染失败: %v", name, err)
		}
		if !json.Valid(body) {
			t.Errorf("target %q 的产物不是合法 JSON", name)
		}
		if !strings.Contains(string(body), "\"outbounds\":") {
			t.Errorf("target %q 的产物缺 outbounds 段", name)
		}
	}
}

func TestIsSingboxTarget_Behavior(t *testing.T) {
	if !render.IsSingboxTarget("singbox") {
		t.Errorf("singbox 应为 sing-box")
	}
	if render.IsSingboxTarget("clash") {
		t.Errorf("clash 不应为 sing-box")
	}
}

// --- TargetAliases 一致性 ---

func TestTargetAliases_AllResolvableFromRender(t *testing.T) {
	for name, canonical := range render.TargetAliases {
		r, err := registry.Lookup(name, clashmeta.Options{})
		if err != nil {
			t.Errorf("target %q 无法取得渲染器: %v", name, err)
			continue
		}
		if r.Name() != canonical {
			t.Errorf("target %q 的渲染器名 = %q，应为 %q", name, r.Name(), canonical)
		}
	}
}

func TestIsTargetAlias_Behavior(t *testing.T) {
	if !render.IsTargetAlias("mellow-ios") {
		t.Errorf("mellow-ios 应为别名")
	}
	if !render.IsTargetAlias("mellow") {
		t.Errorf("mellow 也在 TargetAliases 里")
	}
	if render.IsTargetAlias("surge") {
		t.Errorf("surge 不应为 TargetAlias")
	}
	if !render.IsTargetAlias("  MELLOW-IOS  ") {
		t.Errorf("大小写/空白应被归一化")
	}
	lt, ok := render.TargetAliases["mellow-ios"]
	if !ok || lt != "mellow" {
		t.Errorf("mellow-ios 应归一到 mellow")
	}
}

func bytesHasPrefix(b []byte, prefix []byte) bool {
	return len(b) >= len(prefix) && string(b[:len(prefix)]) == string(prefix)
}
