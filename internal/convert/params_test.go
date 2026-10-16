package convert

import (
	"net/url"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/transform"
)

// testNodes 给出带名称与类型的节点集，用于验证管线语义。
func testNodes() model.NodeList {
	return model.NodeList{
		model.NewNodeFromParts("HK-香港-01 - 10Gbps", "vless", "hk1.example.com", "443", nil),
		model.NewNodeFromParts("HK-香港-01 - 10Gbps", "vless", "hk1.example.com", "443", nil),
		model.NewNodeFromParts("JP-日本-01", "vmess", "jp1.example.com", "8443", nil),
		model.NewNodeFromParts("US-美国-01", "trojan", "us1.example.com", "443", nil),
	}
}

func mustParse(t *testing.T, raw string) Params {
	t.Helper()
	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseQuery(q)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", raw, err)
	}
	return p
}

// TestParseQuery_FullParamSet 覆盖 subconverter 参数超集的常规组合。
// 多值参数：url 用重复的 url= 参数表达（query 里必须百分号转义，和真实客户端
// 发请求的形状一致），其余用分号。
func TestParseQuery_FullParamSet(t *testing.T) {
	raw := "target=Clash.Meta" +
		"&url=http://a/sub&url=http://b/sub&url=http://c/sub" +
		"&insert=http://d/extra" +
		"&include=HK%3BJP" +
		"&exclude=- 02" +
		"&rename=HK%2C🇭🇰HK%3BJP%2C🇯🇵JP" +
		"&emoji=美国%2C🇺🇸" +
		"&list=1&udp=1&tfo=true&scv=on&tls13=yes" +
		"&sort=NAME&config=tpl-x"

	p := mustParse(t, raw)

	if p.Target != "clash.meta" {
		t.Errorf("Target = %q，应为 clash.meta（大小写归一）", p.Target)
	}
	if !sameStrings(p.URLs, []string{"http://a/sub", "http://b/sub", "http://c/sub"}) {
		t.Errorf("URLs = %v", p.URLs)
	}
	if len(p.Insert) != 1 || p.Insert[0] != "http://d/extra" {
		t.Errorf("Insert = %v", p.Insert)
	}
	if !sameStrings(p.Include, []string{"HK", "JP"}) {
		t.Errorf("Include = %v", p.Include)
	}
	if !sameStrings(p.Exclude, []string{"- 02"}) {
		t.Errorf("Exclude = %v", p.Exclude)
	}
	if len(p.Rename) != 2 {
		t.Fatalf("Rename = %v，应为 2 条", p.Rename)
	}
	if p.Rename[0].Pattern != "HK" || p.Rename[0].Replacement != "🇭🇰HK" {
		t.Errorf("Rename[0] = %+v", p.Rename[0])
	}
	if len(p.Emoji) != 1 || p.Emoji[0].Pattern != "美国" {
		t.Errorf("Emoji = %+v", p.Emoji)
	}
	if p.Sort != "name" {
		t.Errorf("Sort = %q", p.Sort)
	}
	if p.Config != "tpl-x" {
		t.Errorf("Config = %q", p.Config)
	}
	for _, on := range []bool{p.List, p.UDP, p.TFO, p.SCV, p.TLS13} {
		if !on {
			t.Error("布尔参数应为 true")
		}
	}
}

// parseValues 直接由 url.Values 构造参数，绕过 url.ParseQuery 的百分号解码
// 和 `;` 分隔符拒绝——测分号分隔形态时不能走字符串解析那条路。
func parseValues(t *testing.T, values url.Values) Params {
	t.Helper()
	p, err := ParseQuery(values)
	if err != nil {
		t.Fatalf("ParseQuery(%v): %v", values, err)
	}
	return p
}

// TestParseQuery_URLWithQueryString 回归：订阅地址自带查询串是常态（?token=），
// 曾按 `?` 当分隔符拆 url，结果 token 被丢掉、上游收不到凭证直接 404，
// 还多出一个 "token=..." 的假来源。
func TestParseQuery_URLWithQueryString(t *testing.T) {
	want := "http://1.2.3.4:3001/share/col/%E8%AE%A2%E9%98%85?token=abc-123"
	p := parseValues(t, url.Values{"url": {want}})
	if !sameStrings(p.URLs, []string{want}) {
		t.Errorf("URLs = %v，应为 %q", p.URLs, want)
	}

	// 分号分隔的多个来源仍然有效，且每个来源各自的查询串都保留。
	p = parseValues(t, url.Values{"url": {"http://a/sub?x=1;http://b/sub?y=2"}})
	if !sameStrings(p.URLs, []string{"http://a/sub?x=1", "http://b/sub?y=2"}) {
		t.Errorf("URLs = %v", p.URLs)
	}

	// INI [common] url= 写多个地址时用分号或逗号，不能靠 `?` 拆。
	p = parseValues(t, url.Values{"url": {"http://a/sub;http://b/sub"}})
	if !sameStrings(p.URLs, []string{"http://a/sub", "http://b/sub"}) {
		t.Errorf("URLs = %v", p.URLs)
	}
}

func TestParseQuery_BoolOff(t *testing.T) {
	p := mustParse(t, "list=0&udp=false&tfo=no&scv=off&tls13=&target=v2ray")
	for _, on := range []bool{p.List, p.UDP, p.TFO, p.SCV, p.TLS13} {
		if on {
			t.Error("布尔参数应为 false")
		}
	}
}

func TestParseQuery_CaseInsensitiveKeys(t *testing.T) {
	p := mustParse(t, "TARGET=Surge&Include=HK&EMOJI=JP%2C🇯🇵")
	if p.Target != "surge" || len(p.Include) != 1 || len(p.Emoji) != 1 {
		t.Errorf("参数键应大小写不敏感: %+v", p)
	}
}

