package main

import "testing"

// TestBuildCLIParams_FlagToRaw 命令行 flag 要归一成 query 参数形状，
// 否则 Params.Raw 是空的，模板覆盖合并会把命令行传入的设置静默丢掉。
func TestBuildCLIParams_FlagToRaw(t *testing.T) {
	p, err := buildCLIParams(false, "clash.meta",
		multiFlag{"http://a/sub", "http://b/sub"},
		multiFlag{"HK,JP"},
		multiFlag{"- 02$", "^TEST"},
		multiFlag{"vless;vmess"},
		multiFlag{"-node,节点"},
		multiFlag{"港,🚀"},
		multiFlag{"sub.txt"},
		multiFlag{"K: V", "Bad"},
		"name",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.URLs) != 2 {
		t.Errorf("URLs = %v", p.URLs)
	}
	if got := p.Include; len(got) != 2 || got[0] != "HK" || got[1] != "JP" {
		t.Errorf("Include = %v，一次 flag 里的逗号应拆开", got)
	}
	if got := p.Exclude; len(got) != 2 {
		t.Errorf("Exclude = %v", got)
	}
	if got := p.Types; len(got) != 2 || got[0] != "vless" || got[1] != "vmess" {
		t.Errorf("Types = %v，分号也应视为分隔符", got)
	}
	if got := p.Rename; len(got) != 1 || got[0].Pattern != "-node" || got[0].Replacement != "节点" {
		t.Errorf("Rename = %v", got)
	}
	if got := p.Emoji; len(got) != 1 || got[0].Pattern != "港" || got[0].Replacement != "🚀" {
		t.Errorf("Emoji = %v", got)
	}
	if p.Sort != "name" {
		t.Errorf("Sort = %q", p.Sort)
	}
	if len(p.Files) != 1 || p.Files[0] != "sub.txt" {
		t.Errorf("Files = %v", p.Files)
	}
	if got := p.Headers["K"]; got != "V" {
		t.Errorf("Headers = %v，非法的 -header 应被跳过", p.Headers)
	}
	if p.Target != "clash.meta" {
		t.Errorf("Target = %q", p.Target)
	}
	// 关键断言：Raw 里必须有归一后的键，模板合并才不会丢掉命令行设置。
	if _, ok := p.Raw["include"]; !ok {
		t.Error("Raw 缺少 include，模板覆盖合并会静默丢弃命令行参数")
	}
	if _, ok := p.Raw["rename"]; !ok {
		t.Error("Raw 缺少 rename")
	}
}

// TestBuildCLIParams_DefaultTarget 不给 -format 时留空，让模板的 target 有机会生效。
func TestBuildCLIParams_DefaultTarget(t *testing.T) {
	p, err := buildCLIParams(false, "", multiFlag{"u"}, nil, nil, nil, nil, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Target != "clash.meta" {
		t.Errorf("无格式、无模板时 Target = %q，应为 clash.meta", p.Target)
	}
	if _, ok := p.Raw["target"]; ok {
		t.Error("未显式指定格式时 Raw 里不应有 target，否则会盖掉模板的 [common] target")
	}
}

// TestBuildCLIParams_BadRegex 正则写错要在返回时报出来，不能等到拉订阅之后。
func TestBuildCLIParams_BadRegex(t *testing.T) {
	if _, err := buildCLIParams(false, "", multiFlag{"u"}, multiFlag{"("}, nil, nil, nil, nil, nil, nil, ""); err == nil {
		t.Error("非法 include 正则应报错")
	}
	if _, err := buildCLIParams(false, "", multiFlag{"u"}, nil, nil, nil, multiFlag{",x"}, nil, nil, nil, ""); err == nil {
		t.Error("缺少正则的 rename 应报错")
	}
}

// TestBuildCLIParams_ListFlag -list 要落成 query 参数，否则 Engine 不知道要切到清单形态。
func TestBuildCLIParams_ListFlag(t *testing.T) {
	p, err := buildCLIParams(true, "", multiFlag{"u"}, nil, nil, nil, nil, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !p.List {
		t.Error("-list 没有生效：Params.List = false")
	}
	if p.Target != "clash.meta" {
		t.Errorf("Target = %q，-list 不应改写 -format", p.Target)
	}
	if _, ok := p.Raw["list"]; !ok {
		t.Error("Raw 缺少 list，模板覆盖合并会丢掉命令行的 list 设置")
	}
}
