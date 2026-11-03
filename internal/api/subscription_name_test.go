package api

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSubscriptionNameHelpers(t *testing.T) {
	cases := []struct {
		token, plan, server, want string
	}{
		{"", "", "", "SubForge"},
		{"令牌名", "方案名", "服务名", "令牌名"},
		{"", "方案名", "服务名", "方案名"},
		{"", "", "服务名", "服务名"},
		{" \r\n ", "方案名", "", "方案名"},
	}
	for _, tc := range cases {
		if got := subscriptionDisplayName(tc.token, tc.plan, tc.server); got != tc.want {
			t.Errorf("subscriptionDisplayName(%q,%q,%q) = %q，want %q",
				tc.token, tc.plan, tc.server, got, tc.want)
		}
	}

	if got := profileTitleHeader("My Sub"); got != "My Sub" {
		t.Errorf("ASCII profile-title = %q", got)
	}
	title := profileTitleHeader("我的订阅")
	if !strings.HasPrefix(title, "base64:") {
		t.Fatalf("中文 profile-title 应使用 base64: %q", title)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(title, "base64:"))
	if err != nil || string(decoded) != "我的订阅" {
		t.Errorf("profile-title 解码 = %q/%v", decoded, err)
	}

	if got := subscriptionContentDisposition("My Sub"); !strings.Contains(got, `filename="My Sub"`) || strings.Contains(got, ".yaml") {
		t.Errorf("ASCII Content-Disposition = %q", got)
	}
	cd := subscriptionContentDisposition("我的/订阅")
	if !strings.Contains(cd, `filename="subforge"`) || strings.Contains(cd, ".yaml") {
		t.Errorf("中文 Content-Disposition 缺少 ASCII 回退或带了扩展名: %q", cd)
	}
	if !strings.Contains(cd, "filename*=UTF-8''"+url.PathEscape("我的订阅")) {
		t.Errorf("中文 Content-Disposition 缺少 filename*: %q", cd)
	}
	if got := sanitizeFileName(" a/b:c*?\"<>| "); got != "abc" {
		t.Errorf("sanitizeFileName = %q", got)
	}
	if got := sanitizeFileName("///"); got != "subforge" {
		t.Errorf("空文件名回退 = %q", got)
	}
	if !isPrintableASCII("abc") || isPrintableASCII("中文") || isPrintableASCII("a\nb") {
		t.Errorf("isPrintableASCII 判定错误")
	}
}

func TestSubscribeNameHeaders(t *testing.T) {
	ts, _, mgr := profileServer(t)
	if _, err := mgr.AddAirport(t.Context(), "fx", fixtureURL(t), "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	p := mustProfile(t, ts, "命名方案")
	cases := []struct {
		token    string
		name     string
		title    string
		filename string
	}{
		{"t-name-zh", "我的订阅", "base64:" + base64.StdEncoding.EncodeToString([]byte("我的订阅")), url.PathEscape("我的订阅")},
		{"t-name-ascii", "My Sub", "My Sub", `filename="My Sub"`},
	}
	for _, tc := range cases {
		st, body := postAPIJSON(t, ts, "/api/tokens", map[string]any{
			"token": tc.token, "name": tc.name, "profile_id": p.ID,
		})
		if st != http.StatusCreated {
			t.Fatalf("创建令牌 %s 状态 = %d；body=%s", tc.token, st, body)
		}
		st, hdr, body := doGet(t, ts.URL+"/subscribe?token="+tc.token, "", "")
		if st != http.StatusOK {
			t.Fatalf("订阅 %s 状态 = %d；body=%s", tc.token, st, body)
		}
		if got := hdr.Get("profile-title"); got != tc.title {
			t.Errorf("profile-title = %q，want %q", got, tc.title)
		}
		cd := hdr.Get("Content-Disposition")
		if !strings.Contains(cd, tc.filename) && !strings.Contains(cd, "filename*=UTF-8''"+tc.filename) {
			t.Errorf("Content-Disposition = %q，应包含 %q", cd, tc.filename)
		}
		if !strings.Contains(body, "配置名: "+tc.name) {
			head := body
			if len(head) > 200 {
				head = head[:200]
			}
			t.Errorf("配置头未写入订阅名 %q：%s", tc.name, head)
		}
	}
}
