package api

// 输入校验（D-05 / D-07 / D-08 / D-10 / D-11）与 sort_order 落库（D-11）的 API 层用例。
// 纪律：只发 HTTP，不直调 Manager——校验发生在 HTTP 层，直调测不到。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// probeSS / probeTrojan 是实测能被 parse.ConvertSubscription + parse.Validate 通过的分享链接
// （vmess://x 这类占位串会返回 format invalid，不能当合法链接用）。
const (
	probeSS     = "ss://YWVzLTI1Ni1nY206cGFzcw==@198.51.100.10:8388#probe"
	probeTrojan = "trojan://pass@198.51.100.10:443?sni=example.com#probe"
)

// apiErr 解析统一错误体 {"error":"..."}。
func apiErr(t *testing.T, body string) string {
	t.Helper()
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("错误体不是 JSON: %v; body=%s", err, body)
	}
	return e.Error
}

// ---------- D-05 机场 ----------

func TestAirportPostValidation(t *testing.T) {
	ts, _ := uiSubServer(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空对象", `{}`, "名称不能为空"},
		{"只有名字", `{"name":"A1"}`, "订阅地址不能为空"},
		{"空白订阅地址", `{"name":"A1","subscription_url":"   "}`, "订阅地址不能为空"},
		{"ftp 协议", `{"name":"A1","subscription_url":"ftp://example.com/sub"}`, "订阅地址必须以 http:// 或 https:// 开头"},
		{"javascript 协议", `{"name":"A1","subscription_url":"javascript:alert(1)"}`, "订阅地址必须以 http:// 或 https:// 开头"},
		{"无 scheme", `{"name":"A1","subscription_url":"example.com/sub"}`, "订阅地址必须以 http:// 或 https:// 开头"},
		{"scheme 无 host", `{"name":"A1","subscription_url":"http://"}`, "订阅地址必须以 http:// 或 https:// 开头"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiPostJSON(t, ts, "/api/airports", c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			if got := apiErr(t, body); got != c.want {
				t.Fatalf("error = %q，应为 %q", got, c.want)
			}
		})
	}

	// 合法 https 应 201
	status, _, body := uiPostJSON(t, ts, "/api/airports",
		`{"name":"A1","subscription_url":"https://example.com/sub?token=1","remarks":"r","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("合法 POST 状态码 = %d，应为 201；body=%s", status, body)
	}
}

func TestAirportPutValidation(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/airports",
		`{"name":"A1","subscription_url":"https://example.com/sub"}`)
	if status != http.StatusCreated {
		t.Fatalf("前置 POST 失败: %d; %s", status, body)
	}
	var created struct {
		Airport struct {
			ID int64 `json:"id"`
		} `json:"airport"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	id := fmt.Sprintf("%d", created.Airport.ID)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空名字", `{"name":"","subscription_url":"https://example.com/sub"}`, "名称不能为空"},
		{"空订阅地址", `{"name":"A1","subscription_url":""}`, "订阅地址不能为空"},
		{"非法协议", `{"name":"A1","subscription_url":"ftp://example.com/sub"}`, "订阅地址必须以 http:// 或 https:// 开头"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiDoJSON(t, ts, "PUT", "/api/airports/"+id, c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			if got := apiErr(t, body); got != c.want {
				t.Fatalf("error = %q，应为 %q", got, c.want)
			}
		})
	}
}

// ---------- D-08 规则 ----------

