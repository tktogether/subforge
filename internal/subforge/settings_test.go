package subforge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newSettingsManager 创建一个临时 SQLite 管理器（settings 专用）。
func newSettingsManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestListSettingsDefaults(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	items, err := m.ListSettings(ctx)
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	if len(items) != 6 {
		t.Fatalf("len = %d, want 6", len(items))
	}
	wantKeys := []string{"script.enabled", "subscheck.enabled", OutboundNodeKey, SourceRefreshKey, "rules.manifest_path", "rules.output_dir"}
	for i, k := range wantKeys {
		if items[i].Key != k {
			t.Fatalf("items[%d].Key = %q, want %q", i, items[i].Key, k)
		}
	}
	if items[0].Value != "false" || items[1].Value != "false" {
		t.Fatalf("bool 默认值应 false, got %q %q", items[0].Value, items[1].Value)
	}
	if items[2].Value != "" || items[3].Value != "" || items[4].Value != "" || items[5].Value != "" {
		t.Fatalf("默认值应空串, got %q %q %q %q", items[2].Value, items[3].Value, items[4].Value, items[5].Value)
	}
	if items[0].Kind != SettingKindBool || items[1].Kind != SettingKindBool {
		t.Errorf("bool kind: %q %q", items[0].Kind, items[1].Kind)
	}
	if items[2].Kind != SettingKindSelect {
		t.Errorf("outbound kind = %q", items[2].Kind)
	}
	if items[3].Kind != SettingKindNumber || items[3].Placeholder != "30" {
		t.Errorf("refresh kind/placeholder = %q/%q", items[3].Kind, items[3].Placeholder)
	}
	if items[4].Kind != SettingKindPath || items[5].Kind != SettingKindPath {
		t.Errorf("path kind: %q %q", items[4].Kind, items[5].Kind)
	}
	if len(items[2].Options) != 1 || items[2].Options[0].Value != "" {
		t.Errorf("无自建节点时只应有直连选项: %+v", items[2].Options)
	}
}

// TestSourceRefreshInterval 刷新间隔设置：默认 30 分钟、可设 0 关闭、越界报错；缓存读写走同一管理器。
func TestSourceRefreshInterval(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)

	d, err := m.SourceRefreshInterval(ctx)
	if err != nil || d != 30*time.Minute {
		t.Fatalf("默认间隔 = %v err=%v", d, err)
	}
	if err := m.SetSettings(ctx, map[string]string{SourceRefreshKey: "5"}); err != nil {
		t.Fatalf("SetSettings 5: %v", err)
	}
	if d, _ = m.SourceRefreshInterval(ctx); d != 5*time.Minute {
		t.Fatalf("5 分钟 = %v", d)
	}
	if err := m.SetSettings(ctx, map[string]string{SourceRefreshKey: "0"}); err != nil {
		t.Fatalf("SetSettings 0: %v", err)
	}
	if d, _ = m.SourceRefreshInterval(ctx); d != 0 {
		t.Fatalf("0 应关闭, got %v", d)
	}
	if err := m.SetSettings(ctx, map[string]string{SourceRefreshKey: ""}); err != nil {
		t.Fatalf("清空间隔: %v", err)
	}
	if d, _ = m.SourceRefreshInterval(ctx); d != 30*time.Minute {
		t.Fatalf("清空后应回默认 30 分钟, got %v", d)
	}
	for _, bad := range []string{"abc", "-1", "10081"} {
		if err := m.SetSettings(ctx, map[string]string{SourceRefreshKey: bad}); err == nil {
			t.Fatalf("%q 应被拒绝", bad)
		}
	}

	if err := m.setConfig(ctx, SourceRefreshKey, "abc"); err != nil {
		t.Fatalf("写入脏配置: %v", err)
	}
	if _, err := m.SourceRefreshInterval(ctx); err == nil {
		t.Fatal("非法间隔应报错")
	}
	if err := m.SetSettings(ctx, map[string]string{SourceRefreshKey: ""}); err != nil {
		t.Fatalf("清理脏配置: %v", err)
	}

	if err := m.PutSourceCache(ctx, "https://x/sub", []byte("content")); err != nil {
		t.Fatalf("PutSourceCache: %v", err)
	}
	content, _, ok, err := m.GetSourceCache(ctx, "https://x/sub")
	if err != nil || !ok || string(content) != "content" {
		t.Fatalf("缓存读回失败: ok=%v err=%v content=%q", ok, err, content)
	}
}

