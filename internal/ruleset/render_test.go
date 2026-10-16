package ruleset

import (
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// fixtureLines 覆盖全部形态，各格式渲染出的子集与丢弃数都由它决定。
var fixtureLines = []string{
	"DOMAIN,example.com",
	"DOMAIN-KEYWORD,hk",
	"DOMAIN-REGEX,^\\.baidu\\.com$",
	"DOMAIN-SUFFIX,com",
	"GEOIP,CN",
	"GEOSITE,category-ads",
	"IP-CIDR,192.168.0.0/16",
	"MATCH,自动选择",
}

func TestParseFormat(t *testing.T) {
	byNumber := map[string]Format{
		"1": SurgeList, "2": QuantumultX, "3": ClashDomainProvider,
		"4": ClashIPCIDRProvider, "5": SurgeDomainSet, "6": ClashClassical,
	}
	for in, want := range byNumber {
		got, err := ParseFormat(in)
		if err != nil || got != want {
			t.Errorf("ParseFormat(%q) = %v, %v；期望 %v", in, got, err, want)
		}
	}
	byName := map[string]Format{
		"surge":            SurgeList,
		"qx":               QuantumultX,
		"quantumult-x":     QuantumultX,
		"clash-domain":     ClashDomainProvider,
		"clash-ipcidr":     ClashIPCIDRProvider,
		"surge-domain-set": SurgeDomainSet,
		"clash-classical":  ClashClassical,
		"clash":            ClashClassical,
		"CLASH-CLASSICAL":  ClashClassical, // 大小写不敏感
	}
	for in, want := range byName {
		got, err := ParseFormat(in)
		if err != nil || got != want {
			t.Errorf("ParseFormat(%q) = %v, %v；期望 %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "7", "0", "surge3", "clash-meta"} {
		if _, err := ParseFormat(in); err == nil {
			t.Errorf("ParseFormat(%q) 应当报错", in)
		}
	}
}

func TestFormat_Label(t *testing.T) {
	if got := SurgeList.Label(); got != "Surge list" {
		t.Errorf("SurgeList.Label() = %q", got)
	}
	if got := Format(99).Label(); got != "未知" {
		t.Errorf("未知形态的 Label = %q", got)
	}
}

func TestFilename(t *testing.T) {
	cases := []struct {
		name string
		fmt  Format
		want string
	}{
		{"ads", SurgeList, "ads.surge-rules"},
		{"ads", QuantumultX, "ads.qxt"},
		{"geoip", ClashDomainProvider, "geoip.yaml"},
		{"cn", ClashIPCIDRProvider, "cn.yaml"},
		{"ads", SurgeDomainSet, "ads.txt"},
		{"rules", ClashClassical, "rules.yaml"},
		{"sub/geoip.cn", ClashClassical, "geoip.cn.yaml"}, // 只取末段，不生成子目录
		{"", SurgeList, "ruleset.surge-rules"},
	}
	for _, c := range cases {
		got := c.fmt.Filename(c.name)
		if got != c.want {
			t.Errorf("Filename(%q) = %q，期望 %q", c.name, got, c.want)
		}
	}
	if got := Format(99).Filename("x"); got != "ruleset.bin" {
		t.Errorf("未知形态的 Filename = %q", got)
	}
}

func TestRender_PlainTextFormats(t *testing.T) {
	s, err := NewSet(fixtureLines...)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		format   Format
		name     string
		wantBody string
		wantCT   string
		wantOut  int
		wantSkip int
	}{
		{
			SurgeList, "ads",
			"[General]\nCNAME = 1\n\n[Rule]\n" +
				"DOMAIN,example.com\n" +
				"DOMAIN-KEYWORD,hk\n" +
				"DOMAIN-REGEX,^\\.baidu\\.com$\n" +
				"DOMAIN-SUFFIX,com\n",
			"application/x-ruleset; charset=utf-8", 4, 4,
		},
		{
			QuantumultX, "ads",
			"[Header]\n" +
				"name = ads\n" +
				"type = remote\n" +
				"app = Quantumult X\n" +
				"expires = 2592000\n" +
				"\n[Rule]\n" +
				"DOMAIN,example.com\n" +
				"DOMAIN-KEYWORD,hk\n" +
				"DOMAIN-REGEX,^\\.baidu\\.com$\n" +
				"DOMAIN-SUFFIX,com\n" +
				"IP-CIDR,192.168.0.0/16\n",
			"text/plain; charset=utf-8", 5, 3,
		},
		{
			SurgeDomainSet, "ads",
			"example.com\ncom\n",
			"text/plain; charset=utf-8", 2, 6,
		},
	}
	for _, c := range cases {
		got, err := s.Render(c.format, c.name, RenderOpts{})
		if err != nil {
			t.Fatalf("Render(%d): %v", c.format, err)
		}
		if string(got.Body) != c.wantBody {
			t.Errorf("Render(%d) 正文不符\n--- got ---\n%s\n--- want ---\n%s", c.format, got.Body, c.wantBody)
		}
		if got.ContentType != c.wantCT {
			t.Errorf("Render(%d) ContentType = %q，期望 %q", c.format, got.ContentType, c.wantCT)
		}
		if got.Out != c.wantOut || got.Skipped != c.wantSkip || got.In != len(fixtureLines) {
			t.Errorf("Render(%d) In/Out/Skipped = %d/%d/%d，期望 %d/%d/%d",
				c.format, got.In, got.Out, got.Skipped, len(fixtureLines), c.wantOut, c.wantSkip)
		}
	}
}

func TestRender_QXNameFallback(t *testing.T) {
	s, err := NewSet("DOMAIN,a.com")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Render(QuantumultX, "   ", RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got.Body), "name = ruleset\n") {
		t.Errorf("空名字应退回 ruleset：%s", got.Body)
	}
	if got.Filename != "ruleset.qxt" {
		t.Errorf("空名字的文件名 = %q", got.Filename)
	}
}

func TestRender_YAMLFormats(t *testing.T) {
	s, err := NewSet(fixtureLines...)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("domain provider", func(t *testing.T) {
		got, err := s.Render(ClashDomainProvider, "ads", RenderOpts{})
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string][]string
		if err := yaml.Unmarshal(got.Body, &doc); err != nil {
			t.Fatalf("产物不是合法 YAML：%v\n%s", err, got.Body)
		}
		want := []string{"example.com", "hk", `^\.baidu\.com$`, "com"}
		if len(doc) != 1 {
			t.Fatalf("应为唯一键 payload，实际 %d 个", len(doc))
		}
		gotPayload, ok := doc["payload"]
		if !ok {
			t.Fatalf("缺少 payload 键：%s", got.Body)
		}
		if strings.Join(gotPayload, "\n") != strings.Join(want, "\n") {
			t.Errorf("payload = %v，期望 %v", gotPayload, want)
		}
		if !strings.HasPrefix(string(got.Body), "payload:\n") {
			t.Errorf("应以 payload: 开头：%s", got.Body)
		}
		if got.Out != 4 || got.Skipped != 4 {
			t.Errorf("Out/Skipped = %d/%d，期望 4/4", got.Out, got.Skipped)
		}
	})

	t.Run("ipcidr provider", func(t *testing.T) {
		got, err := s.Render(ClashIPCIDRProvider, "cn", RenderOpts{})
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string][]string
		if err := yaml.Unmarshal(got.Body, &doc); err != nil {
			t.Fatalf("产物不是合法 YAML：%v\n%s", err, got.Body)
		}
		if gotPayload, ok := doc["payload"]; !ok || len(gotPayload) != 1 || gotPayload[0] != "192.168.0.0/16" {
			t.Errorf("payload = %v，期望 [192.168.0.0/16]：%s", doc["payload"], got.Body)
		}
		if got.Out != 1 || got.Skipped != 7 {
			t.Errorf("Out/Skipped = %d/%d，期望 1/7", got.Out, got.Skipped)
		}
	})

	t.Run("clash classical", func(t *testing.T) {
		// 缺第三段的规则用默认出站补齐；MATCH 的第二段本来就是出站，不动它。
		got, err := s.Render(ClashClassical, "rules", RenderOpts{Action: "自动选择"})
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string][]string
		if err := yaml.Unmarshal(got.Body, &doc); err != nil {
			t.Fatalf("产物不是合法 YAML：%v\n%s", err, got.Body)
		}
		want := []string{
			"DOMAIN,example.com,自动选择",
			"DOMAIN-KEYWORD,hk,自动选择",
			"DOMAIN-REGEX,^\\.baidu\\.com$,自动选择",
			"DOMAIN-SUFFIX,com,自动选择",
			"GEOIP,CN,自动选择",
			"GEOSITE,category-ads,自动选择",
			"IP-CIDR,192.168.0.0/16,自动选择",
			"MATCH,自动选择",
		}
		if strings.Join(doc["rules"], "\n") != strings.Join(want, "\n") {
			t.Errorf("rules = %v，期望 %v\n%s", doc["rules"], want, got.Body)
		}
		if !strings.HasPrefix(string(got.Body), "rules:\n") {
			t.Errorf("应以 rules: 开头：%s", got.Body)
		}
		if got.Out != 8 || got.Skipped != 0 {
			t.Errorf("Out/Skipped = %d/%d，期望 8/0", got.Out, got.Skipped)
		}
		if got.ContentType != "text/yaml; charset=utf-8" {
			t.Errorf("ContentType = %q", got.ContentType)
		}
	})
}

