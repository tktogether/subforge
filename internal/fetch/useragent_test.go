package fetch

import (
	"strings"
	"testing"
)

// TestDefaultUserAgent_ClashCompat 校验默认 UA 带上 Clash 内核标识。
//
// 这一条是合约：上游按 UA 决定是否返回规则与 Subscription-Userinfo 头，
// 一旦默认值退化成纯本程序标识，机场订阅的规则与流量信息会静默消失。
func TestDefaultUserAgent_ClashCompat(t *testing.T) {
	t.Setenv(UserAgentEnv, "")
	ua := DefaultUserAgent()
	if !strings.HasPrefix(ua, clashCompatToken) {
		t.Fatalf("默认 UA 应以 %q 开头，实际 %q", clashCompatToken, ua)
	}
	if !strings.Contains(ua, "SubForge/") {
		t.Errorf("默认 UA 应保留本程序标识 SubForge/，实际 %q", ua)
	}
	if !strings.Contains(ua, "github.com/tktogether/subforge") {
		t.Errorf("默认 UA 应带仓库地址，实际 %q", ua)
	}
}

// TestDefaultUserAgent_EnvOverride 校验 SUBFORGE_USER_AGENT 原样覆盖默认值。
func TestDefaultUserAgent_EnvOverride(t *testing.T) {
	const want = "clash-verge/v1.7.7"
	t.Setenv(UserAgentEnv, want)
	if got := DefaultUserAgent(); got != want {
		t.Fatalf("环境变量覆盖后 UA = %q，want %q", got, want)
	}
}

// TestDefaultUserAgent_EnvBlankIgnored 校验空白环境变量不生效。
func TestDefaultUserAgent_EnvBlankIgnored(t *testing.T) {
	t.Setenv(UserAgentEnv, "   ")
	if got := DefaultUserAgent(); !strings.HasPrefix(got, clashCompatToken) {
		t.Fatalf("空白环境变量应回落到默认值，实际 %q", got)
	}
}

// TestDefaultConfig_UsesDefaultUserAgent 校验 DefaultConfig 与默认 UA 保持同源。
func TestDefaultConfig_UsesDefaultUserAgent(t *testing.T) {
	t.Setenv(UserAgentEnv, "")
	if got, want := DefaultConfig().UserAgent, DefaultUserAgent(); got != want {
		t.Fatalf("DefaultConfig().UserAgent = %q，want %q", got, want)
	}
}
