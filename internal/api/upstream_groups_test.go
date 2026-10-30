package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
			"rules:\n" +
			"  - DOMAIN-SUFFIX,x.test,PROXY\n"))
	}))
	t.Cleanup(up.Close)
	id := addImportAirport(t, mgr, "fx-groups", up.URL)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/import-rules", id),
		strings.NewReader(`{"create_groups":true,"enable_rules":true}`))
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("导入状态=%d body=%s", rr.Code, rr.Body.String())
	}
	var imported struct {
		GroupsCreated int `json:"groups_created"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &imported); err != nil {
		t.Fatalf("解析导入响应: %v", err)
	}
	if imported.GroupsCreated != 1 {
		t.Fatalf("groups_created=%d，应为 1；body=%s", imported.GroupsCreated, rr.Body.String())
	}
	groups, err := mgr.ListProxyGroups(t.Context())
	if err != nil || len(groups) != 1 || groups[0].Name != "PROXY" ||
		strings.Join(groups[0].Members, ",") != "HK-1,JP-1" {
		t.Fatalf("上游分组未落库: %+v err=%v", groups, err)
	}

	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	if err := mgr.CreateSubTokenWithProfile(t.Context(), "t-proxy", "", "", "", "", "", "", 0); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}
	st, body := subscribe(t, ts, "t-proxy")
	if st != http.StatusOK {
		t.Fatalf("订阅状态=%d body=%s", st, body)
	}
	for _, want := range []string{"name: PROXY", "HK-1", "JP-1", "DOMAIN-SUFFIX,x.test,PROXY"} {
		if !strings.Contains(body, want) {
			t.Fatalf("订阅缺少 %q；body=%s", want, body)
		}
	}
}
