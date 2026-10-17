package convert

import (
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/model"
)

func TestApplyProxyOverrides(t *testing.T) {
	raw1 := map[string]any{"name": "HK-01", "type": "vmess", "server": "hk.example.com", "port": "443"}
	raw2 := map[string]any{"name": "JP-01", "type": "vmess", "server": "jp.example.com", "port": "8443"}
	list := model.NodeList{
		model.NodeFromRaw(raw1, "sub"),
		model.NodeFromRaw(raw2, "sub"),
	}
	overrides := []config.ProxyOverride{
		{Name: "HK", Server: "new-hk.example.com", Port: "8443"},
		{Name: "US", Server: "us.example.com"},
	}
	warns, applied := applyProxyOverrides(list, overrides)
	if applied != 1 {
		t.Fatalf("应覆盖 1 个节点，实际 %d", applied)
	}
	hk := list[0]
	if hk.Server != "new-hk.example.com" || hk.Port != "8443" {
		t.Errorf("HK 节点应被改写: server=%q port=%q", hk.Server, hk.Port)
	}
	if hk.Name != "HK-01" || hk.Type != "vmess" || hk.Source != "sub" {
		t.Errorf("不应变字段被破坏: %+v", hk)
	}
	// US 覆盖没有匹配任何节点，应记入告警。
	if len(warns) != 1 || !strings.Contains(warns[0], "US") {
		t.Fatalf("未匹配的覆盖应告警: %v", warns)
	}
}

func TestApplyProxyOverrides_NoMatchNoWarnWhenEmpty(t *testing.T) {
	list := model.NodeList{model.NodeFromRaw(map[string]any{"name": "A"}, "s")}
	warns, applied := applyProxyOverrides(list, nil)
	if warns != nil || applied != 0 {
		t.Fatalf("空覆盖不应有任何动作: warns=%v applied=%d", warns, applied)
	}
}
