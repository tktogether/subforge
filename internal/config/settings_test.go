package config

import (
	"testing"

	"github.com/tktogether/subforge/internal/template"
)

func mustParse(t *testing.T, s string) *template.Config {
	t.Helper()
	cfg, err := template.ParseString(s, "test.ini")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestFromINI_DefaultsWhenNil(t *testing.T) {
	d := Default()
	if got := FromINI(nil, d); got != d {
		t.Errorf("nil 配置应原样返回默认值: %+v", got)
	}
	if d.MixedPort != DefaultMixedPort {
		t.Errorf("默认 mixed-port = %d", d.MixedPort)
	}
}

func TestFromINI_FallsBackToDefaults(t *testing.T) {
	d := Settings{Token: "cli-token", MixedPort: 9090, Name: "cli-name"}
	got := FromINI(mustParse(t, "[其他段]\nx = 1\n"), d)
	if got != d {
		t.Errorf("没有配置段落时应沿用命令行值: %+v", got)
	}
}

func TestFromINI_SubForgeSection(t *testing.T) {
	d := Default()
	got := FromINI(mustParse(t, "[subforge]\ntoken = abc\nmixed_port = 1080\nname = 我的配置\n"), d)
	if got.Token != "abc" || got.MixedPort != 1080 || got.Name != "我的配置" {
		t.Errorf("解析结果 = %+v", got)
	}
}

// TestFromINI_CommonTokenFallback token 允许按 subconverter 习惯写在 [common]。
func TestFromINI_CommonTokenFallback(t *testing.T) {
	got := FromINI(mustParse(t, "[common]\ntoken = secret\n"), Default())
	if got.Token != "secret" {
		t.Errorf("token = %q", got.Token)
	}
	if got.MixedPort != DefaultMixedPort {
		t.Errorf("未配置的 mixed-port 应保持默认: %d", got.MixedPort)
	}
}

func TestFromINI_SubForgeSectionWins(t *testing.T) {
	got := FromINI(mustParse(t, "[common]\ntoken = old\n[subforge]\ntoken = new\n"), Default())
	if got.Token != "new" {
		t.Errorf("[subforge] 应优先于 [common]: %q", got.Token)
	}
}

// TestFromINI_BadMixedPortIgnored 非法端口不能把服务搞坏，退回默认。
func TestFromINI_BadMixedPortIgnored(t *testing.T) {
	cases := map[string]string{
		"不是数字": "abc",
		"零":    "0",
		"负数":   "-1",
		"超范围":  "99999",
		"空值":   "   ",
	}
	for name, raw := range cases {
		got := FromINI(mustParse(t, "[subforge]\nmixed_port = "+raw+"\n"), Default())
		if got.MixedPort != DefaultMixedPort {
			t.Errorf("%s (%q): mixed-port = %d，应退回默认", name, raw, got.MixedPort)
		}
	}
}

func TestFromINI_PartialOverride(t *testing.T) {
	d := Settings{Token: "keep", MixedPort: 1080}
	got := FromINI(mustParse(t, "[subforge]\nmixed_port = 7891\n"), d)
	if got.Token != "keep" {
		t.Errorf("未配置项应保留原值: %+v", got)
	}
	if got.MixedPort != 7891 {
		t.Errorf("已配置项应被覆盖: %+v", got)
	}
}