func TestRulePostValidation(t *testing.T) {
	ts, _ := uiSubServer(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空类型", `{"rule_type":"","rule_param":"DIRECT"}`, "类型不能为空"},
		{"空白类型", `{"rule_type":"  ","rule_param":"DIRECT"}`, "类型不能为空"},
		{"空参数", `{"rule_type":"DOMAIN","rule_param":""}`, "参数不能为空"},
		{"空白参数", `{"rule_type":"DOMAIN","rule_param":"   "}`, "参数不能为空"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiPostJSON(t, ts, "/api/rules", c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			if got := apiErr(t, body); got != c.want {
				t.Fatalf("error = %q，应为 %q", got, c.want)
			}
		})
	}

	status, _, body := uiPostJSON(t, ts, "/api/rules",
		`{"rule_type":"DOMAIN","rule_param":"example.com","rule_config":0,"resolve_dns":false,"enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("合法 POST 状态码 = %d，应为 201；body=%s", status, body)
	}
}

func TestRulePutValidation(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/rules",
		`{"rule_type":"DOMAIN","rule_param":"example.com","rule_config":0}`)
	if status != http.StatusCreated {
		t.Fatalf("前置 POST 失败: %d; %s", status, body)
	}
	var created struct {
		Rule struct {
			ID int64 `json:"id"`
		} `json:"rule"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	id := fmt.Sprintf("%d", created.Rule.ID)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空类型", `{"rule_type":"","rule_param":"DIRECT"}`, "类型不能为空"},
		{"空参数", `{"rule_type":"DOMAIN","rule_param":""}`, "参数不能为空"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiDoJSON(t, ts, "PUT", "/api/rules/"+id, c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			if got := apiErr(t, body); got != c.want {
				t.Fatalf("error = %q，应为 %q", got, c.want)
			}
		})
	}
}

// ---------- D-10 分组 ----------

func TestGroupPostValidation(t *testing.T) {
	ts, _ := uiSubServer(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空名字", `{"name":"","type":"select"}`, "名称不能为空"},
		{"空白名字", `{"name":"   ","type":"select"}`, "名称不能为空"},
		{"非法类型", `{"name":"g","type":"bogus"}`, "分组类型必须是 select/url-test/fallback/load-balance 之一"},
		{"非法正则", `{"name":"g","type":"select","regex":"[a-"}`, "匹配正则不合法: "},
		{"负间隔", `{"name":"g","type":"url-test","interval":-5}`, "测活间隔不能为负数"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiPostJSON(t, ts, "/api/groups", c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			got := apiErr(t, body)
			if !strings.HasPrefix(got, c.want) {
				t.Fatalf("error = %q，应以 %q 开头", got, c.want)
			}
		})
	}

	// 合法：type 大写应被规范化成小写入库
	status, _, body := uiPostJSON(t, ts, "/api/groups",
		`{"name":"g","type":"URL-TEST","regex":"^HK","url":"http://www.gstatic.com/generate_204","interval":300}`)
	if status != http.StatusCreated {
		t.Fatalf("合法 POST 状态码 = %d，应为 201；body=%s", status, body)
	}
	var created struct {
		Group struct {
			Type string `json:"type"`
		} `json:"group"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	if created.Group.Type != "url-test" {
		t.Fatalf("type = %q，应为小写 url-test", created.Group.Type)
	}
}

func TestGroupPutValidation(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/groups", `{"name":"g","type":"select"}`)
	if status != http.StatusCreated {
		t.Fatalf("前置 POST 失败: %d; %s", status, body)
	}
	var created struct {
		Group struct {
			ID int64 `json:"id"`
		} `json:"group"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	id := fmt.Sprintf("%d", created.Group.ID)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空名字", `{"name":"","type":"select"}`, "名称不能为空"},
		{"非法类型", `{"name":"g","type":"bogus"}`, "分组类型必须是 select/url-test/fallback/load-balance 之一"},
		{"非法正则", `{"name":"g","type":"select","regex":"[a-"}`, "匹配正则不合法: "},
		{"负间隔", `{"name":"g","type":"url-test","interval":-1}`, "测活间隔不能为负数"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiDoJSON(t, ts, "PUT", "/api/groups/"+id, c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			got := apiErr(t, body)
			if !strings.HasPrefix(got, c.want) {
				t.Fatalf("error = %q，应以 %q 开头", got, c.want)
			}
		})
	}
}

