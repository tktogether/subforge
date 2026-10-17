package api

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/convert"
)

// subWebClientTypes 是 CareyWang/sub-web 的 src/config/client-types.js 全文，逐项抄录。
// 它是"sub-web 可直连"这条 M2 验收标准的输入清单：前端下拉框里的每一项，
// 用前端真实的参数拼法打过来，都必须拿到可用的产物。
//
// 注意 Surge 的两项在 sub-web 里是 "surge&ver=4" / "surge&ver=3"——
// useSubscription.js 会把它拆成 target=surge 与 ver=4 两个独立参数。
// subWebClientType 是 sub-web 下拉框里的一项。
// Label 是前端原值（含 Surge 的 "&ver=" 后缀写法），Target/Ver 是拆开后的两个参数。
type subWebClientType struct {
	Label  string
	Target string
	Ver    string
}

var subWebClientTypes = []subWebClientType{
	{"Clash", "clash", ""},
	{"Surge", "surge", "4"},
	{"Quantumult", "quan", ""},
	{"QuantumultX", "quanx", ""},
	{"Mellow", "mellow", ""},
	{"Surfboard", "surfboard", ""},
	{"Loon", "loon", ""},
	{"singbox", "singbox", ""},
	{"ss", "ss", ""},
	{"ssd", "ssd", ""},
	{"sssub", "sssub", ""},
	{"ssr", "ssr", ""},
	{"ClashR", "clashr", ""},
	{"V2Ray", "v2ray", ""},
	{"Trojan", "trojan", ""},
	{"Surge3", "surge", "3"},
}

// subWebQuery 复刻 sub-web src/composables/useSubscription.js 的 makeUrl。
//
// advanced=true 对应前端 advanced === "2"（打开"高级"开关）：它会把下面这一整
// 组参数无条件塞进 query，包括 sort=false / emoji=true / list=false 这些布尔。
// 这里逐字复刻，是为了让测试跟着前端真实请求走，而不是跟着我们以为的请求走。
func subWebQuery(clientType subWebClientType, subURL string, advanced bool) string {
	q := url.Values{}
	q.Set("target", clientType.Target)
	if clientType.Target == "surge" && clientType.Ver != "" {
		q.Set("ver", clientType.Ver)
	}
	q.Set("url", subURL)
	q.Set("insert", "false")
	if !advanced {
		return q.Encode()
	}
	// stringParams：前端只在非空时才 set，所以 config/exclude/include/filename 留空。
	// booleanParams：无条件 set。
	for _, k := range []string{"emoji", "list", "tfo", "scv", "fdn", "expand", "sort"} {
		switch k {
		case "emoji", "scv", "expand":
			q.Set(k, "true")
		default:
			q.Set(k, "false")
		}
	}
	q.Set("append_type", "false")
	q.Set("udp", "false")
	q.Set("surge.doh", "false")
	q.Set("clash.doh", "false")
	q.Set("new_name", "true")
	return q.Encode()
}

// TestSubWeb_AllClientTypesConvert sub-web 的 16 个客户端类型都能出产物。
//
// 两种模式都跑：普通模式（只有 target/url/insert）与高级模式（全参数）。
// 高级模式是重点——前端一旦打开"高级"开关，sort=false 这类布尔就会混进来，
// 而 sort 在后端是排序键枚举，只认排序键的实现会整条请求 502。
func TestSubWeb_AllClientTypesConvert(t *testing.T) {
	ts := newTestServer(t, nil)
	sub := fixtureURL(t)

	for _, mode := range []struct {
		name     string
		advanced bool
	}{
		{"simple", false},
		{"advanced", true},
	} {
		for _, ct := range subWebClientTypes {
			q := subWebQuery(ct, sub, mode.advanced)
			status, hdr, body := doGet(t, ts.URL+"/sub?"+q, "", "")
			if status != http.StatusOK {
				t.Errorf("%s/%s: 状态 = %d，body = %s", mode.name, ct.Label, status, truncate(body, 200))
				continue
			}
			if strings.TrimSpace(body) == "" {
				t.Errorf("%s/%s: 产物为空", mode.name, ct.Label)
			}
			if hdr.Get("X-Convert-Format") == "" {
				t.Errorf("%s/%s: 缺少 X-Convert-Format", mode.name, ct.Label)
			}
			if hdr.Get("Content-Disposition") == "" {
				t.Errorf("%s/%s: 缺少 Content-Disposition", mode.name, ct.Label)
			}
		}
	}
}

