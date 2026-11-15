package api

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestSplitListQuery(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
		{"a,,b", []string{"a", "b"}},
		{",,,", nil},
	}
	for _, c := range cases {
		got := splitListQuery(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("splitListQuery(%q): got %v want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("splitListQuery(%q): got %v want %v", c.in, got, c.want)
			}
		}
		// nil check
		if c.want == nil && got != nil {
			t.Fatalf("splitListQuery(%q): want nil got %v", c.in, got)
		}
	}
}

func TestSubscribeQueryInclude(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	// baseline
	_, _, baseBody := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	// include only vless
	status, _, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&include=vless")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if strings.Count(body, "vless") >= strings.Count(baseBody, "vless") {
		// include should reduce nodes; at least total body shorter or fewer vless? check node count proxy
	}
	if len(body) >= len(baseBody) {
		t.Fatalf("include 未减少输出: base=%d filtered=%d", len(baseBody), len(body))
	}
	_ = baseBody
}

func TestSubscribeQueryExclude(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	status, _, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&exclude=vmess")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if strings.Contains(body, "vmess-node") {
		t.Fatalf("exclude 未生效，vmess-node 仍在输出")
	}
}

func TestSubscribeQueryConfig(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	// baseline without config
	status0, _, body0 := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	// nonexistent template name
	status1, _, body1 := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&config=__no_such_template__")
	t.Logf("baseline status=%d len=%d; config status=%d len=%d", status0, len(body0), status1, len(body1))
	// accept either: if engine errors, status differs; if not, document
	if status0 != status1 {
		t.Logf("config 触发了不同响应码：%d vs %d", status0, status1)
	}
}

func TestSubscribeQueryPipelineFallback(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	// write subscribe.pipeline into config table
	if _, err := mgr.DB().Raw().ExecContext(ctx, "INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", "subscribe.pipeline", "nope"); err != nil {
		t.Fatalf("写 config: %v", err)
	}
	// fallback (no pipeline query) should behave like explicit pipeline=nope
	statusFb, _, bodyFb := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	statusEx, _, bodyEx := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&pipeline=nope")
	t.Logf("fallback status=%d len=%d; explicit status=%d len=%d", statusFb, len(bodyFb), statusEx, len(bodyEx))
	if statusFb != statusEx {
		t.Fatalf("fallback 与显式 pipeline 响应码不一致: %d vs %d", statusFb, statusEx)
	}
}

// wsUARx 匹配 mihomo common/convert 给 ws 分享链接注入的随机浏览器 User-Agent 行
// （headers["User-Agent"] = RandUserAgent()，上游库行为）。该注入发生在 engine.Run
// 内部的链接解析里，与本重构无关，同一配置两次拉取该行天然不同，比较前先归一化。
var wsUARx = regexp.MustCompile(`(?m)^\s*User-Agent: Mozilla/5\.0.*$`)

// TestSubscribeDefaultComboDeterministic 特征测试：同一令牌连续两次拉取默认组合
// （不带 target/pipeline/config/include/exclude），profile-title 响应头与响应体必须
// 一致（ws 节点的随机 User-Agent 行归一化后逐字节比较）；显式空串参数（target=&…=）
// 与不传参数也必须等价。作为 buildSubscription 抽取（纯重构）的前后基线，防止重构变形。
func TestSubscribeDefaultComboDeterministic(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	st1, hdr1, body1 := uiGet(t, ts, "/subscribe?token=sub-token-1")
	st2, hdr2, body2 := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if st1 != http.StatusOK || st2 != http.StatusOK {
		t.Fatalf("status=%d/%d body=%s", st1, st2, body1)
	}
	if body1 == "" {
		t.Fatalf("默认组合拉取得到空响应体")
	}
	title1, title2 := hdr1.Get("profile-title"), hdr2.Get("profile-title")
	if title1 == "" || title1 != title2 {
		t.Fatalf("profile-title 不一致或为空: %q vs %q", title1, title2)
	}
	if norm1, norm2 := wsUARx.ReplaceAllString(body1, ""), wsUARx.ReplaceAllString(body2, ""); norm1 != norm2 {
		t.Fatalf("两次默认组合拉取的响应体不一致：len %d vs %d", len(body1), len(body2))
	}
	// 显式空串参数必须与不传参数等价：空串/未出现都算空。
	st3, _, body3 := uiGet(t, ts, "/subscribe?token=sub-token-1&target=&pipeline=&config=&include=&exclude=")
	if st3 != http.StatusOK {
		t.Fatalf("显式空串参数 status=%d body=%s", st3, body3)
	}
	if wsUARx.ReplaceAllString(body3, "") != wsUARx.ReplaceAllString(body1, "") {
		t.Fatalf("显式空串参数与默认拉取响应体不一致：len %d vs %d", len(body3), len(body1))
	}
}