// ---------- D-07 自建节点 ----------

func TestSelfNodePostValidation(t *testing.T) {
	ts, _ := uiSubServer(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空名字", `{"name":"","link":"` + probeSS + `"}`, "名称不能为空"},
		{"空白名字", `{"name":"  ","link":"` + probeSS + `"}`, "名称不能为空"},
		{"空链接", `{"name":"n","link":""}`, "分享链接不能为空"},
		{"空白链接", `{"name":"n","link":"   "}`, "分享链接不能为空"},
		{"不是链接", `{"name":"n","link":"not-a-valid-link"}`, "分享链接格式不正确："},
		{"残缺 vmess", `{"name":"n","link":"vmess://x"}`, "分享链接格式不正确："},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiPostJSON(t, ts, "/api/self-nodes", c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			got := apiErr(t, body)
			if !strings.HasPrefix(got, c.want) {
				t.Fatalf("error = %q，应以 %q 开头", got, c.want)
			}
		})
	}

	status, _, body := uiPostJSON(t, ts, "/api/self-nodes",
		`{"name":"n1","link":"`+probeSS+`","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("合法 POST 状态码 = %d，应为 201；body=%s", status, body)
	}
}

func TestSelfNodePutValidation(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/self-nodes",
		`{"name":"n1","link":"`+probeSS+`","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("前置 POST 失败: %d; %s", status, body)
	}
	var created struct {
		SelfNode struct {
			ID int64 `json:"id"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	id := fmt.Sprintf("%d", created.SelfNode.ID)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"空名字", `{"name":"","link":"` + probeSS + `"}`, "名称不能为空"},
		{"空链接", `{"name":"n","link":""}`, "分享链接不能为空"},
		{"不是链接", `{"name":"n","link":"not-a-valid-link"}`, "分享链接格式不正确："},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, _, body := uiDoJSON(t, ts, "PUT", "/api/self-nodes/"+id, c.body)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d，应为 400；body=%s", status, body)
			}
			got := apiErr(t, body)
			if !strings.HasPrefix(got, c.want) {
				t.Fatalf("error = %q，应以 %q 开头", got, c.want)
			}
		})
	}
}

// ---------- D-11 sort_order ----------

func TestGroupSortOrderPersisted(t *testing.T) {
	ts, _ := uiSubServer(t)

	// 先建 sort_order=5 的，再建 sort_order=1 的：GET 应按 sort_order 升序，g2 在前。
	status, _, body := uiPostJSON(t, ts, "/api/groups",
		`{"name":"probe-g1","type":"select","sort_order":5}`)
	if status != http.StatusCreated {
		t.Fatalf("POST g1 状态码 = %d，应为 201；body=%s", status, body)
	}
	var c1 struct {
		Group struct {
			ID        int64 `json:"id"`
			SortOrder int   `json:"sort_order"`
		} `json:"group"`
	}
	if err := json.Unmarshal([]byte(body), &c1); err != nil {
		t.Fatalf("解析 g1 响应失败: %v; body=%s", err, body)
	}
	if c1.Group.SortOrder != 5 {
		t.Fatalf("g1.sort_order = %d，应为 5", c1.Group.SortOrder)
	}

	status, _, body = uiPostJSON(t, ts, "/api/groups",
		`{"name":"probe-g2","type":"select","sort_order":1}`)
	if status != http.StatusCreated {
		t.Fatalf("POST g2 状态码 = %d，应为 201；body=%s", status, body)
	}

	status, _, body = uiGet(t, ts, "/api/groups")
	if status != http.StatusOK {
		t.Fatalf("GET 状态码 = %d，应为 200；body=%s", status, body)
	}
	var resp struct {
		Groups []struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析列表失败: %v; body=%s", err, body)
	}
	if len(resp.Groups) != 2 {
		t.Fatalf("应有 2 个分组，got %d", len(resp.Groups))
	}
	if resp.Groups[0].Name != "probe-g2" || resp.Groups[0].SortOrder != 1 {
		t.Fatalf("第一项 = %+v，应为 probe-g2/1", resp.Groups[0])
	}
	if resp.Groups[1].Name != "probe-g1" || resp.Groups[1].SortOrder != 5 {
		t.Fatalf("第二项 = %+v，应为 probe-g1/5", resp.Groups[1])
	}

	// PUT 传 0 应保持原值
	status, _, body = uiDoJSON(t, ts, "PUT", "/api/groups/"+fmt.Sprint(c1.Group.ID),
		`{"name":"probe-g1","type":"select","sort_order":0}`)
	if status != http.StatusOK {
		t.Fatalf("PUT 状态码 = %d，应为 200；body=%s", status, body)
	}
	status, _, body = uiGet(t, ts, "/api/groups")
	if status != http.StatusOK {
		t.Fatalf("GET 状态码 = %d，应为 200；body=%s", status, body)
	}
	var after struct {
		Groups []struct {
			Name      string `json:"name"`
			SortOrder int    `json:"sort_order"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(body), &after); err != nil {
		t.Fatalf("解析列表失败: %v; body=%s", err, body)
	}
	for _, g := range after.Groups {
		if g.Name == "probe-g1" && g.SortOrder != 5 {
			t.Fatalf("PUT sort_order=0 后 probe-g1.sort_order = %d，应保持 5", g.SortOrder)
		}
	}
}

func TestSelfNodeSortOrderPersisted(t *testing.T) {
	ts, _ := uiSubServer(t)

	status, _, body := uiPostJSON(t, ts, "/api/self-nodes",
		`{"name":"probe-s1","link":"`+probeSS+`","enabled":true,"sort_order":9}`)
	if status != http.StatusCreated {
		t.Fatalf("POST 状态码 = %d，应为 201；body=%s", status, body)
	}
	var created struct {
		SelfNode struct {
			ID        int64 `json:"id"`
			SortOrder int   `json:"sort_order"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	if created.SelfNode.SortOrder != 9 {
		t.Fatalf("POST 回读 sort_order = %d，应为 9", created.SelfNode.SortOrder)
	}

	// 不传 sort_order（0）时应回退成时间戳，且 > 0
	status, _, body = uiPostJSON(t, ts, "/api/self-nodes",
		`{"name":"probe-s2","link":"`+probeTrojan+`","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("POST 状态码 = %d，应为 201；body=%s", status, body)
	}
	var created2 struct {
		SelfNode struct {
			SortOrder int `json:"sort_order"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created2); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	if created2.SelfNode.SortOrder <= 0 {
		t.Fatalf("未传 sort_order 时应回退为正时间戳，got %d", created2.SelfNode.SortOrder)
	}

	// PUT 显式覆盖
	status, _, body = uiDoJSON(t, ts, "PUT", "/api/self-nodes/"+fmt.Sprint(created.SelfNode.ID),
		`{"name":"probe-s1","link":"`+probeSS+`","enabled":true,"sort_order":3}`)
	if status != http.StatusOK {
		t.Fatalf("PUT 状态码 = %d，应为 200；body=%s", status, body)
	}
	status, _, body = uiGet(t, ts, "/api/self-nodes/"+fmt.Sprint(created.SelfNode.ID))
	if status != http.StatusOK {
		t.Fatalf("GET 状态码 = %d，应为 200；body=%s", status, body)
	}
	var detail struct {
		SelfNode struct {
			SortOrder int `json:"sort_order"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &detail); err != nil {
		t.Fatalf("解析详情失败: %v; body=%s", err, body)
	}
	if detail.SelfNode.SortOrder != 3 {
		t.Fatalf("PUT 后 sort_order = %d，应为 3", detail.SelfNode.SortOrder)
	}
}
