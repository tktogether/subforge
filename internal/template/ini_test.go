package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustParse(t *testing.T, s string) *Config {
	t.Helper()
	cfg, err := ParseString(s, "test.ini")
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	return cfg
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustParseFile(t *testing.T, p string) *Config {
	t.Helper()
	cfg, err := ParseFile(p)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", p, err)
	}
	return cfg
}

func TestParse_Basic(t *testing.T) {
	cfg := mustParse(t, `
# 顶层注释
[common]
!emoji = 1
!remote = 1
sort = name

[node_pref]
include = HK;JP
exclude = - 02
`)

	if got := cfg.SectionNames(); len(got) != 2 || got[0] != "common" || got[1] != "node_pref" {
		t.Errorf("段落顺序 = %v", got)
	}
	if v, ok := cfg.Get("common", "!emoji"); !ok || v.Raw != "1" {
		t.Errorf("!emoji = %+v, %v", v, ok)
	}
	if got := cfg.Scalar("common", "sort", ""); got != "name" {
		t.Errorf("sort = %q", got)
	}
	if got := cfg.Scalar("node_pref", "include", ""); got != "HK;JP" {
		t.Errorf("include = %q", got)
	}
	if got := cfg.Scalar("node_pref", "exclude", ""); got != "- 02" {
		t.Errorf("exclude = %q（前后空白应去掉）", got)
	}
	if cfg.Lookup("nope") != nil {
		t.Error("Lookup 不存在的段落应返回 nil")
	}
	if _, ok := cfg.Get("nope", "x"); ok {
		t.Error("不存在的段落取键应为 false")
	}
}

// TestParse_Comments '#' 任意位置起注释，';' 只在行首起注释。
func TestParse_Comments(t *testing.T) {
	cfg := mustParse(t, `
[common]
a = 1 # 行内注释
b = keep;this	# 分号是值的一部分
; 整行注释
c = 3
`)
	// 值尾部的空白原样保留（去掉注释后剩下的就是值的全部），
	// 因为 [node_pref] 的前缀靠尾部空格和节点名隔开；需要去空白的地方自己 TrimSpace。
	if got := cfg.Scalar("common", "a", ""); got != "1 " {
		t.Errorf("a = %q，应为 \"1 \"（保留注释前的空格）", got)
	}
	if got := cfg.Scalar("common", "b", ""); got != "keep;this\t" {
		t.Errorf("b = %q（分号不应被当注释）", got)
	}
	if cfg.Lookup("common").Has("c") != true {
		t.Error("c 应被解析")
	}
	if _, ok := cfg.Get("common", "; 整行注释"); ok {
		t.Error("整行分号注释不应成为键")
	}
}

// TestParse_ValueKeepsInternalWhitespace 值内部的空白与中文都要原样保留。
func TestParse_ValueKeepsInternalWhitespace(t *testing.T) {
	cfg := mustParse(t, "[rename]\n 规则A = 🇭🇰 HK 香港 - 10Gbps\n")
	if got := cfg.Scalar("rename", "规则A", ""); got != "🇭🇰 HK 香港 - 10Gbps" {
		t.Errorf("值 = %q", got)
	}
}

func TestParse_ColonSeparator(t *testing.T) {
	cfg := mustParse(t, "[common]\nname: 我的配置\nurl = http://a/b\n")
	if got := cfg.Scalar("common", "name", ""); got != "我的配置" {
		t.Errorf("冒号分隔符: %q", got)
	}
	if got := cfg.Scalar("common", "url", ""); got != "http://a/b" {
		t.Errorf("等号优先: %q", got)
	}
}

// TestParse_DirectiveKeysNotConfused '!emoji' 之类是普通键，不能误当 !include。
func TestParse_DirectiveKeysNotConfused(t *testing.T) {
	cfg := mustParse(t, "[common]\n!emoji = 1\n!emoji- = 2\n!remote = 3\n!node-order = 1\n")
	s := cfg.Lookup("common")
	for _, k := range []string{"!emoji", "!emoji-", "!remote", "!node-order"} {
		if !s.Has(k) {
			t.Errorf("%q 应被当成普通键，实际段落 = %v", k, s.Keys())
		}
	}
}

// TestParse_DuplicateKeyLastWins 后者覆盖值，但键位置保留。
func TestParse_DuplicateKeyLastWins(t *testing.T) {
	cfg := mustParse(t, "[common]\na = 1\nb = 2\nc = 3\na = 9\n")
	if got := cfg.Scalar("common", "a", ""); got != "9" {
		t.Errorf("a = %q，应为后出现的 9", got)
	}
	if keys := cfg.Lookup("common").Keys(); strings.Join(keys, ",") != "a,b,c" {
		t.Errorf("键顺序应保留首次出现位置: %v", keys)
	}
}