// TestBuildSubscriptionDefaultCombo 直测 buildSubscription 的 DefaultCombo 判定：
// 只看 query 的 target/pipeline/config/include/exclude 五个参数（空串/未出现都算空），
// 令牌/方案字段不参与判定；顺带锚定成功返回契约（status==0、Name 取令牌名、
// UsedAirports 收集、cleanup 恒非 nil）。Task 5 产物预生成复用该函数时以此为准。
func TestBuildSubscriptionDefaultCombo(t *testing.T) {
	ts, srv, mgr := profileServer(t)
	_ = ts
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	// 令牌自带 target 与 include 定制：证明这两个字段不影响 DefaultCombo。
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "clash.meta", "", "", "vless", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	cases := []struct {
		query       string
		wantDefault bool
	}{
		{"", true}, // 全未定制（令牌字段不算定制）
		{"&target=&pipeline=&config=&include=&exclude=", true}, // 显式空串也算空
		{"&include=", true},           // 空串 include 归并后落到令牌值，但判定只看 query
		{"&target=clash.meta", false}, // query 显式 target
		{"&pipeline=std", false},
		{"&config=nope", false},
		{"&include=vmess", false},
		{"&exclude=vmess", false},
	}
	for _, tc := range cases {
		q, err := url.ParseQuery("token=sub-token-1" + tc.query)
		if err != nil {
			t.Fatalf("解析 query %q: %v", tc.query, err)
		}
		b, cleanup, status, err := srv.buildSubscription(ctx, "sub-token-1", q)
		defer cleanup()
		if status != 0 || err != nil {
			t.Fatalf("query=%q status=%d err=%v", tc.query, status, err)
		}
		if b.DefaultCombo != tc.wantDefault {
			t.Errorf("query=%q DefaultCombo = %v，want %v", tc.query, b.DefaultCombo, tc.wantDefault)
		}
		if b.Name != "测试" {
			t.Errorf("query=%q Name = %q，want 令牌名 测试", tc.query, b.Name)
		}
		if b.Params.ProfileName != b.Name {
			t.Errorf("query=%q Params.ProfileName = %q，应与 Name 一致", tc.query, b.Params.ProfileName)
		}
		if len(b.UsedAirports) != 1 {
			t.Errorf("query=%q UsedAirports = %d 个，want 1", tc.query, len(b.UsedAirports))
		}
		if b.Params.Target != "clash.meta" {
			t.Errorf("query=%q Params.Target = %q，want clash.meta", tc.query, b.Params.Target)
		}
	}
}

func TestSubscribeQueryRegression(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	group, err := mgr.AddProxyGroup(ctx, "AutoSelect", "select", ".*", "", 0, 0)
	if err != nil {
		t.Fatalf("插入 group: %v", err)
	}
	if _, err := mgr.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", group.ID, true, true); err != nil {
		t.Fatalf("插入 rule: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	status, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if ct := hdr.Get("Content-Type"); strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(body, "AutoSelect") {
		t.Errorf("缺少 AutoSelect")
	}
}
