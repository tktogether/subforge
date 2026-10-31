package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/subforge"
)

// TestImportSubscribePreservesUpstreamGroups 端到端验证机场自带分组：
// 解析 proxy-groups → 导入落库 → 规则目标指向该组 → 订阅按成员渲染。
func TestImportSubscribePreservesUpstreamGroups(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n" +
			"  - {name: HK-1, type: ss, server: 127.0.0.1, port: 30021, cipher: aes-256-gcm, password: pass}\n" +
			"  - {name: JP-1, type: ss, server: 127.0.0.1, port: 30022, cipher: aes-256-gcm, password: pass}\n" +
			"proxy-groups:\n" +
			"  - name: PROXY\n" +
			"    type: select\n" +
			"    proxies: [HK-1, JP-1]\n" +
			"  - name: HK-ONLY\n" +
			"    type: select\n" +
			"    proxies: [HK-1]\n" +
			"  - name: GROUP-ONLY\n" +
			"    type: select\n" +
			"    proxies: [PROXY, DIRECT]\n" +
			"rules:\n" +
			"  - DOMAIN-SUFFIX,x.test,PROXY\n"))
	}))
	t.Cleanup(up.Close)
	id := addImportAirport(t, mgr, "fx-groups", up.URL)
	if _, err := mgr.AddSelfNode(t.Context(), "trojan://pass@127.0.0.1:30001?allowInsecure=1#orig", "SELF-X", true, 1); err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/import-rules", id),
		strings.NewReader(`{"create_groups":true,"enable_rules":true}`))
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("导入状态=%d body=%s", rr.Code, rr.Body.String())
	}
	var imported struct {
		GroupsCreated  int    `json:"groups_created"`
		RuleSourceName string `json:"rule_source_name"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &imported); err != nil {
		t.Fatalf("解析导入响应: %v", err)
	}
	if imported.GroupsCreated != 3 {
		t.Fatalf("groups_created=%d，应为 3；body=%s", imported.GroupsCreated, rr.Body.String())
	}
	if imported.RuleSourceName == "" {
		t.Fatalf("机场规则未生成规则源：%s", rr.Body.String())
	}
	groups, err := mgr.ListProxyGroups(t.Context())
	if err != nil || len(groups) != 3 {
		t.Fatalf("上游分组未落库: %+v err=%v", groups, err)
	}
	byName := make(map[string]subforge.ProxyGroup, len(groups))
	for _, g := range groups {
		byName[g.Name] = g
	}
	if g := byName["PROXY"]; strings.Join(g.Members, ",") != "HK-1,JP-1" || !g.IncludeSelf || !g.IncludeAllSources {
		t.Fatalf("覆盖全部机场节点的 PROXY 应开启自建/全部来源扩展: %+v", g)
	}
	if byName["HK-ONLY"].IncludeSelf || byName["HK-ONLY"].IncludeAllSources {
		t.Fatalf("子集组 HK-ONLY 不应开启扩展: %+v", byName["HK-ONLY"])
	}
	if byName["GROUP-ONLY"].IncludeSelf || byName["GROUP-ONLY"].IncludeAllSources {
		t.Fatalf("只引用子组的 GROUP-ONLY 不应开启扩展: %+v", byName["GROUP-ONLY"])
	}

	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	p := mustProfilePayload(t, ts, map[string]any{
		"name": "proxy-src", "rule_sources": []string{imported.RuleSourceName},
	})
	mustToken(t, ts, "t-proxy", p.ID)
	st, body := subscribe(t, ts, "t-proxy")
	if st != http.StatusOK {
		t.Fatalf("订阅状态=%d body=%s", st, body)
	}
	for _, want := range []string{"name: PROXY", "HK-1", "JP-1", "DOMAIN-SUFFIX,x.test,PROXY"} {
		if !strings.Contains(body, want) {
			t.Fatalf("订阅缺少 %q；body=%s", want, body)
		}
	}
	rendered := parseRenderedGroups(t, body)
	if !containsName(rendered["PROXY"].Proxies, "SELF-X") {
		t.Fatalf("PROXY 分组应追加自建节点 SELF-X: %v", rendered["PROXY"].Proxies)
	}
	if containsName(rendered["HK-ONLY"].Proxies, "SELF-X") {
		t.Fatalf("HK-ONLY 分组不应追加自建节点: %v", rendered["HK-ONLY"].Proxies)
	}
}