// TestParseQuery_RenamePatternOnly 正则的首尾空格有语义，不能被解析层吃掉。
func TestParseQuery_RenamePatternOnly(t *testing.T) {
	p := mustParse(t, "rename=%20-%20.*")
	if len(p.Rename) != 1 || p.Rename[0].Pattern != " - .*" || p.Rename[0].Replacement != "" {
		t.Errorf("只有正则时应退化为空替换，实际 %+v", p.Rename)
	}
	if _, err := transform.NewRename(p.Rename); err != nil {
		t.Fatalf("正则里的空格应有语义: %v", err)
	}
}

func TestParseQuery_BadRegexRejected(t *testing.T) {
	for _, raw := range []string{"include=(", "exclude=[", "rename=(", "emoji=["} {
		k, v, _ := strings.Cut(raw, "=")
		if _, err := ParseQuery(url.Values{k: []string{v}}); err == nil {
			t.Errorf("%q 应报错", raw)
		}
	}
}

func TestParseQuery_TypeParam(t *testing.T) {
	p := mustParse(t, "type=vless%2Cvmess%3Btrojan")
	if !sameStrings(p.Types, []string{"vless", "vmess", "trojan"}) {
		t.Errorf("Types = %v，逗号与分号都应视为分隔符", p.Types)
	}
}

func TestParseQuery_Ruleset(t *testing.T) {
	p := mustParse(t, "ruleset=MyGeoSite%3A2%3BMyGeoIP%3A1")
	if p.Ruleset["MyGeoSite"] != "2" || p.Ruleset["MyGeoIP"] != "1" {
		t.Errorf("Ruleset = %v", p.Ruleset)
	}
	if _, err := ParseQuery(url.Values{"ruleset": []string{"没有类型的条目"}}); err == nil {
		t.Error("缺少类型的 ruleset 应报错")
	}
}

func TestParseQuery_UnknownAndExtensionsPreserved(t *testing.T) {
	p := mustParse(t, "custom-foo=bar&x-debug=1&x-emoji-allow=JP")
	if got := p.Raw["custom-foo"]; len(got) != 1 || got[0] != "bar" {
		t.Errorf("未知参数应保留在 Raw: %v", got)
	}
	if len(p.Extensions) != 2 || p.Extensions["x-debug"][0] != "1" {
		t.Errorf("x-* 扩展应进入 Extensions: %v", p.Extensions)
	}
}

func TestBuildPipeline_Semantics(t *testing.T) {
	p := Params{
		Exclude: []string{`^US`},
		Rename:  []transform.RegexPair{{Pattern: " - 10Gbps", Replacement: ""}},
		Emoji:   []transform.RegexPair{{Pattern: "JP", Replacement: "🇯🇵"}},
		Sort:    "name",
	}
	pipe, err := BuildPipeline(p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pipe.Apply(testNodes())
	if err != nil {
		t.Fatal(err)
	}
	// exclude 掉 US；指纹相同的 HK 两节点合并；rename 去掉流量标注；emoji 加国旗。
	want := []string{"HK-香港-01", "🇯🇵JP-日本-01"}
	if !sameStrings(out.Names(), want) {
		t.Errorf("管线输出 = %v，应为 %v", out.Names(), want)
	}
}

// TestBuildPipeline_DedupesByFingerprint 去重只看 server+port+type，名称不参与。
func TestBuildPipeline_DedupesByFingerprint(t *testing.T) {
	ns := model.NodeList{
		model.NewNodeFromParts("DUP1", "ss", "a.example.com", "443", nil),
		model.NewNodeFromParts("DUP2", "ss", "a.example.com", "443", nil),
		model.NewNodeFromParts("DUP3", "ss", "b.example.com", "443", nil),
	}
	pipe, err := BuildPipeline(Params{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := pipe.Apply(ns)
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 2 {
		t.Errorf("应按指纹去重为 2 个节点，实际 %d 个: %v", out.Len(), out.Names())
	}
}

// TestBuildPipeline_TypeFilter 协议白名单与 include/exclude 是"与"关系。
func TestBuildPipeline_TypeFilter(t *testing.T) {
	pipe, err := BuildPipeline(Params{
		Types:   []string{"vless", "vmess"},
		Exclude: []string{`^JP`},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := pipe.Apply(testNodes())
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(out.Names(), []string{"HK-香港-01 - 10Gbps"}) {
		t.Errorf("类型过滤后 = %v，应只剩 vless 且不在黑名单内", out.Names())
	}
	if len(pipe) != 3 {
		t.Errorf("应多出一步类型过滤（filter + type + dedupe），实际 %d 步", len(pipe))
	}
}

func TestBuildPipeline_UnknownSortKey(t *testing.T) {
	if _, err := BuildPipeline(Params{Sort: "color"}); err == nil {
		t.Error("未知排序键应报错")
	}
}

func TestBuildPipeline_NoSortStep(t *testing.T) {
	for _, sort := range []string{"", "original"} {
		pipe, err := BuildPipeline(Params{Sort: sort})
		if err != nil {
			t.Fatalf("sort=%q: %v", sort, err)
		}
		if len(pipe) != 2 {
			t.Errorf("sort=%q 时管线应为 filter + dedupe 两步，实际 %d 步", sort, len(pipe))
		}
	}
}

func TestHasURL(t *testing.T) {
	if mustParse(t, "target=clash").HasURL() {
		t.Error("无 url 时应为 false")
	}
	if !mustParse(t, "url=http://a").HasURL() {
		t.Error("有 url 时应为 true")
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
