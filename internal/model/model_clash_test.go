package model

import (
	"strings"
	"testing"
)

func TestParseClashProxies_More(t *testing.T) {
	yamlBuf := []byte("proxies:\n  - {name: a, type: ss}\n  - {name: b, type: vmess}\n")
	got, err := ParseClashProxies(yamlBuf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0]["name"] != "a" {
		t.Errorf("name = %v", got[0]["name"])
	}
	for _, in := range [][]byte{[]byte("{}"), []byte("proxies: []")} {
		_, err := ParseClashProxies(in)
		if err == nil {
			t.Fatalf("expected err for %s", in)
		}
		if err.Error() != "配置中不含 proxies" {
			t.Errorf("err = %q", err.Error())
		}
	}
	_, err = ParseClashProxies([]byte("proxies: ["))
	if err == nil {
		t.Fatal("expected err")
	}
	if !strings.Contains(err.Error(), "解析 YAML") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestParseClashRules_More(t *testing.T) {
	yamlBuf := []byte("rules:\n  - DOMAIN,a,PROXY\n  - MATCH,DIRECT\n")
	got, err := ParseClashRules(yamlBuf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0] != "DOMAIN,a,PROXY" {
		t.Errorf("got[0] = %q", got[0])
	}
	got2, err := ParseClashRules([]byte("{}"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got2) != 0 {
		t.Errorf("len = %d", len(got2))
	}
	_, err = ParseClashRules([]byte("rules: ["))
	if err == nil {
		t.Fatal("expected err")
	}
	if !strings.Contains(err.Error(), "解析 YAML") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestParseClashRuleProviders_Normal(t *testing.T) {
	yamlBuf := []byte("rule-providers:\n  alpha:\n    type: http\n    behavior: domain\n    url: https://example.com/alpha.yaml\n    path: ./alpha.yaml\n    interval: 86400\n  beta:\n    type: file\n    behavior: ipcidr\n    path: ./beta.yaml\n")
	got, err := ParseClashRuleProviders(yamlBuf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Name != "alpha" || got[1].Name != "beta" {
		t.Fatalf("names = %v / %v", got[0].Name, got[1].Name)
	}
	if got[0].Raw["type"] != "http" {
		t.Errorf("type = %v", got[0].Raw["type"])
	}
	if got[0].Raw["behavior"] != "domain" {
		t.Errorf("behavior = %v", got[0].Raw["behavior"])
	}
	if got[0].Raw["url"] != "https://example.com/alpha.yaml" {
		t.Errorf("url = %v", got[0].Raw["url"])
	}
	if got[0].Raw["path"] != "./alpha.yaml" {
		t.Errorf("path = %v", got[0].Raw["path"])
	}
	if got[0].Raw["interval"] != 86400 {
		t.Errorf("interval = %v", got[0].Raw["interval"])
	}
}

func TestParseClashRuleProviders_None(t *testing.T) {
	got, err := ParseClashRuleProviders([]byte("proxies:\n  - {name: a, type: ss}\n"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Fatalf("got = %v, want nil", got)
	}
	got2, err := ParseClashRuleProviders([]byte("{}"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got2 != nil {
		t.Fatalf("got2 = %v, want nil", got2)
	}
}

func TestParseClashRuleProviders_NullItem(t *testing.T) {
	got, err := ParseClashRuleProviders([]byte("rule-providers:\n  alpha: null\n"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Raw == nil {
		t.Fatal("Raw should be non-nil empty map")
	}
	if len(got[0].Raw) != 0 {
		t.Errorf("Raw = %v, want empty", got[0].Raw)
	}
}

func TestParseClashRuleProviders_BadYAML(t *testing.T) {
	_, err := ParseClashRuleProviders([]byte("rule-providers: ["))
	if err == nil {
		t.Fatal("expected err")
	}
	if !strings.HasPrefix(err.Error(), "解析 YAML: ") {
		t.Errorf("err = %q", err.Error())
	}
}
