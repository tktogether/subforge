package ini_rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/fetch"
)

// stubFetcher 让测试完全掌控"远程规则集"返回什么，并记录调用方发来的源。
// 用它而不是 httptest：内联形态根本不该发请求，这一点要能被断言。
type stubFetcher struct {
	bodies  [][]byte
	err     error
	sources []fetch.Source
	calls   int
}

func (s *stubFetcher) Fetch(_ context.Context, sources []fetch.Source) ([]fetch.Result, error) {
	s.calls++
	s.sources = append([]fetch.Source(nil), sources...)
	res := make([]fetch.Result, 0, len(sources))
	for i, src := range sources {
		out := fetch.Result{Source: src}
		if i < len(s.bodies) {
			out.Content = append([]byte(nil), s.bodies[i]...)
		}
		res = append(res, out)
	}
	return res, s.err
}

// --- 内联形态解析 ---

// TestParse_RulesetForms 覆盖 ruleset= 的全部形态解析。四种形态取自任务书：
// 内联 []GEOSITE / []GEOIP / []FINAL，远程 kind:URL[,interval]。
// 真实文件（Aethersailor Custom_Clash_Full.ini）56 条里有 50 条是前三种。
func TestParse_RulesetForms(t *testing.T) {
	cases := []struct {
		name     string
		val      string
		group    string
		kind     string
		body     string
		flag     string
		url      string
		interval int
		inline   bool
	}{
		{
			name: "内联GEOSITE", val: "🎯 全球直连,[]GEOSITE,private",
			group: "🎯 全球直连", kind: "GEOSITE", body: "private", inline: true,
		},
		{
			name: "内联GEOIP带标志", val: "🎯 全球直连,[]GEOIP,private,no-resolve",
			group: "🎯 全球直连", kind: "GEOIP", body: "private", flag: "no-resolve", inline: true,
		},
		{
			name: "内联FINAL", val: "🐟 漏网之鱼,[]FINAL",
			group: "🐟 漏网之鱼", kind: "FINAL", inline: true,
		},
		{
			name: "远程domain带interval", val: "🛑 广告拦截,clash-domain:https://example.com/a.mrs,1800",
			group: "🛑 广告拦截", kind: "domain", url: "https://example.com/a.mrs", interval: 1800,
		},
		{
			name: "远程classic带interval", val: "🍃 应用净化,clash-classic:https://example.com/b.yaml,28800",
			group: "🍃 应用净化", kind: "classic", url: "https://example.com/b.yaml", interval: 28800,
		},
		{
			name: "远程interval等号写法", val: "G,clash-domain:https://example.com/c.list,interval=3600",
			group: "G", kind: "domain", url: "https://example.com/c.list", interval: 3600,
		},
		{
			name: "老式裸URL", val: "G,https://example.com/d.list",
			group: "G", url: "https://example.com/d.list",
		},
		{
			// 组名缺省的单段写法：整段是规则集声明，不能整段当 URL。
			name: "单段远程形态声明", val: "clash-domain:https://example.com/e.list",
			kind: "domain", url: "https://example.com/e.list",
		},
		{
			name: "单段裸URL", val: "https://example.com/f.list",
			url: "https://example.com/f.list",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr, err := parseRuleset(tc.val, 1)
			if err != nil {
				t.Fatalf("parseRuleset(%q): %v", tc.val, err)
			}
			if rr.Group != tc.group {
				t.Errorf("Group = %q, want %q", rr.Group, tc.group)
			}
			if rr.Kind != tc.kind {
				t.Errorf("Kind = %q, want %q", rr.Kind, tc.kind)
			}
			if rr.Body != tc.body {
				t.Errorf("Body = %q, want %q", rr.Body, tc.body)
			}
			if rr.Flag != tc.flag {
				t.Errorf("Flag = %q, want %q", rr.Flag, tc.flag)
			}
			if rr.URL != tc.url {
				t.Errorf("URL = %q, want %q", rr.URL, tc.url)
			}
			if rr.Interval != tc.interval {
				t.Errorf("Interval = %d, want %d", rr.Interval, tc.interval)
			}
			if rr.IsInline() != tc.inline {
				t.Errorf("IsInline = %v, want %v", rr.IsInline(), tc.inline)
			}
		})
	}
}

