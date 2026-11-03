package subcfg

import (
	"strings"
	"testing"
)

func TestParseFinalGroupEmpty(t *testing.T) {
	cfg, err := ParseFinalGroup("")
	if err != nil {
		t.Fatalf("ParseFinalGroup 空串: %v", err)
	}
	if cfg.Enabled || cfg.Mode != FinalGroupModeAuto || cfg.Name != DefaultFinalGroupName || !cfg.IncludeDirect {
		t.Fatalf("默认配置 = %+v", cfg)
	}
}

func TestFinalGroupNormalizeAndMarshal(t *testing.T) {
	cfg := FinalGroupConfig{
		Enabled:   true,
		Mode:      "CUSTOM",
		Name:      "  🌐 统一出口  ",
		Overrides: map[string][]string{"group:airport:A": {" 选择节点 ", "选择节点", ""}},
	}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if cfg.Mode != FinalGroupModeCustom || cfg.Name != "🌐 统一出口" {
		t.Errorf("归一化结果 = %+v", cfg)
	}
	if got := cfg.Overrides["group:airport:A"]; len(got) != 1 || got[0] != "选择节点" {
		t.Errorf("override 归一化 = %v", got)
	}
	raw, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := ParseFinalGroup(raw)
	if err != nil {
		t.Fatalf("ParseFinalGroup: %v", err)
	}
	if got.Name != cfg.Name || !got.Enabled || got.Mode != cfg.Mode {
		t.Errorf("往返结果 = %+v", got)
	}
}

func TestFinalGroupValidation(t *testing.T) {
	cases := []FinalGroupConfig{
		{Enabled: true, Mode: FinalGroupModeCustom, Name: ""},
		{Enabled: true, Mode: "other", Name: "x"},
		{Enabled: true, Mode: FinalGroupModeAuto, Name: "a,b"},
		{Enabled: true, Mode: FinalGroupModeAuto, Name: "a\nb"},
		{Enabled: true, Mode: FinalGroupModeAuto, Name: "DIRECT"},
	}
	for _, cfg := range cases {
		if err := cfg.Normalize(); err == nil {
			t.Errorf("配置 %+v 应校验失败", cfg)
		}
	}
}

func TestFinalGroupNormalizeEdgeBranches(t *testing.T) {
	// 空 mode 归一为 auto；空名字在 auto 下补默认。
	cfg := FinalGroupConfig{Enabled: true}
	if err := cfg.Normalize(); err != nil || cfg.Mode != FinalGroupModeAuto || cfg.Name != DefaultFinalGroupName {
		t.Errorf("空配置归一化 = %+v, err=%v", cfg, err)
	}
	// 空 override 键 / 空名字被丢弃。
	cfg = FinalGroupConfig{Mode: FinalGroupModeAuto, Name: "x", Overrides: map[string][]string{" ": {"a"}, "k": {"", " "}}}
	if err := cfg.Normalize(); err != nil || len(cfg.Overrides) != 0 {
		t.Errorf("空 override 未清理: %+v, err=%v", cfg.Overrides, err)
	}
	// 超长与包含控制字符的名字都要拒绝。
	long := strings.Repeat("a", 101)
	for _, name := range []string{long, "a\x01b"} {
		cfg := FinalGroupConfig{Mode: FinalGroupModeCustom, Name: name, Enabled: true}
		if err := cfg.Normalize(); err == nil {
			t.Errorf("非法名称 %q 应被拒绝", name)
		}
	}
}

func TestFinalGroupParseAndMarshalErrors(t *testing.T) {
	if _, err := ParseFinalGroup("{"); err == nil {
		t.Errorf("非法 JSON 应报错")
	}
	bad := FinalGroupConfig{Mode: FinalGroupModeCustom, Name: "", Enabled: true}
	if _, err := bad.Marshal(); err == nil {
		t.Errorf("非法配置 Marshal 应报错")
	}
}

func TestResolvedNameAndOverrideFor(t *testing.T) {
	cfg := FinalGroupConfig{Mode: FinalGroupModeAuto, Name: "旧名字"}
	if got := cfg.ResolvedName(); got != DefaultFinalGroupName {
		t.Errorf("auto 模式应忽略 name，得到 %q", got)
	}
	cfg.Mode = FinalGroupModeCustom
	cfg.Name = "🌐 统一出口"
	if got := cfg.ResolvedName(); got != "🌐 统一出口" {
		t.Errorf("custom 模式 ResolvedName = %q", got)
	}
	cfg.Overrides = map[string][]string{"group:airport:A": {"选择节点"}}
	if got, ok := cfg.OverrideFor("group:airport:A"); !ok || len(got) != 1 || got[0] != "选择节点" {
		t.Errorf("OverrideFor = %v/%v", got, ok)
	}
	if _, ok := cfg.OverrideFor("group:airport:B"); ok {
		t.Errorf("未配置的来源不应返回 ok")
	}
	var empty FinalGroupConfig
	if _, ok := empty.OverrideFor("x"); ok {
		t.Errorf("nil Overrides 不应返回 ok")
	}
}

func TestIsNoneOverride(t *testing.T) {
	for _, names := range [][]string{{"none"}, {"-"}, {"off"}, {"不处理"}} {
		if !IsNoneOverride(names) {
			t.Errorf("%v 应为不处理", names)
		}
	}
	if IsNoneOverride([]string{"选择节点"}) {
		t.Errorf("普通组名不应被当作不处理")
	}
}

func TestRuleSourceMetaJSONShape(t *testing.T) {
	meta := RuleSourceMeta{Name: "src", RuleCount: 2, FinalTargets: []string{"🐟 漏网之鱼"}, TargetCounts: map[string]int{"🚀 手动选择": 1}}
	if meta.Name == "" || !strings.Contains(meta.FinalTargets[0], "漏网") {
		t.Fatalf("meta = %+v", meta)
	}
}