func TestRender_EmptySet(t *testing.T) {
	got, err := Set{}.Render(ClashIPCIDRProvider, "cn", RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	// 空集要产出合法文档，而不是报错——由接口层决定要不要把它当成 400。
	if !strings.HasPrefix(string(got.Body), "payload:") {
		t.Errorf("空集输出：%s", got.Body)
	}
	if got.In != 0 || got.Out != 0 || got.Skipped != 0 {
		t.Errorf("In/Out/Skipped = %d/%d/%d", got.In, got.Out, got.Skipped)
	}
}

func TestRender_UnknownFormat(t *testing.T) {
	s, err := NewSet("DOMAIN,a.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Render(Format(99), "x", RenderOpts{}); err == nil {
		t.Error("未知形态应当报错")
	}
}

func TestRender_IPv6CIDR(t *testing.T) {
	s, err := NewSet("IP-CIDR,10.0.0.0/8", "IP-CIDR6,2001:db8::/32")
	if err != nil {
		t.Fatal(err)
	}
	// IP-CIDR6 只有能表达 IPv6 网段的形态会收；Surge list 全丢。
	cases := []struct {
		format Format
		out    int
	}{
		{SurgeList, 0},
		{QuantumultX, 2},
		{ClashIPCIDRProvider, 2},
		{ClashClassical, 2},
	}
	for _, c := range cases {
		got, err := s.Render(c.format, "net", RenderOpts{Action: "直连"})
		if err != nil {
			t.Errorf("Render(%d): %v", c.format, err)
			continue
		}
		if got.Out != c.out {
			t.Errorf("Render(%d) Out = %d，期望 %d\n%s", c.format, got.Out, c.out, got.Body)
		}
	}
}

// TestRender_BareCIDRNotMisreadAsDomain 回归：/getruleset 的 list 参数把一批
// 裸规则用分号隔开，里面既有域名也有网段。网段曾被判成 DOMAIN，导致
// ipcidr provider 吐空 payload、clash 经典规则写出 DOMAIN,10.0.0.0/8。
// 这条用例是在 192.168.8.112 上跑真实 HTTP 服务时抓出来的。
func TestRender_BareCIDRNotMisreadAsDomain(t *testing.T) {
	s, err := NewSet("example.com", "google.com", "10.0.0.0/8", "198.18.0.0/15")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(s), 4; got != want {
		t.Fatalf("解析出 %d 条，期望 %d", got, want)
	}

	got, err := s.Render(ClashIPCIDRProvider, "net", RenderOpts{})
	if err != nil {
		t.Fatalf("Render(ipcidr): %v", err)
	}
	if got.Out != 2 {
		t.Errorf("ipcidr provider 应收到 2 条网段，实际 %d\n%s", got.Out, got.Body)
	}
	if !strings.Contains(string(got.Body), "10.0.0.0/8") {
		t.Errorf("ipcidr provider payload 缺网段:\n%s", got.Body)
	}

	// 域名 provider 只能收到域名，网段按 Kind 过滤掉。
	got, err = s.Render(ClashDomainProvider, "dom", RenderOpts{})
	if err != nil {
		t.Fatalf("Render(domain): %v", err)
	}
	if got.Out != 2 {
		t.Errorf("domain provider 应只有 2 条域名，实际 %d\n%s", got.Out, got.Body)
	}

	// clash 经典规则里网段必须标 IP-CIDR，不能标 DOMAIN。
	got, err = s.Render(ClashClassical, "cl", RenderOpts{Action: "proxy"})
	if err != nil {
		t.Fatalf("Render(classical): %v", err)
	}
	body := string(got.Body)
	if !strings.Contains(body, "IP-CIDR,10.0.0.0/8,proxy") {
		t.Errorf("经典规则缺 IP-CIDR 条目:\n%s", body)
	}
	if strings.Contains(body, "DOMAIN,10.0.0.0/8") {
		t.Errorf("网段被错标成 DOMAIN:\n%s", body)
	}
	if got.Out != 4 {
		t.Errorf("经典规则应保留全部 4 条，实际 %d\n%s", got.Out, body)
	}
}

func TestRender_ClassicalRequiresOutbound(t *testing.T) {
	s, err := NewSet("DOMAIN,example.com", "GEOIP,CN,DIRECT")
	if err != nil {
		t.Fatal(err)
	}
	// 内核要求每条规则都有出站，缺了就拒载；我们在这里就报出来。
	if _, err := s.Render(ClashClassical, "cl", RenderOpts{}); !errors.Is(err, ErrMissingAction) {
		t.Errorf("缺出站应报 ErrMissingAction，实际 %v", err)
	}
	// 给默认出站就能补齐，且已经写了出站的规则不被覆盖。
	got, err := s.Render(ClashClassical, "cl", RenderOpts{Action: "自动选择"})
	if err != nil {
		t.Fatalf("给默认出站后仍报错: %v", err)
	}
	for _, want := range []string{"DOMAIN,example.com,自动选择", "GEOIP,CN,DIRECT"} {
		if !strings.Contains(string(got.Body), want) {
			t.Errorf("缺少 %q:\n%s", want, got.Body)
		}
	}
	// 其它形态不强制出站。
	for _, f := range []Format{SurgeList, QuantumultX, SurgeDomainSet, ClashDomainProvider} {
		if _, err := s.Render(f, "cl", RenderOpts{}); err != nil {
			t.Errorf("Render(%d) 不应因缺出站报错: %v", f, err)
		}
	}
}

func TestRender_ActionsPreserved(t *testing.T) {
	s, err := NewSet("DOMAIN,example.com,PROXY", "DOMAIN-SUFFIX,com,DIRECT")
	if err != nil {
		t.Fatal(err)
	}
	// 规则行形态必须保留第三段，否则 Clash 经典规则集粘进配置就是一堆没有出站的死规则。
	for _, f := range []Format{SurgeList, QuantumultX, ClashClassical} {
		got, err := s.Render(f, "act", RenderOpts{})
		if err != nil {
			t.Errorf("Render(%d): %v", f, err)
			continue
		}
		for _, want := range []string{"DOMAIN,example.com,PROXY", "DOMAIN-SUFFIX,com,DIRECT"} {
			if !strings.Contains(string(got.Body), want) {
				t.Errorf("Render(%d) 缺少 %q:\n%s", f, want, got.Body)
			}
		}
	}
	// provider 只表达取值，第三段天然不出现。
	got, err := s.Render(ClashDomainProvider, "act", RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got.Body), "DIRECT") || strings.Contains(string(got.Body), "PROXY") {
		t.Errorf("provider 不应带动作:\n%s", got.Body)
	}
}

func TestRender_ClassicalKeepsAllKinds(t *testing.T) {
	s, err := NewSet("GEOIP,CN,MATCH", "GEOSITE,category-ads,PROXY", "MATCH,自动选择")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Render(ClashClassical, "cl", RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 0 {
		t.Errorf("经典形态应保留全部规则，实际丢了 %d 条", got.Skipped)
	}
	for _, want := range []string{"GEOIP,CN,MATCH", "GEOSITE,category-ads,PROXY", "MATCH,自动选择"} {
		if !strings.Contains(string(got.Body), want) {
			t.Errorf("缺少 %q:\n%s", want, got.Body)
		}
	}
}

func TestRender_FilterThenRender(t *testing.T) {
	s, err := NewSet(fixtureLines...)
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := s.Filter([]string{`^example|openai`}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := filtered.Render(SurgeList, "min", RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Out != 1 || !strings.Contains(string(got.Body), "DOMAIN,example.com") {
		t.Errorf("筛选后渲染不符：%+v\n%s", got, got.Body)
	}
}