func TestParse_RulesetInlineErrors(t *testing.T) {
	cases := []struct {
		val  string
		want string
	}{
		{"G,[]GEOSITE", "缺取值"},
		{"G,[]GEOIP", "缺取值"},
		{"G,[]", "缺形态名"},
		{"G,[]DOMAIN,example.com", "不支持"},
		// 内联形态没法从清单内容里找命中组，组名必填。
		{"[]FINAL", "缺组名"},
		{"[]GEOSITE,private", "缺组名"},
	}
	for _, tc := range cases {
		_, err := parseRuleset(tc.val, 7)
		if err == nil {
			t.Fatalf("parseRuleset(%q) 期待报错", tc.val)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("parseRuleset(%q) err = %v, want 含 %q", tc.val, err, tc.want)
		}
		if !strings.Contains(err.Error(), "7") {
			t.Errorf("parseRuleset(%q) err = %v, want 含行号 7", tc.val, err)
		}
	}
}

// --- 内联展开 ---

// TestExpand_InlineForms 三种内联形态的展开结果与顺序。
// []FINAL → MATCH 是硬要求：mihomo 的 rules 解析器没有 FINAL 分支
// （docs/compatibility.md:449 K2），产出 FINAL 会让规则集加载失败。
func TestExpand_InlineForms(t *testing.T) {
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "🎯 全球直连", Kind: "GEOSITE", Body: "private"},
		{Group: "🎯 全球直连", Kind: "GEOIP", Body: "private", Flag: "no-resolve"},
		{Group: "📲 电报消息", Kind: "GEOIP", Body: "telegram"},
		{Group: "🐟 漏网之鱼", Kind: "FINAL"},
	}}
	stub := &stubFetcher{}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// GEOIP 未写标志位时补 no-resolve（与 internal/config/custom.go 同口径）。
	want := []string{
		"GEOSITE,private,🎯 全球直连",
		"GEOIP,private,🎯 全球直连,no-resolve",
		"GEOIP,telegram,📲 电报消息,no-resolve",
		"MATCH,🐟 漏网之鱼",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// 内联形态不该联网：一个请求都不许发。
	if stub.calls != 0 {
		t.Fatalf("内联展开不应调用 fetcher，实际调用 %d 次（sources=%+v）", stub.calls, stub.sources)
	}
}

// TestExpand_InlineMixedWithRemote_Order 内联与远程混排时，规则必须按 ini 声明顺序输出。
// MATCH 之前的规则集不能被挪到后面——mihomo 按顺序匹配，挪位会让后面的规则变成死规则。
func TestExpand_InlineMixedWithRemote_Order(t *testing.T) {
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "🎯 全球直连", Kind: "GEOSITE", Body: "private"},
		{Group: "🍃 应用净化", Kind: "classic", URL: "https://example.com/classic.yaml"},
		{Group: "📲 电报消息", Kind: "GEOIP", Body: "telegram", Flag: "no-resolve"},
		{Group: "🐟 漏网之鱼", Kind: "FINAL"},
	}}
	stub := &stubFetcher{bodies: [][]byte{[]byte("payload:\n  - DOMAIN-SUFFIX,example.com,DIRECT\n")}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		"GEOSITE,private,🎯 全球直连",
		"DOMAIN-SUFFIX,example.com,🍃 应用净化",
		"GEOIP,telegram,📲 电报消息,no-resolve",
		"MATCH,🐟 漏网之鱼",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
	// 只对远程引用发请求，内联引用不能混进 fetch.Source。
	if len(stub.sources) != 1 || stub.sources[0].URL != "https://example.com/classic.yaml" {
		t.Fatalf("sources = %+v, want 仅远程引用", stub.sources)
	}
	if stub.sources[0].Name != "🍃 应用净化" {
		t.Fatalf("source name = %q, want 组名", stub.sources[0].Name)
	}
}

