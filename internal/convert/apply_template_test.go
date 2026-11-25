package convert

import (
	"testing"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/subcfg"
)

// ApplyTemplate 只从 Raw 重建参数；调用方已经解析好的结构化字段必须原样带回，
// 否则模板路径会静默丢掉方案的 target/pipeline/include/exclude 与统一最终选择组。
func TestApplyTemplateKeepsRequestStructuredFields(t *testing.T) {
	base := Params{
		Target:            "surge",
		TargetSpecified:   true,
		Pipeline:          "clean",
		Config:            "tpl.ini",
		Include:           []string{"^HK"},
		Exclude:           []string{"过期"},
		Types:             []string{"ss"},
		Sort:              "name",
		Sources:           []Source{{Name: "airport", URLs: []string{"http://example.invalid/sub"}}},
		FinalGroup:        subcfg.FinalGroupConfig{Enabled: true, Mode: subcfg.FinalGroupModeAuto, Name: subcfg.DefaultFinalGroupName, IncludeDirect: true},
		RuleSourceMeta:    []subcfg.RuleSourceMeta{{Name: "src"}},
		ProfileName:       "pname",
		PruneUnusedGroups: true,
	}
	p, err := ApplyTemplate(config.Semantics{}, base)
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if p.Target != "surge" || p.Pipeline != "clean" || p.Config != "tpl.ini" {
		t.Errorf("target/pipeline/config 丢失: %q %q %q", p.Target, p.Pipeline, p.Config)
	}
	if len(p.Include) != 1 || p.Include[0] != "^HK" || len(p.Exclude) != 1 || p.Exclude[0] != "过期" {
		t.Errorf("include/exclude 丢失: %v %v", p.Include, p.Exclude)
	}
	if len(p.Types) != 1 || p.Types[0] != "ss" || p.Sort != "name" {
		t.Errorf("type/sort 丢失: %v %q", p.Types, p.Sort)
	}
	if len(p.Sources) != 1 || len(p.Sources[0].URLs) != 1 {
		t.Errorf("sources 丢失: %+v", p.Sources)
	}
	if !p.FinalGroup.Enabled || p.FinalGroup.ResolvedName() != subcfg.DefaultFinalGroupName {
		t.Errorf("final_group 丢失: %+v", p.FinalGroup)
	}
	if len(p.RuleSourceMeta) != 1 || p.ProfileName != "pname" || !p.PruneUnusedGroups {
		t.Errorf("rule meta/profile/prune 丢失: %+v %q %v", p.RuleSourceMeta, p.ProfileName, p.PruneUnusedGroups)
	}
}

// 服务端套用的默认 target 不能覆盖模板 target；显式指定的 target 才优先于模板。
func TestApplyTemplateTargetPrecedence(t *testing.T) {
	sem := config.Semantics{Params: map[string][]string{"target": {"surge"}}}

	// query/CLI 里写了 target：ParseMulti 必须标记为显式，模板 target 不能压掉它。
	parsed, err := ParseMulti(map[string][]string{"target": {"v2ray"}})
	if err != nil {
		t.Fatalf("ParseMulti: %v", err)
	}
	if !parsed.TargetSpecified {
		t.Fatalf("target=v2ray 应标记为显式")
	}
	p, err := ApplyTemplate(sem, parsed)
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if p.Target != "v2ray" {
		t.Errorf("显式 target 未覆盖模板 target: %q", p.Target)
	}

	p, err = ApplyTemplate(sem, Params{Target: "clash.meta", TargetSpecified: false})
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if p.Target != "surge" {
		t.Errorf("默认 target 覆盖了模板 target: %q", p.Target)
	}

	p, err = ApplyTemplate(sem, Params{Target: "v2ray", TargetSpecified: true})
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if p.Target != "v2ray" {
		t.Errorf("显式 target 未覆盖模板 target: %q", p.Target)
	}
}
