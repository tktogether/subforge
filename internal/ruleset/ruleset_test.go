package ruleset

import (
	"strings"
	"testing"
)

func TestParseLine(t *testing.T) {
	cases := []struct {
		line   string
		kind   Kind
		value  string
		action string
		errStr string
	}{
		{`DOMAIN,example.com`, Domain, "example.com", "", ""},
		{`domain-suffix,com`, DomainSuffix, "com", "", ""},       // 形态名大小写不敏感
		{` DOMAIN-KEYWORD ,  hk  `, DomainKeyword, "hk", "", ""}, // 两端空白被吃掉
		{`DOMAIN-REGEX,^\.baidu\.com$`, DomainRegex, `^\.baidu\.com$`, "", ""},
		{`IP-CIDR,192.168.0.0/16`, IPCIDR, "192.168.0.0/16", "", ""},
		{`GEOIP,CN`, GeoIP, "CN", "", ""},
		{`GEOIP,CN,DIRECT`, GeoIP, "CN", "DIRECT", ""}, // 第三段是出站
		{`GEOSITE,category-ads`, GeoSite, "category-ads", "", ""},
		{`MATCH,自动选择`, Match, "自动选择", "", ""},
		{`FINAL,自动选择`, Final, "自动选择", "", ""},
		{`DOMAIN,example.com,PROXY`, Domain, "example.com", "PROXY", ""}, // 第三段是动作
		{`example.com`, Domain, "example.com", "", ""},                   // 裸域名按 DOMAIN 处理
		// Surge 的 "example.com,PROXY" 不带类型前缀，刻意不猜：把第一段当域名会让
		// "DOMAIN.SUFFIX,com" 这类笔误静默变成 DOMAIN。写清楚前缀。
		{`example.com,PROXY`, Domain, "", "", "未知规则形态"},
		{`ip-cidr6,2001:db8::/32`, IPCIDR6, "2001:db8::/32", "", ""},
		{`geosite,geolocation-!cn`, GeoSite, "geolocation-!cn", "", ""},
		{`FOO,bar`, Domain, "", "", "未知规则形态"},
		{`DOMAIN,`, Domain, "", "", "取值不能为空"},
		{`DOMAIN-REGEX,(`, Domain, "", "", "正则无效"},
		{`DOMAIN-REGEX,`, Domain, "", "", "取值不能为空"},
		{``, Domain, "", "", "无法解析"},
		{`- not a domain`, Domain, "", "", "无法解析"},
	}
	for _, c := range cases {
		got, err := ParseLine(c.line)
		if c.errStr != "" {
			if err == nil {
				t.Errorf("ParseLine(%q) 期望报错含 %q，得到 %v", c.line, c.errStr, got)
			} else if !strings.Contains(err.Error(), c.errStr) {
				t.Errorf("ParseLine(%q) 错误信息 %q 不含 %q", c.line, err, c.errStr)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseLine(%q) 意外报错: %v", c.line, err)
			continue
		}
		want := Rule{Kind: c.kind, Value: c.value, Action: c.action}
		if got != want {
			t.Errorf("ParseLine(%q) = %+v，期望 %+v", c.line, got, want)
		}
	}
}

func TestRuleString(t *testing.T) {
	cases := []struct {
		rule Rule
		want string
	}{
		{Rule{Kind: DomainSuffix, Value: "com"}, "DOMAIN-SUFFIX,com"},
		{Rule{Kind: GeoIP, Value: "CN", Action: "DIRECT"}, "GEOIP,CN,DIRECT"},
		{Rule{Kind: Match, Value: "自动选择", Action: "PROXY"}, "MATCH,自动选择,PROXY"},
	}
	for _, c := range cases {
		if got := c.rule.String(); got != c.want {
			t.Errorf("Rule.String() = %q，期望 %q", got, c.want)
		}
	}
}

func TestNewSet_Normalizes(t *testing.T) {
	s, err := NewSet(
		"GEOIP,CN",
		"DOMAIN,example.com",
		"DOMAIN,example.com", // 重复
		"MATCH,自动选择",
		"IP-CIDR,10.0.0.0/8",
		"",
	)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	want := []Rule{
		{Kind: Domain, Value: "example.com"},
		{Kind: GeoIP, Value: "CN"},
		{Kind: IPCIDR, Value: "10.0.0.0/8"},
		{Kind: Match, Value: "自动选择"},
	}
	if s.Count() != len(want) {
		t.Fatalf("归并后条数 = %d，期望 %d: %+v", s.Count(), len(want), s)
	}
	for i, r := range want {
		if s[i] != r {
			t.Errorf("第 %d 条 = %v，期望 %v", i, s[i], r)
		}
	}
}

func TestNormalize_SortsByKindThenValue(t *testing.T) {
	s, err := NewSet("DOMAIN-SUFFIX,com", "DOMAIN-SUFFIX,ai", "DOMAIN,example.com")
	if err != nil {
		t.Fatal(err)
	}
	want := []Rule{
		{Kind: Domain, Value: "example.com"},
		{Kind: DomainSuffix, Value: "ai"},
		{Kind: DomainSuffix, Value: "com"},
	}
	for i, r := range want {
		if s[i] != r {
			t.Errorf("第 %d 条 = %v，期望 %v", i, s[i], r)
		}
	}
}

func TestDedup_KeepsOrder(t *testing.T) {
	// NewSet 已经归并排序，这里直接构造有序输入来验证 Dedup 只做保序去重。
	in := Set{
		{Kind: Match, Value: "直连"},
		{Kind: Domain, Value: "a.com"},
		{Kind: Match, Value: "直连"},
		{Kind: Domain, Value: "a.com"},
	}
	got := in.Dedup()
	if got.Count() != 2 {
		t.Fatalf("Dedup 后条数 = %d，期望 2: %+v", got.Count(), got)
	}
	if got[0] != in[0] || got[1] != in[1] {
		t.Errorf("Dedup 未保持原顺序: %+v", got)
	}
}

func TestFilter(t *testing.T) {
	s, err := NewSet(
		"DOMAIN,ai.openai.com",
		"DOMAIN,mail.google.com",
		"DOMAIN-SUFFIX,openai.com",
		"IP-CIDR,9.9.9.9/32",
	)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		include []string
		exclude []string
		want    int
	}{
		{"无过滤保留全部", nil, nil, 4},
		{"仅域名形态", []string{`openai`}, nil, 2},
		{"剔除域名集", nil, []string{`openai`}, 2},
		{"先剔再加", []string{`\.com$`}, []string{`openai`}, 1},
		{"全被剔除", nil, []string{`.`, `IP-CIDR`}, 0},
	}
	for _, c := range cases {
		got, err := s.Filter(c.include, c.exclude)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got.Count() != c.want {
			t.Errorf("%s: 得到 %d 条 %+v，期望 %d 条", c.name, got.Count(), got, c.want)
		}
	}

	if _, err := s.Filter([]string{`(`}, nil); err == nil {
		t.Error("include 给出坏正则应当报错")
	}
	if _, err := s.Filter(nil, []string{`(`}); err == nil {
		t.Error("exclude 给出坏正则应当报错")
	}
}

func TestSet_Empty(t *testing.T) {
	empty := Set{}
	if !empty.Empty() {
		t.Error("空集应判定为 Empty")
	}
	s, err := NewSet("DOMAIN,a.com")
	if err != nil || s.Empty() {
		t.Errorf("非空集被判为空 (err=%v)", err)
	}
}