// --- 远程 clash-domain / clash-classic 展开 ---

// TestExpand_RemoteDomainList 裸域名清单（subconverter clash-domain 的文本形态）。
func TestExpand_RemoteDomainList(t *testing.T) {
	body := []byte("# comment\nmrs-ver:1\nexample.com\n.example.org\n+.suffix.net\nDOMAIN-KEYWORD,ads,DIRECT\n")
	stub := &stubFetcher{bodies: [][]byte{body}}
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "G", Kind: "domain", URL: "https://example.com/domains.list"},
	}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		"DOMAIN,example.com,G",
		"DOMAIN-SUFFIX,example.org,G",
		"DOMAIN-SUFFIX,suffix.net,G",
		"DOMAIN-KEYWORD,ads,G",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestExpand_RemoteClassicList Clash 经典清单（含 YAML 序列写法）。
// DST-PORT / SRC-PORT 这类 mihomo 支持的形态必须保留——internal/ruleset 的形态表
// 不覆盖它们，按它校验会静默丢规则（Aethersailor 的 Custom_Direct_Classical_IP.yaml
// 里就有这两条）。
func TestExpand_RemoteClassicList(t *testing.T) {
	body := []byte(`# Generated from rule/Custom_Direct.list
payload:
  - DST-PORT,7844
  - SRC-PORT,41641
  - IP-CIDR,108.174.48.100/32,no-resolve
  - IP-CIDR6,2001:db8::/32
  - DOMAIN-SUFFIX,example.com,DIRECT
  - MATCH,🐟 漏网之鱼
`)
	stub := &stubFetcher{bodies: [][]byte{body}}
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "🍃 应用净化", Kind: "classic", URL: "https://example.com/classic.yaml"},
	}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// MATCH 行必须跳过：它的第二段已经是出站名，再塞组名会变成三段非法规则。
	want := []string{
		"DST-PORT,7844,🍃 应用净化",
		"SRC-PORT,41641,🍃 应用净化",
		"IP-CIDR,108.174.48.100/32,🍃 应用净化,no-resolve",
		"IP-CIDR6,2001:db8::/32,🍃 应用净化",
		"DOMAIN-SUFFIX,example.com,🍃 应用净化",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestExpand_LegacyNoGroup_Passthrough 老式 `ruleset=<URL>`（没声明命中组）：
// 没有组名可补，清单原样保留——补空组会生成 "DOMAIN,x," 这种末尾空段的非法规则行。
// 这条钉住改动前的兼容行为。
func TestExpand_LegacyNoGroup_Passthrough(t *testing.T) {
	body := []byte(`# comment
DOMAIN-SUFFIX,a.com,PROXY
IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
IP-CIDR6,2001:db8::/32,DIRECT
`)
	stub := &stubFetcher{bodies: [][]byte{body}}
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{URL: "https://example.com/legacy.list"},
	}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		"DOMAIN-SUFFIX,a.com,PROXY",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR6,2001:db8::/32,DIRECT",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, line := range got {
		if strings.HasSuffix(line, ",") {
			t.Fatalf("产出了末尾空组的非法规则行: %q", line)
		}
	}
}

