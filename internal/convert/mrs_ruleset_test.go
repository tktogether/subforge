package convert

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/parse"
)

// mrsFixturePath 指向 MRS 夹具目录。夹具与参考输出都由 mihomo v1.19.31 生成，
// 见 internal/parse/mrs_test.go 的逐行对拍。
func mrsFixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", "fixtures", "mrs", name)
}

// decodeMRSFixture 读取并解码一份 MRS 夹具。
func decodeMRSFixture(t *testing.T, name string) parse.MRSRuleSet {
	t.Helper()
	buf, err := os.ReadFile(mrsFixturePath(name))
	if err != nil {
		t.Fatalf("读取夹具 %s: %v", name, err)
	}
	set, err := parse.DecodeMRS(buf)
	if err != nil {
		t.Fatalf("解码夹具 %s: %v", name, err)
	}
	return set
}

// TestExpandMRSBinary_Domain 真实 Aethersailor MRS 展开成 DOMAIN-SUFFIX 规则行。
// 夹具里 992 条全是 "+.<domain>" 形态（内核把精确条目与后缀条目合并过），
// 因此展开结果应当全是 DOMAIN-SUFFIX，一条 DOMAIN 都不该有。
func TestExpandMRSBinary_Domain(t *testing.T) {
	set := decodeMRSFixture(t, "Custom_Direct_Domain.mrs")
	const group = "🎯 全球直连"
	got := expandMRSBinary(set, group)
	if len(got) != len(set.Rules) {
		t.Fatalf("展开条数 = %d，want %d", len(got), len(set.Rules))
	}
	if len(got) != 992 {
		t.Errorf("展开条数 = %d，want 992", len(got))
	}
	for i, r := range got {
		if !strings.HasPrefix(r, "DOMAIN-SUFFIX,") {
			t.Fatalf("第 %d 条 = %q，want DOMAIN-SUFFIX 前缀", i+1, r)
		}
		if !strings.HasSuffix(r, ","+group) {
			t.Fatalf("第 %d 条 = %q，want 以 ,%s 结尾", i+1, r, group)
		}
	}
	// 首条与内核参考输出的第一条 "+.00333118.com" 对应。
	if got[0] != "DOMAIN-SUFFIX,00333118.com,"+group {
		t.Errorf("首条 = %q", got[0])
	}
}

// TestExpandMRSBinary_DomainForms 四种域名写法各自的映射：
//
//	+.d → DOMAIN-SUFFIX（含 d 本身）  .d → DOMAIN-SUFFIX（略微放宽）
//	d  → DOMAIN                        *.d → DOMAIN-REGEX（精确表达单标签通配）
func TestExpandMRSBinary_DomainForms(t *testing.T) {
	set := parse.MRSRuleSet{Behavior: parse.MRSBehaviorDomain, Rules: []string{
		"+.a.com", ".b.com", "c.com", "*.d.com",
	}}
	got := expandMRSBinary(set, "G")
	want := []string{
		"DOMAIN-SUFFIX,a.com,G",
		"DOMAIN-SUFFIX,b.com,G",
		"DOMAIN,c.com,G",
		`DOMAIN-REGEX,^[^.]+\.d\.com$,G`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("展开结果 = %v\nwant %v", got, want)
	}
}

// TestExpandMRSBinary_IPCIDR 网段集合按 IPv4 / IPv6 分成 IP-CIDR 与 IP-CIDR6。
// 夹具是 mihomo 用 text → mrs 生成的，参考输出见 cidr-sample.ref.txt。
func TestExpandMRSBinary_IPCIDR(t *testing.T) {
	set := decodeMRSFixture(t, "cidr-sample.mrs")
	got := expandMRSBinary(set, "G")
	var v4, v6 int
	for _, r := range got {
		switch {
		case strings.HasPrefix(r, "IP-CIDR,"):
			v4++
		case strings.HasPrefix(r, "IP-CIDR6,"):
			v6++
		default:
			t.Errorf("非预期规则行 %q", r)
		}
	}
	if v4 != 6 || v6 != 2 {
		t.Errorf("IPv4/IPv6 条数 = %d/%d，want 6/2: %v", v4, v6, got)
	}
	if got[0] != "IP-CIDR,1.0.0.0/24,G" {
		t.Errorf("首条 = %q", got[0])
	}
}

// TestMRSKindMismatch 模板声明与文件内部行为不一致时必须告警（仍然按文件展开），
// 一致或类型未知时不告警。
func TestMRSKindMismatch(t *testing.T) {
	if got := mrsKindMismatch("domain", parse.MRSBehaviorDomain); got != "" {
		t.Errorf("一致时不该告警: %q", got)
	}
	if got := mrsKindMismatch("domain", parse.MRSBehaviorIPCIDR); got == "" {
		t.Error("domain 声明配 ipcidr 文件应告警")
	}
	if got := mrsKindMismatch("classic", parse.MRSBehaviorDomain); got == "" {
		t.Error("classic 声明配 domain 文件应告警")
	}
	if got := mrsKindMismatch("", parse.MRSBehaviorDomain); got != "" {
		t.Errorf("类型未知时不该告警: %q", got)
	}
}

// TestEngine_RemoteMRSBinaryRuleset 端到端：模板的 clash-domain: 指向一份本地 .mrs，
// 产物里必须出现展开后的域名规则，且不能再有"内容异常"告警。
//
// 这是兼容性核查 G1 的回归：修复前二进制内容被文本校验判成"含 NUL"整份丢弃，
// 产物里一条域名规则都没有。
func TestEngine_RemoteMRSBinaryRuleset(t *testing.T) {
	srv := subServer(t, ssLink("10.10.0.1", 8388, "HK-01"))
	body := "[common]\n" +
		"target = clash.meta\n" +
		"\n" +
		"[custom]\n" +
		"custom_proxy_group=🚀 手动选择`select`.*\n" +
		"ruleset=🚀 手动选择,clash-domain:" + mrsFixturePath("Custom_Direct_Domain.mrs") + ",1800\n" +
		"ruleset=🚀 手动选择,[]FINAL\n"
	e := withTemplate(t, newEngine(t), body)

	res, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srv.URL},
		Raw:    map[string][]string{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, w := range res.Warnings {
		if strings.Contains(w, "内容异常") || strings.Contains(w, "解码失败") {
			t.Errorf("不应再有规则集异常告警: %s", w)
		}
	}
	out := string(res.Body)
	if n := strings.Count(out, "DOMAIN-SUFFIX,"); n != 992 {
		t.Errorf("产物里 DOMAIN-SUFFIX 条数 = %d，want 992", n)
	}
	if !strings.Contains(out, "DOMAIN-SUFFIX,00333118.com,") {
		t.Error("产物缺少展开后的域名规则")
	}
	if strings.Contains(out, ".mrs") {
		t.Error("产物不应残留 .mrs 引用")
	}
}
