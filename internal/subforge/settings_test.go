package subforge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
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
	if len(items) != 4 {
		t.Fatalf("len = %d, want 4", len(items))
	}
	wantKeys := []string{"script.enabled", "subscheck.enabled", "rules.manifest_path", "rules.output_dir"}
	for i, k := range wantKeys {
		if items[i].Key != k {
			t.Fatalf("items[%d].Key = %q, want %q", i, items[i].Key, k)
		}
	}
	if items[0].Value != "false" || items[1].Value != "false" {
		t.Fatalf("bool 默认值应 false, got %q %q", items[0].Value, items[1].Value)
	}
	if items[2].Value != "" || items[3].Value != "" {
		t.Fatalf("path 默认值应空串, got %q %q", items[2].Value, items[3].Value)
	}
	if items[0].Kind != SettingKindBool || items[1].Kind != SettingKindBool {
		t.Errorf("bool kind: %q %q", items[0].Kind, items[1].Kind)
	}
	if items[2].Kind != SettingKindPath || items[3].Kind != SettingKindPath {
		t.Errorf("path kind: %q %q", items[2].Kind, items[3].Kind)
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
	if len(items) != 4 {
		t.Fatalf("ListSettings 应 4 项, got %d", len(items))
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
	if len(items) != 4 {
		t.Fatalf("len = %d, want 4", len(items))
	}
	want := map[string]string{
		"script.enabled":      "true",
		"subscheck.enabled":   "false",
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
