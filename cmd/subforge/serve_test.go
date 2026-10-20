package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvOr(t *testing.T) {
	const key = "SUBFORGE_TEST_ENVOR"
	cases := []struct {
		name     string
		value    string
		set      bool
		fallback string
		want     string
	}{
		{name: "未设置用兜底值", set: false, fallback: "127.0.0.1:25500", want: "127.0.0.1:25500"},
		{name: "空串用兜底值", value: "", set: true, fallback: "/data", want: "/data"},
		{name: "只有空白也用兜底值", value: "   \t ", set: true, fallback: "/data", want: "/data"},
		{name: "设置后用环境变量的值", value: "0.0.0.0:25500", set: true, fallback: "127.0.0.1:25500", want: "0.0.0.0:25500"},
		{name: "两端空白被裁掉", value: "  /srv/subforge  ", set: true, fallback: "", want: "/srv/subforge"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// t.Setenv 会在子测试结束时自动还原，也能阻止并行测试互相干扰。
			t.Setenv(key, tc.value)
			if !tc.set {
				// 变量可能被外部环境预设过，这里显式清掉再测。
				if err := os.Unsetenv(key); err != nil {
					t.Fatalf("Unsetenv: %v", err)
				}
			}
			if got := envOr(key, tc.fallback); got != tc.want {
				t.Errorf("envOr(%q, %q) = %q, want %q", key, tc.fallback, got, tc.want)
			}
		})
	}
}

// serve 读取的四个环境变量必须与文档、容器镜像里写的一致。名字写错不会报错，
// 只会静默退回默认值（订阅管理不启用、监听回环），因此这里把名字钉住：
// 测试里设置它们，再确认 envOr 真的读得到。
func TestServeEnvVarNames(t *testing.T) {
	for _, key := range []string{
		"SUBFORGE_LISTEN",
		"SUBFORGE_TOKEN",
		"SUBFORGE_CONFIG",
		"SUBFORGE_DATA_DIR",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "/from-env")
			if got := envOr(key, "/fallback"); got != "/from-env" {
				t.Errorf("envOr(%q) = %q，want %q", key, got, "/from-env")
			}
		})
	}
}

// serveUsage 是用户看到的第一手说明，必须提到这四个环境变量，
// 否则容器里「设置了环境变量却没生效」无从查起。
func TestServeUsageDocumentsEnvVars(t *testing.T) {
	for _, key := range []string{
		"SUBFORGE_LISTEN",
		"SUBFORGE_TOKEN",
		"SUBFORGE_CONFIG",
		"SUBFORGE_DATA_DIR",
	} {
		if !strings.Contains(serveUsage, key) {
			t.Errorf("serveUsage 未说明环境变量 %s", key)
		}
	}
}

// 数据目录不存在时 serve 要自己建出来，而不是让用户先手工 mkdir。
func TestEnsureDataDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "data")

	if err := ensureDataDir(nested); err != nil {
		t.Fatalf("ensureDataDir 应能建出多级目录: %v", err)
	}
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s 不是目录", nested)
	}

	// 已存在时重复调用也必须成功（每次启动都会走一遍）。
	if err := ensureDataDir(nested); err != nil {
		t.Fatalf("目录已存在时 ensureDataDir 不应报错: %v", err)
	}

	// 父路径被一个普通文件占住时应当报错，而不是静默继续。
	blocked := filepath.Join(base, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	err = ensureDataDir(filepath.Join(blocked, "data"))
	if err == nil {
		t.Fatal("父路径是文件时 ensureDataDir 应当报错")
	}
	if !strings.Contains(err.Error(), "创建数据目录") {
		t.Errorf("错误信息应说明是创建数据目录失败，实际: %v", err)
	}
}