// TestExpand_RemoteMRS_Domain 真实 MRS 二进制（zstd）规则集。
// 夹具与 internal/parse 的基准测试共用，取自 mihomo v1.19.31 的 convert-ruleset 产出。
func TestExpand_RemoteMRS_Domain(t *testing.T) {
	body := readFixture(t, "Custom_Direct_Domain.mrs")
	stub := &stubFetcher{bodies: [][]byte{body}}
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "🎯 全球直连", Kind: "domain", URL: "https://example.com/direct.mrs"},
	}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("MRS 规则集未展开出任何规则")
	}
	// dumpDomainSet 把精确条目与后缀条目合并成 "+.d"，这里统一展开成 DOMAIN-SUFFIX。
	if got[0] != "DOMAIN-SUFFIX,00333118.com,🎯 全球直连" {
		t.Fatalf("got[0] = %q", got[0])
	}
	for _, line := range got {
		if strings.Contains(line, "\x00") || strings.Contains(line, "\ufffd") {
			t.Fatalf("展开结果混入乱码: %q", line)
		}
		if !strings.HasPrefix(line, "DOMAIN,") && !strings.HasPrefix(line, "DOMAIN-SUFFIX,") {
			t.Fatalf("意外的规则形态: %q", line)
		}
	}
	// 夹具 ref 有 992 条，合并后仍应是全量展开，不能出现"只展开了一部分"。
	if len(got) != 992 {
		t.Fatalf("got %d 条，want 992", len(got))
	}
}

// TestExpand_RemoteMRS_IPCIDR ipcidr 行为的 MRS → IP-CIDR / IP-CIDR6。
func TestExpand_RemoteMRS_IPCIDR(t *testing.T) {
	body := readFixture(t, "cidr-sample.mrs")
	stub := &stubFetcher{bodies: [][]byte{body}}
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		// 声明成 classic 也照文件内容展开：形态以文件为准。
		{Group: "🧱 直连IP", Kind: "classic", URL: "https://example.com/cidr.mrs"},
	}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{
		"IP-CIDR,1.0.0.0/24,🧱 直连IP",
		"IP-CIDR,10.0.0.0/8,🧱 直连IP",
		"IP-CIDR,172.16.0.0/12,🧱 直连IP",
		"IP-CIDR,192.168.0.0/16,🧱 直连IP",
		"IP-CIDR,198.51.100.0/24,🧱 直连IP",
		"IP-CIDR,203.0.113.0/24,🧱 直连IP",
		"IP-CIDR6,2001:db8::/32,🧱 直连IP",
		"IP-CIDR6,2400:cb00::/32,🧱 直连IP",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// --- 部分失败 ---

// TestExpand_PartialFail_OneRemoteDown 一条远程失败、另一条成功：不能报错，
// 失败的引用跳过、其余按顺序展开。这条与 TestExpand_Custom_AllFail 一起钉住失败口径：
// 只有"远程一条都没成功"才整份失败。
func TestExpand_PartialFail_OneRemoteDown(t *testing.T) {
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "🎯 全球直连", Kind: "GEOSITE", Body: "private"},
		{Group: "🍃 应用净化", Kind: "classic", URL: "https://example.com/ok.yaml"},
		{Group: "🛑 广告拦截", Kind: "classic", URL: "http://127.0.0.1:1/nope"},
		{Group: "🐟 漏网之鱼", Kind: "FINAL"},
	}}
	stub := &stubFetcher{bodies: [][]byte{[]byte("payload:\n  - DOMAIN-SUFFIX,example.com,DIRECT\n"), nil}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("部分失败不应报错: %v", err)
	}
	want := []string{
		"GEOSITE,private,🎯 全球直连",
		"DOMAIN-SUFFIX,example.com,🍃 应用净化",
		"MATCH,🐟 漏网之鱼",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestExpand_AllRemoteFail_Error 远程全失败（内联存在与否都是）必须报错，
// 否则会写出"只有几条内联规则"的残缺规则集，用户以为是更新成功了。
func TestExpand_AllRemoteFail_Error(t *testing.T) {
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "G", Kind: "classic", URL: "http://127.0.0.1:1/nope"},
		{Group: "H", URL: "http://127.0.0.1:1/nope2"},
	}}
	stub := &stubFetcher{bodies: [][]byte{nil, nil}}
	_, err := Expand(context.Background(), doc, stub)
	if err == nil {
		t.Fatal("远程全失败应报错")
	}
	if !strings.Contains(err.Error(), "全部 2 个 ruleset 拉取失败") {
		t.Fatalf("err = %v", err)
	}
}