// TestSubWeb_VersionEndpointIsPlainText 是 sub-web 版本显示的硬契约。
//
// sub-web 的 BackendService 直接 GET <backend>/version，拿到 response.data 后
// 交给 formatVersion 做字符串处理：先剥结尾的 "backend\n"，再剥 "subconverter"。
// 这里把 formatVersion 的算法原样搬过来跑一遍，证明我们的输出能过。
func TestSubWeb_VersionEndpointIsPlainText(t *testing.T) {
	ts := newTestServer(t, nil)

	status, hdr, body := doGet(t, ts.URL+"/version", "", "")
	if status != http.StatusOK {
		t.Fatalf("/version 状态 = %d", status)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("/version Content-Type = %q，want text/plain", ct)
	}

	// formatVersion 的等价实现：replace(/backend\n$/gm, "") 后 replace("subconverter", "")。
	cleaned := subWebFormatVersion(body)
	if strings.TrimSpace(cleaned) == "" {
		t.Errorf("formatVersion 后为空——sub-web 页面上版本号会显示不出来，原始响应 = %q", body)
	}
	if !strings.HasSuffix(body, "backend\n") {
		t.Errorf("/version 必须以 backend 加换行结尾，实际 = %q", body)
	}
	// 名字里出现 "subconverter" 会被前端误剥，得确认我们没冒充上游。
	if strings.Contains(body, "subconverter") {
		t.Errorf("/version 不该自称 subconverter: %q", body)
	}
}

// TestSubWeb_SortAcceptsSubconverterBoolean sort 的两种写法都要吃。
//
// subconverter / sub-web 用布尔（true=按备注排序，false=保持原序）；
// Sub-Store 语义用排序键。只认后者会让高级模式整条请求 502。
func TestSubWeb_SortAcceptsSubconverterBoolean(t *testing.T) {
	ts := newTestServer(t, nil)
	sub := fixtureURL(t)
	base := "target=clash&url=" + url.QueryEscape(sub)

	for _, tc := range []struct {
		val  string
		want int
	}{
		{"false", http.StatusOK},
		{"true", http.StatusOK},
		{"0", http.StatusOK},
		{"1", http.StatusOK},
		{"name", http.StatusOK},
		{"original", http.StatusOK},
		{"port", http.StatusOK},
		{"bogus", http.StatusBadGateway}, // 非法排序键仍然要报错，别把校验放宽没了
	} {
		status, _, body := doGet(t, ts.URL+"/sub?"+base+"&sort="+tc.val, "", "")
		if status != tc.want {
			t.Errorf("sort=%s 状态 = %d，want %d（body=%s）", tc.val, status, tc.want, truncate(body, 160))
		}
	}
}

// TestSubWeb_SortBooleanSemantics 布尔映射到正确的排序键，不只是"不报错"。
func TestSubWeb_SortBooleanSemantics(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want string
	}{
		{"true", "name"},
		{"1", "name"},
		{"yes", "name"},
		{"false", "original"},
		{"0", "original"},
		{"no", "original"},
		{"port", "port"},
		{"", ""},
	} {
		p, err := convert.ParseMulti(map[string][]string{"sort": {tc.val}})
		if err != nil {
			t.Errorf("sort=%q 解析失败: %v", tc.val, err)
			continue
		}
		if p.Sort != tc.want {
			t.Errorf("sort=%q -> %q，want %q", tc.val, p.Sort, tc.want)
		}
	}
}

// TestSubWeb_FilenameParam filename 参数覆盖下载文件名。
func TestSubWeb_FilenameParam(t *testing.T) {
	ts := newTestServer(t, nil)
	sub := fixtureURL(t)

	status, hdr, _ := doGet(t, ts.URL+"/sub?target=clash&url="+url.QueryEscape(sub)+"&filename=my-custom.yaml", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d", status)
	}
	if got := hdr.Get("Content-Disposition"); !strings.Contains(got, "my-custom.yaml") {
		t.Errorf("Content-Disposition = %q，应含 my-custom.yaml", got)
	}
}

// subWebFormatVersion 逐行搬运 sub-web src/utils/formatters.js 的 formatVersion：
//
//	let cleaned = version.replace(/backend\n$/gm, "");
//	cleaned = cleaned.replace("subconverter", "");
//	return cleaned;
//
// 搬过来是为了让「我们的输出能不能过前端」这件事在测试里有据可依，
// 而不是靠人去心算 JS 的字符串语义。
func subWebFormatVersion(version string) string {
	cleaned := regexp.MustCompile("backend\n$").ReplaceAllString(version, "")
	return strings.Replace(cleaned, "subconverter", "", 1)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
