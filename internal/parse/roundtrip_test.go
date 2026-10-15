// 往返测试：配置 map -> 分享链接 -> 配置 map。
//
// 内核 outbound 适配器没有 Marshal 方法，序列化是自研的，所以用一个硬 oracle 兜住正确性：
// 把产物再喂回内核解析器，比对的不是链接文本本身（那是格式细节），而是解析出来的配置。
// 这个测试放在 parse 包里，因为它是全项目唯一被允许 import 内核的包。
package parse_test

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/sharelink"
)

// derivedKeys 是内核解析时补的默认值，与链接格式无关，比对时剔除。
var derivedKeys = map[string]bool{
	"udp":  true,
	"xudp": true,
}

// normalize 去掉内核派生字段，并把随机注入的 User-Agent 抹平。
func normalize(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if derivedKeys[k] {
			continue
		}
		out[k] = v
	}
	if ws, ok := out["ws-opts"].(map[string]any); ok {
		if h, ok := ws["headers"].(map[string]any); ok {
			cp := map[string]any(h)
			delete(cp, "User-Agent")
			ws = map[string]any(ws)
			ws["headers"] = cp
			out["ws-opts"] = ws
		}
	}
	return out
}

func TestSharelinkRoundTrip_Fixture(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/fixtures/mixed-subscription.txt")
	if err != nil {
		t.Fatal(err)
	}
	nodes := model.NewNodeList(mustParse(t, raw), "fixture")
	out, err := sharelink.RenderAll(nodes)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if len(out.Skipped) > 0 {
		t.Fatalf("有节点未能序列化: %v", out.Skipped)
	}
	if out.Lines != nodes.Len() {
		t.Fatalf("行数 %d，期望 %d", out.Lines, nodes.Len())
	}

	// 产物再喂回内核，验证语义等价。
	reread := mustParse(t, out.Body)
	if len(reread) != len(nodes) {
		t.Fatalf("往返后节点数 %d，期望 %d", len(reread), len(nodes))
	}
	for i := range nodes {
		got := normalize(reread[i])
		want := normalize(nodes[i].Raw)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%d] %s 往返不等价\n--- 原始 ---\n%s\n--- 往返 ---\n%s",
				i, nodes[i].Name, dump(want), dump(got))
		}
	}
}

func TestSharelinkRoundTrip_EmptyFragment(t *testing.T) {
	// 内核在 fragment 为空时会用 server:port 兜底生成名字；序列化必须显式带上名字，
	// 否则往返后节点名会漂移。
	nodes := model.NewNodeList(mustParse(t, []byte("http://u:p@example.com:8080")), "t")
	out, err := sharelink.RenderAll(nodes)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(out.Body))
	if !strings.Contains(line, "#") {
		t.Fatalf("缺 fragment，内核会给节点起别的名字: %s", line)
	}
}

func TestSharelinkRoundTrip_FragmentEscaping(t *testing.T) {
	// 节点名带 emoji、空格与 URL 结构字符——fragment 必须正确转义才能原样回来。
	src := "http://u:p@example.com:8080#🚀 节点 A/B&C?D=E+F"
	a := mustParse(t, []byte(src))
	b := mustParse(t, mustSerialize(t, a))
	if normalize(a[0])["name"] != normalize(b[0])["name"] {
		t.Errorf("名字未往返: %q -> %q",
			normalize(a[0])["name"], normalize(b[0])["name"])
	}
}

// TestSharelink_MixedRoundTrip base64 混合形态（subconverter 的 mixed / ss / ssr / trojan /
// v2ray）也要能被我方解析器读回来——产物是订阅地址，读不回来等于给客户端一份死链。
func TestSharelink_MixedRoundTrip(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/fixtures/mixed-subscription.txt")
	if err != nil {
		t.Fatal(err)
	}
	nodes := model.NewNodeList(mustParse(t, raw), "fixture")

	r := sharelink.New("mixed", sharelink.Options{Base64: true})
	body, err := r.Render(render.Request{Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Warnings(); len(got) != 0 {
		t.Fatalf("有节点未能序列化: %v", got)
	}
	reread := mustParse(t, body)
	if len(reread) != len(nodes) {
		t.Errorf("往返后节点数 %d，期望 %d", len(reread), len(nodes))
	}
}

func TestSharelink_SkipsUnsupported(t *testing.T) {
	weird := model.Node{
		Name: "w",
		Raw:  map[string]any{"name": "w", "type": "wireguard", "server": "s", "port": "51820"},
		Type: "wireguard",
	}
	if _, err := sharelink.Render(weird); !strings.Contains(err.Error(), "不支持") {
		t.Errorf("应当报不支持，实际: %v", err)
	}
	ok := model.Node{
		Name: "ok",
		Raw:  map[string]any{"name": "ok", "type": "socks5", "server": "s", "port": "1080"},
		Type: "socks5",
	}
	out, err := sharelink.RenderAll(model.NodeList{weird, ok})
	if err != nil {
		t.Fatalf("不支持的协议不应中断整批导出: %v", err)
	}
	if out.Lines != 1 {
		t.Errorf("期望 1 行，实际 %d", out.Lines)
	}
	// 被跳过的节点必须记名：静默丢节点等于让用户以为节点都还在。
	if len(out.Skipped) != 1 || out.Skipped[0] != "w" {
		t.Errorf("Skipped = %v，应记录被跳过的节点名", out.Skipped)
	}
}

func mustParse(t *testing.T, buf []byte) []map[string]any {
	t.Helper()
	nodes, err := parse.DecodeSubscription(buf)
	if err != nil {
		t.Fatalf("解析夹具失败: %v", err)
	}
	return nodes
}

func mustSerialize(t *testing.T, nodes []map[string]any) []byte {
	t.Helper()
	out, err := sharelink.RenderAll(model.NewNodeList(nodes, ""))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if len(out.Skipped) > 0 {
		t.Fatalf("有节点未能序列化: %v", out.Skipped)
	}
	return out.Body
}

// dump 按键字典序输出 map，便于失败时肉眼定位差异。
func dump(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString("  ")
		b.WriteString(k)
		b.WriteString(" = ")
		b.WriteString(reflect.ValueOf(m[k]).String())
		b.WriteByte('\n')
	}
	return b.String()
}