// TestExpand_RemoteProblemContent_Error 二进制乱码被当成文本规则集时要报错，
// 不能把控制字符写进产物（YAML 会把它编码成 !!binary）。
func TestExpand_RemoteProblemContent_Error(t *testing.T) {
	stub := &stubFetcher{bodies: [][]byte{[]byte("DOMAIN,a.com\x00\x01")}}
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "G", Kind: "classic", URL: "https://example.com/x.yaml"},
	}}
	_, err := Expand(context.Background(), doc, stub)
	if err == nil || !strings.Contains(err.Error(), "内容异常") {
		t.Fatalf("err = %v, want 内容异常", err)
	}
}

// --- 真实结构夹具：subconverter 内联写法走完 Parse → Expand ---

// TestParse_SubconverterInlineFixture 用一份按真实文件结构写的夹具走完整链路。
// 夹具缩到 6 条，覆盖四种形态与它们的相对顺序（[]FINAL 在末尾）。
func TestParse_SubconverterInlineFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "subconverter_inline.ini"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Form != FormCustom {
		t.Fatalf("Form = %v, want FormCustom", doc.Form)
	}
	if len(doc.RulesetRefs) != 6 {
		t.Fatalf("refs = %d, want 6", len(doc.RulesetRefs))
	}
	kinds := make([]string, 0, len(doc.RulesetRefs))
	for _, rr := range doc.RulesetRefs {
		kinds = append(kinds, rr.Kind)
	}
	wantKinds := []string{"GEOSITE", "GEOIP", "domain", "classic", "GEOIP", "FINAL"}
	if strings.Join(kinds, ",") != strings.Join(wantKinds, ",") {
		t.Fatalf("kinds = %v, want %v", kinds, wantKinds)
	}
	if doc.RulesetRefs[2].Interval != 1800 || doc.RulesetRefs[3].Interval != 28800 {
		t.Fatalf("interval = %d/%d", doc.RulesetRefs[2].Interval, doc.RulesetRefs[3].Interval)
	}

	stub := &stubFetcher{bodies: [][]byte{
		readFixture(t, "cidr-sample.mrs"),
		[]byte("payload:\n  - DST-PORT,7844\n  - DOMAIN-SUFFIX,example.com,DIRECT\n"),
	}}
	got, err := Expand(context.Background(), doc, stub)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(stub.sources) != 2 {
		t.Fatalf("sources = %d, want 2（只应拉远程的 2 条）", len(stub.sources))
	}
	if !strings.HasPrefix(got[0], "GEOSITE,private,") {
		t.Fatalf("got[0] = %q", got[0])
	}
	if got[len(got)-1] != "MATCH,🐟 漏网之鱼" {
		t.Fatalf("最后一条 = %q, want MATCH（[]FINAL 必须收尾）", got[len(got)-1])
	}
	// 远程两条的展开结果按声明顺序插回原位：MRS 的 IP-CIDR 在 classic 之前。
	idxIPCIDR := indexOfPrefix(got, "IP-CIDR,1.0.0.0/24,")
	idxPort := indexOfPrefix(got, "DST-PORT,7844,")
	if idxIPCIDR < 0 || idxPort < 0 || idxIPCIDR > idxPort {
		t.Fatalf("远程展开顺序错误: ipcidr=%d port=%d\n%s", idxIPCIDR, idxPort, strings.Join(got, "\n"))
	}
	if !strings.Contains(strings.Join(got, "\n"), "GEOIP,telegram,📲 电报消息,no-resolve") {
		t.Fatalf("缺少 GEOIP 内联规则:\n%s", strings.Join(got, "\n"))
	}
}

// readFixture 读取 testdata/fixtures 下的共享夹具（与 internal/parse 共用）。
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", name))
	if err != nil {
		t.Fatalf("读取夹具 %s: %v", name, err)
	}
	return b
}

func indexOfPrefix(lines []string, prefix string) int {
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return i
		}
	}
	return -1
}