func TestValue_SourceAndLine(t *testing.T) {
	cfg := mustParse(t, "[common]\n\nname = 测试配置\n")
	v, ok := cfg.Get("common", "name")
	if !ok {
		t.Fatal("缺少 name")
	}
	if v.Source != "test.ini" || v.Line != 3 {
		t.Errorf("来源 = %+v，应为 test.ini 第 3 行", v)
	}
}

// --- 错误路径 ---

func TestParse_Errors(t *testing.T) {
	cases := map[string]string{
		"键在段落之前": "name = x\n[common]\n",
		"无法解析的行": "[common]\n这不是键值对\n",
		"空段落名":   "[]\n",
		"未闭合段落":  "[common\nname = 1\n",
	}
	for want, body := range cases {
		if _, err := ParseString(body, "t.ini"); err == nil {
			t.Errorf("%s: 应报错", want)
		}
	}
}

// --- !include ---

func TestInclude_TopLevel(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "child.ini", "[common]\nfrom_child = yes\n[child]\nonly = here\n")
	root := write(t, dir, "root.ini", "!include child.ini\n[common]\nfrom_root = yes\n")

	cfg := mustParseFile(t, root)
	if got := cfg.Scalar("common", "from_child", ""); got != "yes" {
		t.Errorf("from_child = %q", got)
	}
	if got := cfg.Scalar("child", "only", ""); got != "here" {
		t.Errorf("child 段落未合并: %q", got)
	}
	if got := cfg.Scalar("common", "from_root", ""); got != "yes" {
		t.Errorf("from_root = %q", got)
	}
	// 来源应指向实际定义处，而不是引用处。
	if v, _ := cfg.Get("common", "from_child"); v.Source != "child.ini" {
		t.Errorf("Value.Source = %q，应指向 child.ini", v.Source)
	}
}

func TestInclude_InSection(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.ini", "[common]\nshared = 1\n[私有段]\nno = 1\n")
	root := write(t, dir, "root.ini", "[common]\n!import rules.ini\nown = 2\n")

	cfg := mustParseFile(t, root)
	if got := cfg.Scalar("common", "shared", ""); got != "1" {
		t.Errorf("同名段落应被合并: shared=%q", got)
	}
	if cfg.Has("私有段") {
		t.Error("段内 !import 不应把整份配置灌进来")
	}
}

// TestInclude_NotADirectiveInSection 段内 "!include" 是普通键，不是指令。
// subconverter 的 [common] 用 !include / !exclude 做节点名过滤，误判会去读文件。
func TestInclude_NotADirectiveInSection(t *testing.T) {
	cfg := mustParse(t, "[common]\n!include = .*HK.*\n!exclude = - 02$\n")
	if got := cfg.Scalar("common", "!include", ""); got != ".*HK.*" {
		t.Errorf("!include = %q", got)
	}
	if got := cfg.Scalar("common", "!exclude", ""); got != "- 02$" {
		t.Errorf("!exclude = %q", got)
	}
}

func TestInclude_NestedRelativePaths(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, sub, "deep.ini", "[common]\ndeep = 42\n")
	write(t, dir, "mid.ini", "!include sub/deep.ini\n[mid]\nmid = 1\n")
	root := write(t, dir, "root.ini", "!include mid.ini\n")

	cfg := mustParseFile(t, root)
	if got := cfg.Scalar("common", "deep", ""); got != "42" {
		t.Errorf("相对路径未正确解析: deep=%q", got)
	}
	if !cfg.Has("mid") {
		t.Error("mid 段落丢失")
	}
}

func TestInclude_MissingFile(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "root.ini", "!include nowhere.ini\n")
	if _, err := ParseFile(p); err == nil {
		t.Error("引用不存在的文件应报错")
	}
}

func TestInclude_Cycle(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.ini", "!include b.ini\n")
	write(t, dir, "b.ini", "[common]\n!include a.ini\n")
	if _, err := ParseFile(filepath.Join(dir, "a.ini")); err == nil {
		t.Error("循环引用应报错")
	}
}

func TestInclude_SelfReference(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "self.ini", "[common]\n!include self.ini\n")
	if _, err := ParseFile(p); err == nil {
		t.Error("自引用应报错")
	}
}

