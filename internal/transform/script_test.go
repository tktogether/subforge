package transform

import (
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/model"
)

func scriptNodes() model.NodeList {
	return model.NodeList{
		model.NewNodeFromParts("HK-1", "vless", "hk1.example.com", "443", map[string]any{"sni": "a.com"}),
		model.NewNodeFromParts("JP-1", "vmess", "jp1.example.com", "8443", map[string]any{"sni": "b.com"}),
		model.NewNodeFromParts("US-1", "trojan", "us1.example.com", "443", map[string]any{"sni": "c.com"}),
	}
}

func TestScriptFilter_Boolean(t *testing.T) {
	f, err := NewScriptFilter("only-vless", `node.type === "vless"`, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Apply(scriptNodes())
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "HK-1")
}

// 脚本可读取内核写入的任意 raw 字段。
func TestScriptFilter_ReadsRaw(t *testing.T) {
	f, err := NewScriptFilter("sni", `node.raw.sni === "b.com"`, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Apply(scriptNodes())
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "JP-1")
}

func TestScriptOperate_Rename(t *testing.T) {
	o, err := NewScriptOperate("prefix", `node.name = "[" + node.type + "] " + node.name`, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err := o.Apply(scriptNodes())
	if err != nil {
		t.Fatal(err)
	}
	assertNames(t, out, "[vless] HK-1", "[vmess] JP-1", "[trojan] US-1")
	// 改名必须写回 Raw（渲染取 Raw）
	if out[0].Raw["name"] != "[vless] HK-1" {
		t.Fatalf("Raw 未更新: %q", out[0].Raw["name"])
	}
}

// 脚本改 node.raw.* 要写回节点原始配置，且只影响被改动的节点。
func TestScriptOperate_WriteRaw(t *testing.T) {
	o, err := NewScriptOperate("sni", `if (node.name === "HK-1") { node.raw.sni = "x.example.com" }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err := o.Apply(scriptNodes())
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Raw["sni"] != "x.example.com" {
		t.Fatalf("raw.sni 未更新: %v", out[0].Raw["sni"])
	}
	// 未命中的节点保持原值（节点间不共享 raw）
	if out[1].Raw["sni"] != "b.com" {
		t.Fatalf("其他节点被污染: %v", out[1].Raw["sni"])
	}
}

// 脚本语法错误在构造期暴露，而不是等到请求期。
func TestScript_CompileError(t *testing.T) {
	if _, err := NewScriptFilter("bad", `node.type ===`, 0); err == nil {
		t.Error("非法过滤脚本应在构造期报错")
	}
	if _, err := NewScriptOperate("bad", `function(`, 0); err == nil {
		t.Error("非法操作脚本应在构造期报错")
	}
	if _, err := NewScriptFilter("empty", "   ", 0); err == nil {
		t.Error("空脚本应报错")
	}
}

// 死循环由超时打断，不拖垮请求。
func TestScript_Timeout(t *testing.T) {
	o, err := NewScriptOperate("loop", `while (true) { }`, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := o.Apply(scriptNodes()); err == nil {
		t.Fatal("死循环应被超时打断")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("超时未生效，耗时 %v", elapsed)
	}
}

// 脚本运行期抛异常作为错误返回，不 crash 主进程。
func TestScript_RuntimeError(t *testing.T) {
	o, err := NewScriptOperate("boom", `throw new Error("炸了")`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Apply(scriptNodes()); err == nil {
		t.Fatal("脚本抛异常应作为错误返回")
	}
}

// 沙箱里不应存在 require / process / fetch 等宿主能力。
func TestScript_SandboxNoGlobals(t *testing.T) {
	f, err := NewScriptFilter("sandbox",
		`typeof require === "undefined" && typeof process === "undefined" && typeof fetch === "undefined"`, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Apply(scriptNodes())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("沙箱中不应存在 require/process/fetch，全部节点应保留: %d", len(out))
	}
}
