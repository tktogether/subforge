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
