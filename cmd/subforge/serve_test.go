package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/runtime"
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

// waitFor 轮询等待 cond 成立，超过 timeout 报失败。watchConfig 里的
// runtime.Watch 轮询间隔固定 1 秒且要连续两轮读到相同摘要才交付，
// 所以不能用精确时序，只能这样等到条件满足。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// INI 热重载成功后必须回调注入的 onReload（serve 用它联动推进产物版本）。
// runtime.Watch 启动时会先拍一次基线回调，文件可解析时同样走 Store，因此
// onReload 会被提前触发一次——这次预热 bump 无害（产物版本只是缓存失效
// 计数，后台重建器本来启动时就要预热一轮），所以断言放宽为「至少一次」：
// 先等预热那一拍落稳记下基线，再改文件，断言基线之后又触发了新的回调。
func TestWatchConfigReloadCallsOnReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subforge.ini")
	initial := "[subforge]\ntoken = abc\nmixed_port = 1080\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var calls atomic.Int32
	bump := func() { calls.Add(1) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	base := config.Settings{Token: "abc", MixedPort: 1080}
	snap := runtime.NewSnapshot(liveSettings{Settings: base})
	go watchConfig(ctx, path, base, snap, bump)

	// 预热那一拍（启动基线回调）先落稳，之后的回调才对应真实的文件改动。
	waitFor(t, 5*time.Second, func() bool { return calls.Load() >= 1 },
		"启动基线回调未触发 onReload（5 秒超时）")
	baseline := calls.Load()

	// 改动配置（token 变长顺带改变了文件大小，避免文件系统 mtime 粒度粗时漏检）。
	changed := "[subforge]\ntoken = abcdef\nmixed_port = 1080\n"
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatalf("改写配置: %v", err)
	}

	waitFor(t, 10*time.Second, func() bool { return calls.Load() > baseline },
		"热重载成功后未触发 onReload（10 秒超时）")

	// onReload 在 snap.Store 之后调用，此刻快照必须已是新配置。
	if got := snap.Load().Token; got != "abcdef" {
		t.Errorf("重载后快照 token = %q, want %q", got, "abcdef")
	}
}

// onReload 允许为 nil（没有 -data-dir 时 serve 没有产物可 bump，传空即可）。
// 借已取消的 context 让 Watch 在首拍回调后立即退出，同步调用即可完成，
// 不需要等轮询 tick。
func TestWatchConfigNilOnReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subforge.ini")
	if err := os.WriteFile(path, []byte("[subforge]\ntoken = abc\nmixed_port = 1080\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	snap := runtime.NewSnapshot(liveSettings{})
	// 首拍回调会走到 onReload 判断处；为 nil 时不得 panic。
	watchConfig(ctx, path, config.Settings{}, snap, nil)
}
