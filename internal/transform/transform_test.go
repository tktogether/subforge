package transform

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
)

// sampleNodes 造一组贴近真实机场订阅的节点（地区前缀 + 协议 + 速率）。
func sampleNodes() model.NodeList {
	profs := []struct {
		name, typ, server, port string
	}{
		{"HK-香港-01 - 10Gbps", "vless", "hk1.example.com", "443"},
		{"HK-香港-02 - 10Gbps", "vmess", "hk2.example.com", "8443"},
		{"JP-日本-01 - 5Gbps", "vless", "jp1.example.com", "443"},
		{"TW-台湾-01 - 3Gbps", "trojan", "tw1.example.com", "443"},
		{"US-美国-01 - 2Gbps", "hysteria2", "us1.example.com", "8843"},
		{"SG-新加坡-01 - 2Gbps", "vless", "sg1.example.com", "443"},
		{"HK-香港-02 - 10Gbps", "vmess", "hk2.example.com", "8443"}, // 与 [1] 完全重复
	}
	ns := make(model.NodeList, 0, len(profs))
	for _, p := range profs {
		ns = append(ns, model.NewNodeFromParts(p.name, p.typ, p.server, p.port,
			map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"}))
	}
	return ns
}

func assertNames(t *testing.T, ns model.NodeList, want ...string) {
	t.Helper()
	got := ns.Names()
	if len(got) != len(want) {
		t.Fatalf("节点数 = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 项: %q, want %q\n全部: %v", i, got[i], want[i], got)
		}
	}
}

// failingStage 用于验证管线错误信息带上步序号。
type failingStage struct{}

func (failingStage) Apply(model.NodeList) (model.NodeList, error) {
	return nil, errors.New("这一步坏了")
}

func TestPipeline_OrderAndNoMutation(t *testing.T) {
	in := sampleNodes()
	before := in.Names()

	remove := NewRemove([]string{"US-美国-01 - 2Gbps"})
	region, err := NewRegionFilter([]string{"hk", "jp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rename, err := NewRename([]RegexPair{{Pattern: ` - .*$`, Replacement: ""}})
	if err != nil {
		t.Fatal(err)
	}
	emoji, err := NewEmoji([]RegexPair{
		{Pattern: `^HK`, Replacement: "🇭🇰"},
		{Pattern: `^JP`, Replacement: "🇯🇵"},
	})
	if err != nil {
		t.Fatal(err)
	}
	sortStep, err := NewSort(SortByName)
	if err != nil {
		t.Fatal(err)
	}

	out, err := Pipeline{remove, region, rename, emoji, Dedupe{}, sortStep}.Apply(in)
	if err != nil {
		t.Fatalf("管线执行失败: %v", err)
	}
	// 移除 US → 地区 hk/jp → 截断 " - 速率" → emoji → 去重（hk2 重复）→ 按名排序
	assertNames(t, out, "🇭🇰HK-香港-01", "🇭🇰HK-香港-02", "🇯🇵JP-日本-01")

	// 管线不得修改入参（Raw 是共享的事实源）
	if got := in.Names(); !equalStrings(got, before) {
		t.Fatalf("管线修改了入参:\n before=%v\n after =%v", before, got)
	}
}

func TestPipeline_ReportsStepOnError(t *testing.T) {
	f, err := NewRegexFilter(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Pipeline{f, failingStage{}}.Apply(sampleNodes())
	if err == nil {
		t.Fatal("管线中一步失败应返回错误")
	}
	if !strings.Contains(err.Error(), "管线第 2 步") {
		t.Fatalf("错误信息应定位到步序号: %v", err)
	}

	empty, err := Pipeline{}.Apply(nil)
	if err != nil {
		t.Fatalf("空管线不应报错: %v", err)
	}
	if empty.Len() != 0 {
		t.Fatal("空管线应返回空列表")
	}
}

func TestRegexFilter(t *testing.T) {
	in := sampleNodes()

	f, err := NewRegexFilter([]string{`^(HK|JP)`}, []string{`02`})
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "HK-香港-01 - 10Gbps", "JP-日本-01 - 5Gbps")

	// 排除优先于白名单：HK-02 在白名单内但被排除
	f2, err := NewRegexFilter([]string{`HK`}, []string{`^HK.*02`})
	if err != nil {
		t.Fatal(err)
	}
	out2, err := f2.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out2, "HK-香港-01 - 10Gbps")

	// 空白条件不过滤
	f3, err := NewRegexFilter(nil, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	out3, err := f3.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if out3.Len() != in.Len() {
		t.Fatalf("空条件不应过滤: %d vs %d", out3.Len(), in.Len())
	}
}

func TestRegionFilter(t *testing.T) {
	in := sampleNodes()
	f, err := NewRegionFilter([]string{"hK"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "HK-香港-01 - 10Gbps", "HK-香港-02 - 10Gbps", "HK-香港-02 - 10Gbps")

	f2, err := NewRegionFilter(nil, []string{"hk"})
	if err != nil {
		t.Fatal(err)
	}
	out2, err := f2.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if out2.Len() != in.Len()-3 {
		t.Fatalf("排除 hk 后应有 %d 个节点，实际 %d: %v", in.Len()-3, out2.Len(), out2.Names())
	}
}

func TestTypeFilter(t *testing.T) {
	in := sampleNodes()
	f, err := NewTypeFilter([]string{"VLESS"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 3 {
		t.Fatalf("vless 节点数 = %d: %v", out.Len(), out.Names())
	}
	for _, n := range out {
		if n.Type != "vless" {
			t.Errorf("混入非 vless 节点: %q type=%q", n.Name, n.Type)
		}
	}

	f2, err := NewTypeFilter(nil, []string{"vmess", "vless"})
	if err != nil {
		t.Fatal(err)
	}
	out2, err := f2.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if out2.Len() != 2 {
		t.Fatalf("排除 vmess/vless 后应有 2 个节点，实际 %d: %v", out2.Len(), out2.Names())
	}
}

// TestTypeFilter_Aliases 用户按常见长名写 -type 时必须能命中内核真名。
// 这些名字曾经会静默匹配零节点，请求退化成"没有可用节点"，用户无从判断是拼错还是没节点。
func TestTypeFilter_Aliases(t *testing.T) {
	in := model.NodeList{
		model.NewNodeFromParts("ss-1", "ss", "a.example.com", "8388", nil),
		model.NewNodeFromParts("ssr-1", "ssr", "b.example.com", "443", nil),
		model.NewNodeFromParts("socks-1", "socks5", "c.example.com", "1080", nil),
		model.NewNodeFromParts("http-1", "http", "d.example.com", "8888", nil),
		model.NewNodeFromParts("hy2-1", "hysteria2", "e.example.com", "443", nil),
		model.NewNodeFromParts("vm-1", "vmess", "f.example.com", "443", nil),
	}
	cases := []struct {
		name string
		in   []string
		want int
	}{
		{"shadowsocks", []string{"shadowsocks"}, 1},
		{"shadowsocksr", []string{"shadowsocksr"}, 1},
		{"socks", []string{"socks"}, 1},
		{"https", []string{"https"}, 1},
		{"hy2", []string{"hy2"}, 1},
		{"v2ray", []string{"v2ray"}, 1},
		{"SS 大小写不敏感", []string{"SS"}, 1},
		{"原生内核名不受影响", []string{"ss"}, 1},
		{"未知类型拼错 → 空结果", []string{"vles"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := NewTypeFilter(c.in, nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := f.Apply(in)
			if err != nil {
				t.Fatal(err)
			}
			if out.Len() != c.want {
				t.Fatalf("%v → %d 个节点，期望 %d: %v", c.in, out.Len(), c.want, out.Names())
			}
		})
	}
}

func TestRename_Sequential(t *testing.T) {
	in := model.NodeList{
		model.NewNodeFromParts("HK-香港-01 - 10Gbps", "vless", "hk1.example.com", "443", nil),
	}
	r, err := NewRename([]RegexPair{
		{Pattern: ` - .*$`, Replacement: ""},
		{Pattern: `HK-(香港)-(\d+)`, Replacement: "香港[$1]-$2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "香港[香港]-01")

	// 改名必须写回 Raw（渲染取 Raw）
	if out[0].Raw["name"] != "香港[香港]-01" {
		t.Fatalf("Raw 未更新: %q", out[0].Raw["name"])
	}
	if in[0].Name != "HK-香港-01 - 10Gbps" {
		t.Fatalf("入参被修改: %q", in[0].Name)
	}
}

func TestEmoji_FirstMatchWins(t *testing.T) {
	in := sampleNodes()
	e, err := NewEmoji([]RegexPair{
		{Pattern: `HK`, Replacement: "🇭🇰"},
		{Pattern: `HK`, Replacement: "🚫"}, // 第二条不应生效
		{Pattern: `^JP`, Replacement: "🇯🇵"},
		{Pattern: `^SG`, Replacement: "🇸🇬"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out[0].Name, "🇭🇰") {
		t.Fatalf("第一条规则应生效: %q", out[0].Name)
	}
	if !strings.HasPrefix(out[2].Name, "🇯🇵") {
		t.Fatalf("JP 应命中: %q", out[2].Name)
	}
	if !strings.HasPrefix(out[5].Name, "🇸🇬") {
		t.Fatalf("SG 应命中: %q", out[5].Name)
	}
	if strings.HasPrefix(out[4].Name, "🇭🇰") || strings.HasPrefix(out[4].Name, "🇯🇵") {
		t.Fatalf("US 不应命中任何规则: %q", out[4].Name)
	}
}

func TestSplit(t *testing.T) {
	in := model.NodeList{
		model.NewNodeFromParts("HK-香港-01 - 10Gbps", "vless", "hk1.example.com", "443", nil),
		model.NewNodeFromParts("NOSEP-节点", "vless", "a.example.com", "443", nil),
		model.NewNodeFromParts(" - 只有分隔符", "vless", "b.example.com", "443", nil),
	}
	out, err := NewSplit("").Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "HK-香港-01", "NOSEP-节点", " - 只有分隔符")

	out2, err := NewSplit("-").Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if out2[0].Name != "HK" {
		t.Fatalf("按 '-' 截断: %q", out2[0].Name)
	}
}

func TestSort(t *testing.T) {
	in := model.NodeList{
		model.NewNodeFromParts("c", "vless", "c.example.com", "443", nil),
		model.NewNodeFromParts("b", "vmess", "b.example.com", "8443", nil),
		model.NewNodeFromParts("a", "ss", "a.example.com", "8388", nil),
	}
	cases := []struct {
		name   string
		by     SortBy
		want   []string
		errMsg string
	}{
		{"按名称", SortByName, []string{"a", "b", "c"}, ""},
		// 类型字典序 ss < vless < vmess → a(ss), c(vless), b(vmess)
		{"按类型", SortByType, []string{"a", "c", "b"}, ""},
		// 端口升序 443(c) < 8388(a) < 8443(b)
		{"按端口", SortByPort, []string{"c", "a", "b"}, ""},
		{"按服务器", SortByServer, []string{"a", "b", "c"}, ""},
		{"原顺序", SortByOriginal, []string{"c", "b", "a"}, ""},
		{"非法键", "bogus", nil, "未知排序键"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step, err := NewSort(tc.by)
			if tc.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("应报 %q，实际 %v", tc.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := step.Apply(in)
			if err != nil {
				t.Fatal(err)
			}
			assertNames(t, out, tc.want...)
		})
	}
}

func TestDedupe(t *testing.T) {
	in := sampleNodes()
	out, err := Dedupe{}.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != in.Len()-1 {
		t.Fatalf("应去掉 1 个重复节点: %d vs %d", out.Len(), in.Len())
	}
}

func TestConstructorsRejectBadRegex(t *testing.T) {
	if _, err := NewRegexFilter([]string{"["}, nil); err == nil {
		t.Error("NewRegexFilter 应拒绝非法正则")
	}
	if _, err := NewRegexFilter(nil, []string{"["}); err == nil {
		t.Error("NewRegexFilter(exclude) 应拒绝非法正则")
	}
	if _, err := NewRename([]RegexPair{{Pattern: "(", Replacement: "x"}}); err == nil {
		t.Error("NewRename 应拒绝非法正则")
	}
	if _, err := NewEmoji([]RegexPair{{Pattern: "(", Replacement: "x"}}); err == nil {
		t.Error("NewEmoji 应拒绝非法正则")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BenchmarkTransform1000 覆盖过滤+重命名+emoji+去重+排序的整链成本
// （对齐设计书 §8 的 1000 节点 P99<50ms 预算）。
func BenchmarkTransform1000(b *testing.B) {
	r, err := NewRename([]RegexPair{{Pattern: ` - .*$`, Replacement: ""}})
	if err != nil {
		b.Fatal(err)
	}
	e, err := NewEmoji([]RegexPair{
		{Pattern: `^hk`, Replacement: "🇭🇰"},
		{Pattern: `^jp`, Replacement: "🇯🇵"},
		{Pattern: `^us`, Replacement: "🇺🇸"},
	})
	if err != nil {
		b.Fatal(err)
	}
	ns := model.NodeList{}
	for i := 0; i < 1000; i++ {
		region := []string{"hk", "jp", "us"}[i%3]
		ns = append(ns, model.NewNodeFromParts(
			region+"-节点-"+strconv.Itoa(i)+" - 10Gbps", "vless",
			region+strconv.Itoa(i%50)+".example.com", "443",
			map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"}))
	}
	p := Pipeline{r, e, Dedupe{}, Sort{By: SortByName}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Apply(ns); err != nil {
			b.Fatal(err)
		}
	}
}

func TestNodePref_ByType(t *testing.T) {
	in := model.NodeList{
		model.NewNodeFromParts("HK-香港-01", "vless", "hk1.example.com", "443", nil),
		model.NewNodeFromParts("HK-香港-02", "vmess", "hk2.example.com", "8443", nil),
		model.NewNodeFromParts("JP-日本-01", "trojan", "jp1.example.com", "443", nil),
	}
	stage, err := NewNodePref(NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] ", "vmess": "[V2Ray] "}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := stage.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "[VLESS] HK-香港-01", "[V2Ray] HK-香港-02", "JP-日本-01")
}

// TestNodePref_IncludeExclude 前缀只作用于名称匹配白名单、且不匹配黑名单的节点。
func TestNodePref_IncludeExclude(t *testing.T) {
	in := model.NodeList{
		model.NewNodeFromParts("HK-香港-01", "vless", "hk1.example.com", "443", nil),
		model.NewNodeFromParts("JP-日本-01", "vless", "jp1.example.com", "443", nil),
	}
	stage, err := NewNodePref(NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] "}, Include: []string{`^HK`}, Exclude: []string{`-01$`}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := stage.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	// HK-香港-01 命中黑名单，JP-日本-01 不在白名单里，两个都不加前缀。
	assertNames(t, out, "HK-香港-01", "JP-日本-01")

	stage, err = NewNodePref(NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] "}, Include: []string{`^HK`}})
	if err != nil {
		t.Fatal(err)
	}
	out, err = stage.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "[VLESS] HK-香港-01", "JP-日本-01")
}

// TestNodePref_TypeKeyCaseInsensitive 协议类型大小写不敏感。
func TestNodePref_TypeKeyCaseInsensitive(t *testing.T) {
	in := model.NodeList{model.NewNodeFromParts("JP-日本-01", "VLESS", "jp1.example.com", "443", nil)}
	stage, err := NewNodePref(NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] "}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := stage.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "[VLESS] JP-日本-01")
}

// TestNodePref_Empty 没有前缀表时是空操作。
func TestNodePref_Empty(t *testing.T) {
	in := sampleNodes()
	stage, err := NewNodePref(NodePrefSpec{Include: []string{`^HK`}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := stage.Apply(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) {
		t.Fatalf("节点数变化: %d -> %d", len(in), len(out))
	}
}

// TestNodePref_BadRegex 正则写错在构造期暴露。
func TestNodePref_BadRegex(t *testing.T) {
	if _, err := NewNodePref(NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] "}, Include: []string{"("}}); err == nil {
		t.Error("非法正则应报错")
	}
	if _, err := NewNodePref(NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] "}, Exclude: []string{"["}}); err == nil {
		t.Error("非法正则应报错")
	}
}
