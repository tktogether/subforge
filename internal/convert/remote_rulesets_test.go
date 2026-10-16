package convert

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/config"
)

// TestEngine_RemoteRulesetExpansion 模板声明的远程规则集要按声明位置展开进规则流：
// 插在 MATCH 前面的规则得留在 MATCH 前面，否则展开出的规则全部命中不到。
func TestEngine_RemoteRulesetExpansion(t *testing.T) {
	tpl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a.mrs":
			_, _ = w.Write([]byte("# 说明\nmrs-ver:mrs1.0\n.google.com\nexample.org\n.a.com\n"))
		case "/b.yaml":
			_, _ = w.Write([]byte("payload:\n  - IP-CIDR,10.0.0.0/8\n  - IP-CIDR,1.1.1.1/32,DIRECT,no-resolve\n  - DOMAIN-SUFFIX:example.net\n  - MATCH,DIRECT\n"))
		case "/bad":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			t.Errorf("意外的请求路径 %s", r.URL.Path)
		}
	}))
	t.Cleanup(tpl.Close)

	loader := config.HTTPLoader(&http.Client{Timeout: 5 * time.Second}, nil)
	e := &Engine{Fetcher: nil, RulesetLoader: loader}

	p := Params{Rules: []string{"用户给的规则"}, TemplateRules: []config.RuleItem{
		{Line: "GEOIP,cn,🎯 全球直连,no-resolve", Remote: -1},
		{Remote: 0},
		{Line: "MATCH,🚀 手动选择", Remote: -1},
		{Remote: 1},
		{Remote: 2},
	}, TemplateRemoteRulesets: []config.RemoteRuleset{
		{Group: "🚀 手动选择", URL: tpl.URL + "/a.mrs", Kind: "domain"},
		{Group: "🎯 全球直连", URL: tpl.URL + "/b.yaml", Kind: "classic"},
		{Group: "🎯 全球直连", URL: tpl.URL + "/bad", Kind: "domain"},
	}}

	out, warns := e.expandRemoteRulesets(context.Background(), p)

	// MATCH 必须留在它声明的位置：前面的展开行在前，后面的展开行在后。
	want := []string{
		"用户给的规则",
		"GEOIP,cn,🎯 全球直连,no-resolve",
		"DOMAIN-SUFFIX,google.com,🚀 手动选择",
		"DOMAIN,example.org,🚀 手动选择",
		"DOMAIN-SUFFIX,a.com,🚀 手动选择",
		"MATCH,🚀 手动选择",
		"IP-CIDR,10.0.0.0/8,🎯 全球直连",
		"IP-CIDR,1.1.1.1/32,🎯 全球直连,no-resolve",
		"DOMAIN-SUFFIX,example.net,🎯 全球直连",
	}
	if strings.Join(out.Rules, "|") != strings.Join(want, "|") {
		t.Errorf("规则 =\n%s\n应为\n%s", strings.Join(out.Rules, "\n"), strings.Join(want, "\n"))
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "/bad") {
		t.Errorf("Warnings = %v，应只有失败那份规则集的提示", warns)
	}
	// 展开后不能再留下未展开的引用，否则会被当成中间状态往下传。
	if len(out.TemplateRules) != 0 || len(out.TemplateRemoteRulesets) != 0 {
		t.Errorf("展开后应清空规则流: %v %v", out.TemplateRules, out.TemplateRemoteRulesets)
	}
}

// TestEngine_RemoteRulesetExpansion_AllFail 全部失败时不能阻断转换：
// 规则集拉不下来，订阅本身还是要能转出配置。
func TestEngine_RemoteRulesetExpansion_AllFail(t *testing.T) {
	e := &Engine{Fetcher: nil, RulesetLoader: nil}
	p := Params{TemplateRules: []config.RuleItem{{Remote: 0}}, TemplateRemoteRulesets: []config.RemoteRuleset{
		{Group: "A", URL: "https://127.0.0.1:1/rule.mrs", Kind: "domain"},
	}}
	out, warns := e.expandRemoteRulesets(context.Background(), p)
	if len(warns) != 1 {
		t.Fatalf("Warnings = %v，应为 1 条", warns)
	}
	if len(out.Rules) != 0 {
		t.Errorf("Rules = %v，失败时不应产生规则", out.Rules)
	}
}

func TestExpandMRS(t *testing.T) {
	body := []byte("# 注释\n\nmrs-ver:mrs1.0\n.google.com\nexample.org\n.a.com\n   \n")
	got := expandMRS(body, "组")
	want := []string{
		"DOMAIN-SUFFIX,google.com,组",
		"DOMAIN,example.org,组",
		"DOMAIN-SUFFIX,a.com,组",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got = %v，want = %v", got, want)
	}
}

func TestExpandClassic(t *testing.T) {
	// 公开清单是 YAML 形态：规则写在 payload: 下的序列里。
	body := []byte(`# 头
# TOTAL: 6

payload:
  - DOMAIN-SUFFIX,example.com
  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
  - DOMAIN-REGEX:^[\w\d.:]+$
  - MATCH,DIRECT
  - FINAL,🐟 漏网之鱼
  - 192.168.0.0/16
  - DOMAIN-SUFFIX:alt.example
`)
	got := expandClassic(body, "组")
	want := []string{
		"DOMAIN-SUFFIX,example.com,组",
		"IP-CIDR,10.0.0.0/8,组,no-resolve",
		`DOMAIN-REGEX,^[\w\d.:]+$,组`,
		"IP-CIDR,192.168.0.0/16,组",
		"DOMAIN-SUFFIX,alt.example,组",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got =\n%s\nwant =\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestRulesetProblem(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		bad  bool
	}{
		{"纯文本", []byte("# 头\n.google.com\nexample.org\n"), false},
		{"CRLF", []byte("example.org\r\n.google.com\r\n"), false},
		{"中文域名备注", []byte("# 香港\n.hk.example\n"), false},
		{"NUL", []byte("example.org\x00.google.com"), true},
		{"压缩残留", []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\x03"), true},
		{"控制字符", []byte("example.org\x01\x02.google"), true},
		{"非法UTF8", []byte("example.org\xc0\xc1"), true},
	}
	for _, c := range cases {
		got := rulesetProblem(c.body)
		if (got != "") != c.bad {
			t.Errorf("%s: rulesetProblem = %q，期望异常=%v", c.name, got, c.bad)
		}
	}
}

// TestEngine_RemoteRulesetExpansion_BinaryContent 拉回来的是压缩或编码异常的内容时，
// 必须提示并跳过——写成规则行会产出 mihomo 加载不了的 !!binary 条目。
func TestEngine_RemoteRulesetExpansion_BinaryContent(t *testing.T) {
	tpl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("\x1f\x8b\x08\x00junk\x00\x02"))
	}))
	t.Cleanup(tpl.Close)

	e := &Engine{RulesetLoader: config.HTTPLoader(&http.Client{Timeout: 5 * time.Second}, nil)}
	p := Params{TemplateRules: []config.RuleItem{{Remote: 0}}, TemplateRemoteRulesets: []config.RemoteRuleset{
		{Group: "A", URL: tpl.URL + "/a.mrs", Kind: "domain"},
	}}
	out, warns := e.expandRemoteRulesets(context.Background(), p)
	if len(warns) != 1 || !strings.Contains(warns[0], "内容异常") {
		t.Errorf("Warnings = %v", warns)
	}
	if len(out.Rules) != 0 {
		t.Errorf("Rules = %v，不应产出规则", out.Rules)
	}
}