// TestOutboundNodeSetting 出网节点设置：候选来自自建节点、写入校验、停用后回落到直连。
func TestOutboundNodeSetting(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	n, err := m.AddSelfNode(ctx, "socks5://user:pass@127.0.0.1:1080", "测试节点", true, 0)
	if err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}
	unnamed, err := m.AddSelfNode(ctx, "socks5://user:pass@127.0.0.1:1081", "", false, 0)
	if err != nil {
		t.Fatalf("AddSelfNode unnamed: %v", err)
	}

	if err := m.SetSettings(ctx, map[string]string{OutboundNodeKey: fmt.Sprint(n.ID)}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	items, err := m.ListSettings(ctx)
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	var found *SettingItem
	for i := range items {
		if items[i].Key == OutboundNodeKey {
			found = &items[i]
		}
	}
	if found == nil || found.Value != fmt.Sprint(n.ID) {
		t.Fatalf("设置值未落库: %+v", found)
	}
	if len(found.Options) != 3 || found.Options[1].Label != "测试节点" {
		t.Fatalf("选项应包含自建节点: %+v", found.Options)
	}
	disabledLabel := ""
	for _, o := range found.Options {
		if o.Value == fmt.Sprint(unnamed.ID) {
			disabledLabel = o.Label
		}
	}
	if !strings.HasPrefix(disabledLabel, "节点 ") || !strings.HasSuffix(disabledLabel, "（已停用）") {
		t.Fatalf("未命名停用节点标签 = %q", disabledLabel)
	}

	got, err := m.OutboundNode(ctx)
	if err != nil || got.ID != n.ID || got.Link == "" {
		t.Fatalf("OutboundNode = %+v, err=%v", got, err)
	}

	if err := m.UpdateSelfNode(ctx, n.ID, n.Link, n.Name, false, 0); err != nil {
		t.Fatalf("停用节点: %v", err)
	}
	got, err = m.OutboundNode(ctx)
	if err != nil || got.ID != 0 {
		t.Fatalf("停用后应直连, got %+v err=%v", got, err)
	}

	if err := m.SetSettings(ctx, map[string]string{OutboundNodeKey: "9999"}); err == nil {
		t.Fatal("不存在的节点 ID 应被拒绝")
	}
}

func TestSetSettingsBoolNormalization(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	if err := m.SetSettings(ctx, map[string]string{"script.enabled": "ON"}); err != nil {
		t.Fatalf("SetSettings ON: %v", err)
	}
	v, err := m.GetConfig(ctx, "script.enabled")
	if err != nil || v != "true" {
		t.Fatalf("script.enabled 应 true, got %q err=%v", v, err)
	}
	if err := m.SetSettings(ctx, map[string]string{"subscheck.enabled": "no"}); err != nil {
		t.Fatalf("SetSettings no: %v", err)
	}
	v, err = m.GetConfig(ctx, "subscheck.enabled")
	if err != nil || v != "false" {
		t.Fatalf("subscheck.enabled 应 false, got %q err=%v", v, err)
	}
}