func TestInclude_DiamondNoError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "base.ini", "[common]\nbase = 1\n")
	write(t, dir, "left.ini", "!include base.ini\n[left]\nl = 1\n")
	write(t, dir, "right.ini", "!include base.ini\n[right]\nr = 1\n")
	root := write(t, dir, "root.ini", "!include left.ini\n!include right.ini\n")

	cfg := mustParseFile(t, root)
	// 菱形引用不是循环：同一文件被引用两次应当成功。
	if !cfg.Has("left") || !cfg.Has("right") {
		t.Errorf("段落丢失: %v", cfg.SectionNames())
	}
}

func TestInclude_AllowEmptyTargetRejected(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "root.ini", "!include   \n")
	if _, err := ParseFile(p); err == nil {
		t.Error("空的 !include 目标应报错")
	}
}

// --- Overlay / 覆盖链 ---

func TestOverlay_LowerPriorityFirst(t *testing.T) {
	base := mustParse(t, "[common]\na = base\nb = base\n[only_base]\nx = 1\n")
	override := mustParse(t, "[common]\nb = override\n")

	base.Overlay(override)
	if got := base.Scalar("common", "a", ""); got != "base" {
		t.Errorf("未覆盖的键应保留: a=%q", got)
	}
	if got := base.Scalar("common", "b", ""); got != "override" {
		t.Errorf("应被覆盖: b=%q", got)
	}
	if !base.Has("only_base") {
		t.Error("低优先级独有段落应保留")
	}
}

func TestOverlay_DoesNotCreateEmptySections(t *testing.T) {
	a := &Config{sections: map[string]*Section{}}
	b := mustParse(t, "[common]\nk = v\n")
	a.Overlay(b)
	if got := a.Scalar("common", "k", ""); got != "v" {
		t.Errorf("Overlay 结果: %q", got)
	}
	if !a.Has("common") {
		t.Error("common 段落应存在")
	}
}

// --- [clash_proxy_group] 的 "---" 分组分隔符 ---

// TestParse_GroupSeparator 单独一行的 "---" 只在 [clash_proxy_group] 里是分隔符。
func TestParse_GroupSeparator(t *testing.T) {
	cfg := mustParse(t, `[clash_proxy_group]
name = 自动选择
type = url-test
interval = 300
proxies = .*港.*
---
name = 手动选择
type = select
proxies = DIRECT;.*港.*
`)

	blocks := cfg.Lookup("clash_proxy_group").GroupBlocks()
	if len(blocks) != 2 {
		t.Fatalf("应切出 2 个代理组，实际 %d", len(blocks))
	}
	first := map[string]string{}
	for _, e := range blocks[0] {
		first[e.Key] = e.Value.Raw
	}
	if first["name"] != "自动选择" || first["type"] != "url-test" || first["interval"] != "300" ||
		first["proxies"] != ".*港.*" {
		t.Errorf("第一个组 = %v", first)
	}
	second := map[string]string{}
	for _, e := range blocks[1] {
		second[e.Key] = e.Value.Raw
	}
	if second["name"] != "手动选择" || second["type"] != "select" || second["proxies"] != "DIRECT;.*港.*" {
		t.Errorf("第二个组 = %v", second)
	}
	// 分隔符的内部键名带 NUL 前缀，不会被误读成用户写的配置键。
	for _, k := range cfg.Lookup("clash_proxy_group").Keys() {
		if k == "group" {
			t.Errorf("不应出现键 %q", k)
		}
	}
}

// TestParse_GroupSeparatorTrailingAndRepeated 尾随与连续分隔符不应产生空块。
func TestParse_GroupSeparatorTrailingAndRepeated(t *testing.T) {
	cfg := mustParse(t, "[clash_proxy_group]\nname = a\ntype = select\n---\n---\nname = b\ntype = select\n---\n")
	blocks := cfg.Lookup("clash_proxy_group").GroupBlocks()
	if len(blocks) != 2 {
		t.Fatalf("空块应被丢弃，实际块数 %d", len(blocks))
	}
}

// TestParse_GroupSeparatorOutsideGroupSection 其它段落里的 "---" 仍是语法错误。
func TestParse_GroupSeparatorOutsideGroupSection(t *testing.T) {
	if _, err := ParseString("[common]\n---\nname = x\n", "t.ini"); err == nil {
		t.Error("非 [clash_proxy_group] 段落里的 --- 应报错")
	}
	if _, err := ParseString("[CLASH_PROXY_GROUP]\nname = a\ntype = select\n---\nname = b\n", "t.ini"); err != nil {
		t.Errorf("段落名应大小写不敏感: %v", err)
	}
}
