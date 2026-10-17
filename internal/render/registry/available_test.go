package registry

import (
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/render/clashmeta"
)

// TestAvailable_Deterministic 名单必须逐次稳定。
//
// 尾段由 render 包的四张 map 展开，Go 的 map 迭代顺序是随机的：不排序的话
// /api/targets 每次请求顺序都不同（前端下拉框抖动），/sub 的 501 错误信息里
// target 清单也乱序，任何断言全序的测试会随机红。
func TestAvailable_Deterministic(t *testing.T) {
	first := Available()
	if len(first) == 0 {
		t.Fatal("Available() 不应为空")
	}
	for i := 0; i < 20; i++ {
		got := Available()
		if len(got) != len(first) {
			t.Fatalf("第 %d 次调用长度 = %d，want %d", i+2, len(got), len(first))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("第 %d 次调用顺序不稳定：位置 %d 是 %q，want %q\nfirst=%v\ngot  =%v",
					i+2, j, got[j], first[j], first, got)
			}
		}
	}
}

// TestAvailable_Shape 名单形状：手写头部顺序有意义（常用写法优先），
// 整份名单去重、全小写，且每一项都能过 Lookup（照抄提示不能踩空）。
func TestAvailable_Shape(t *testing.T) {
	names := Available()
	head := []string{"clash.meta", "surge", "quan", "quanx", "loon", "surfboard", "mellow"}
	if len(names) < len(head) {
		t.Fatalf("Available() 太短: %v", names)
	}
	for i, h := range head {
		if names[i] != h {
			t.Errorf("头部第 %d 项 = %q，want %q", i, names[i], h)
		}
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("target %q 重复出现", n)
		}
		seen[n] = true
		if n != strings.ToLower(n) {
			t.Errorf("target %q 应是小写规范写法", n)
		}
		if _, err := Lookup(n, clashmeta.Options{}); err != nil {
			t.Errorf("Available() 列出的 target %q 无法 Lookup: %v", n, err)
		}
	}
	// 尾段是各族的展开，不保证字典序（别名跟随规范名），但必须覆盖全部真源表。
	for _, want := range []string{"clash-json", "singbox", "mellow-ios", "surge4", "quantumultx", "ss-sub-b64"} {
		if !seen[want] {
			t.Errorf("Available() 缺少 target %q", want)
		}
	}
}