func TestSetSettingsStringTrim(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	if err := m.SetSettings(ctx, map[string]string{"rules.manifest_path": "  /tmp/x.yaml  "}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	v, err := m.GetConfig(ctx, "rules.manifest_path")
	if err != nil || v != "/tmp/x.yaml" {
		t.Fatalf("rules.manifest_path 应 /tmp/x.yaml, got %q err=%v", v, err)
	}
}

func TestSetSettingsUnknownKey(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	err := m.SetSettings(ctx, map[string]string{"nope": "1"})
	if err == nil {
		t.Fatal("未知 key 应报错")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Fatalf("错误应含 nope, got %q", err.Error())
	}
	items, lerr := m.ListSettings(ctx)
	if lerr != nil {
		t.Fatalf("ListSettings: %v", lerr)
	}
	if len(items) != 6 {
		t.Fatalf("ListSettings 应 6 项, got %d", len(items))
	}
}

func TestSetSettingsBadBool(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	if err := m.SetSettings(ctx, map[string]string{"script.enabled": "maybe"}); err == nil {
		t.Fatal("非法 bool 应报错")
	}
}

func TestSetSettingsAtomicOnError(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	err := m.SetSettings(ctx, map[string]string{"script.enabled": "true", "nope": "1"})
	if err == nil {
		t.Fatal("含未知 key 应报错")
	}
	v, gerr := m.GetConfig(ctx, "script.enabled")
	if gerr != nil {
		t.Fatalf("GetConfig: %v", gerr)
	}
	if v != "" {
		t.Fatalf("不应部分写入, script.enabled = %q", v)
	}
}

func TestListSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	if err := m.SetSettings(ctx, map[string]string{
		"script.enabled":      "true",
		"subscheck.enabled":   "false",
		"rules.manifest_path": "/tmp/manifest.yaml",
		"rules.output_dir":    "/tmp/out",
	}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	items, err := m.ListSettings(ctx)
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	if len(items) != 6 {
		t.Fatalf("len = %d, want 6", len(items))
	}
	want := map[string]string{
		"script.enabled":      "true",
		"subscheck.enabled":   "false",
		OutboundNodeKey:       "",
		SourceRefreshKey:      "",
		"rules.manifest_path": "/tmp/manifest.yaml",
		"rules.output_dir":    "/tmp/out",
	}
	for _, it := range items {
		if it.Value != want[it.Key] {
			t.Fatalf("%s = %q, want %q", it.Key, it.Value, want[it.Key])
		}
	}
}

// TestListSettingsIsDefault 覆盖 IsDefault 字段：未配置为 true，已配置为 false。
func TestListSettingsIsDefault(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	items, err := m.ListSettings(ctx)
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	for _, it := range items {
		if !it.IsDefault {
			t.Errorf("%s 初始应 is_default=true", it.Key)
		}
	}
	if err := m.SetSettings(ctx, map[string]string{"script.enabled": "true"}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	items, err = m.ListSettings(ctx)
	if err != nil {
		t.Fatalf("ListSettings 二次: %v", err)
	}
	for _, it := range items {
		if it.Key == "script.enabled" && it.IsDefault {
			t.Errorf("script.enabled 配置后应 is_default=false")
		}
		if it.Key != "script.enabled" && !it.IsDefault {
			t.Errorf("%s 未配置应 is_default=true", it.Key)
		}
	}
}

// TestListSettingsPlaceholder 覆盖 path 类型填 Placeholder、bool 留空。
func TestListSettingsPlaceholder(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	items, err := m.ListSettings(ctx)
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	for _, it := range items {
		if it.Kind == SettingKindPath && it.Placeholder == "" {
			t.Errorf("%s(path) 应有非空 Placeholder", it.Key)
		}
		if it.Kind == SettingKindBool && it.Placeholder != "" {
			t.Errorf("%s(bool) 的 Placeholder 应为空", it.Key)
		}
	}
}

// TestSetSettingsEmptyStringDeletes 覆盖空串删键：删除后 is_default 变 true。
func TestSetSettingsEmptyStringDeletes(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	if err := m.SetSettings(ctx, map[string]string{"rules.manifest_path": "/tmp/x.yaml"}); err != nil {
		t.Fatalf("SetSettings 配置: %v", err)
	}
	v, err := m.GetConfig(ctx, "rules.manifest_path")
	if err != nil || v != "/tmp/x.yaml" {
		t.Fatalf("配置后应存在, got %q err=%v", v, err)
	}
	if err := m.SetSettings(ctx, map[string]string{"rules.manifest_path": ""}); err != nil {
		t.Fatalf("SetSettings 空串: %v", err)
	}
	v, err = m.GetConfig(ctx, "rules.manifest_path")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if v != "" {
		t.Fatalf("空串应删键, got %q", v)
	}
	items, err := m.ListSettings(ctx)
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}
	for _, it := range items {
		if it.Key == "rules.manifest_path" && !it.IsDefault {
			t.Errorf("删键后 is_default 应为 true")
		}
	}
}

func TestListSettingsAfterClose(t *testing.T) {
	ctx := context.Background()
	m := newSettingsManager(t)
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := m.ListSettings(ctx); err == nil {
		t.Fatal("Close 后 ListSettings 应报错")
	}
}
