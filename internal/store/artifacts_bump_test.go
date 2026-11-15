package store

// 产物相关表写路径 bump 接线测试（订阅产物预生成 任务 2）。
//
// 表驱动：每个被接线的变更方法一条用例——先铺底（setup，允许顺带 bump），记
// ArtifactVersion 前值，执行变更方法，断言版本恰好 +1；外加负例：PutSourceCache
// 不在 bump 清单，执行后版本不变。
// 全部走真实 DB（newTestDB 临时文件库），无 mock。

import (
	"context"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/subcfg"
)

// strPtr / idSlicePtr 是部分更新接口（nil 区分「不动」与「清空」）的构造辅助。
func strPtr(s string) *string       { return &s }
func idSlicePtr(v []int64) *[]int64 { return &v }

// bumpCase 是一条「变更方法 → 版本恰好 +1」用例。
// setup 铺底数据（可选）：铺底自身可能触发 bump，但前值在 setup 之后才读，不影响断言。
type bumpCase struct {
	name   string
	setup  func(t *testing.T, d *DB, ctx context.Context) error
	mutate func(t *testing.T, d *DB, ctx context.Context) error
}

// mustAddGroup 铺一条自定义分组并返回 id（供 Update/Delete 类用例复用）。
func mustAddGroup(t *testing.T, d *DB, ctx context.Context, name string) int64 {
	t.Helper()
	g, err := d.AddProxyGroupWithMembersAndFlags(ctx, name, "select", "", "", 0, 0, "", nil, false, false)
	if err != nil {
		t.Fatalf("铺底分组 %s: %v", name, err)
	}
	return g.ID
}

// mustAddCustomRule 铺一条自定义规则（source=”）并返回 id。
func mustAddCustomRule(t *testing.T, d *DB, ctx context.Context, ruleType, param string) int64 {
	t.Helper()
	r, err := d.AddRuleWithSource(ctx, Rule{RuleType: ruleType, RuleParam: param, RuleConfig: 1})
	if err != nil {
		t.Fatalf("铺底规则 %s: %v", param, err)
	}
	return r.ID
}

// mustAddProfile 铺一个订阅方案并返回 id。
func mustAddProfile(t *testing.T, d *DB, ctx context.Context, name string) int64 {
	t.Helper()
	p, err := d.AddProfile(ctx, name, "", "", "", true, 0)
	if err != nil {
		t.Fatalf("铺底方案 %s: %v", name, err)
	}
	return p.ID
}

// mustAddScript / mustAddPipeline 铺脚本与管线并返回 id。
func mustAddScript(t *testing.T, d *DB, ctx context.Context, name string) Script {
	t.Helper()
	s, err := d.CreateScript(ctx, Script{Name: name, Kind: "filter", Code: "return nodes"})
	if err != nil {
		t.Fatalf("铺底脚本 %s: %v", name, err)
	}
	return *s
}

func mustAddPipeline(t *testing.T, d *DB, ctx context.Context, name string) Pipeline {
	t.Helper()
	p, err := d.CreatePipeline(ctx, Pipeline{Name: name, StepsJSON: "[]"})
	if err != nil {
		t.Fatalf("铺底管线 %s: %v", name, err)
	}
	return *p
}

// TestBumpArtifactVersion_OnMutation 断言每个被接线的产物相关表变更方法恰好 +1。
func TestBumpArtifactVersion_OnMutation(t *testing.T) {
	cases := []bumpCase{
		// --- self_nodes ---
		{name: "AddSelfNode", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddSelfNode(ctx, "ss://bmFtZTE=", "自建1", true, 0)
			return err
		}},
		{name: "UpdateSelfNode", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddSelfNode(ctx, "ss://bmFtZTE=", "自建1", true, 0)
			return err
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			nodes, err := d.ListSelfNodes(ctx)
			if err != nil {
				return err
			}
			return d.UpdateSelfNode(ctx, nodes[0].ID, "ss://bmFtZTEy", "自建1改", false, 0)
		}},
		{name: "DeleteSelfNode", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddSelfNode(ctx, "ss://bmFtZTE=", "自建1", true, 0)
			return err
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			nodes, err := d.ListSelfNodes(ctx)
			if err != nil {
				return err
			}
			return d.DeleteSelfNode(ctx, nodes[0].ID)
		}},

		// --- sub_tokens ---
		{name: "UpdateSubToken", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.CreateSubTokenWithProfile(ctx, "tok-us", "令牌", "", "", "", "", "", 0)
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.UpdateSubToken(ctx, "tok-us", SubTokenUpdate{Name: strPtr("令牌改")})
		}},
		{name: "DeleteSubToken", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.CreateSubTokenWithProfile(ctx, "tok-del", "令牌", "", "", "", "", "", 0)
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.DeleteSubToken(ctx, "tok-del")
		}},
		{name: "CreateSubTokenWithProfile", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.CreateSubTokenWithProfile(ctx, "tok-new", "令牌", "clash", "", "", "", "", 0)
		}},

		// --- proxy_groups ---
		{name: "AddProxyGroupWithMembersAndFlags", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddProxyGroupWithMembersAndFlags(ctx, "分组", "select", "", "", 0, 0, "", nil, true, false)
			return err
		}},
		{name: "UpdateProxyGroup", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddGroup(t, d, ctx, "分组")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			groups, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			return d.UpdateProxyGroup(ctx, groups[0].ID, "分组改", "url-test", "", "http://g", 300, 0)
		}},
		{name: "SetProxyGroupIncludeSelf", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddGroup(t, d, ctx, "分组")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			groups, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			return d.SetProxyGroupIncludeSelf(ctx, groups[0].ID, true)
		}},
		{name: "SetProxyGroupIncludeAllSources", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddGroup(t, d, ctx, "分组")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			groups, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			return d.SetProxyGroupIncludeAllSources(ctx, groups[0].ID, true)
		}},
		{name: "UpdateImportedGroupMembersFlags", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddGroup(t, d, ctx, "机场组")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			groups, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			return d.UpdateImportedGroupMembersFlags(ctx, groups[0].ID, []string{"a", "b"}, true, true)
		}},

		// --- rules ---
		{name: "AddRuleWithSource", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN", RuleParam: "add.com", RuleConfig: 1})
			return err
		}},
		{name: "UpdateRule", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddCustomRule(t, d, ctx, "DOMAIN", "up.com")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			rules, err := d.ListRulesFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			return d.UpdateRule(ctx, rules[0].ID, "DOMAIN-SUFFIX", "up.com", 0, true, false)
		}},
		{name: "UpdateRuleFromSource", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddCustomRule(t, d, ctx, "DOMAIN", "src.com")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			rules, err := d.ListRulesFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			return d.UpdateRuleFromSource(ctx, rules[0].ID, "DOMAIN-SUFFIX", "src.net", 0, false, "DOMAIN-SUFFIX,src.net", "src")
		}},
		{name: "SetRuleSource", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddCustomRule(t, d, ctx, "DOMAIN", "mv.com")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			rules, err := d.ListRulesFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			return d.SetRuleSource(ctx, rules[0].ID, "airport:重指")
		}},
		{name: "BatchSetRulesEnabledBySource", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN", RuleParam: "b1.com", RuleConfig: 1, Source: "airport:批量", OriginKey: "b1"})
			if err != nil {
				return err
			}
			_, err = d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN", RuleParam: "b2.com", RuleConfig: 1, Source: "airport:批量", OriginKey: "b2"})
			return err
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.BatchSetRulesEnabledBySource(ctx, SourceFilter{Source: "airport:批量"}, true)
			return err
		}},
		{name: "BatchDeleteRulesBySource", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN", RuleParam: "d1.com", RuleConfig: 1, Source: "airport:删批", OriginKey: "d1"})
			return err
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.BatchDeleteRulesBySource(ctx, SourceFilter{Source: "airport:删批"})
			return err
		}},
		{name: "BatchInsertImportedRules", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			now := time.Now()
			_, _, err := d.BatchInsertImportedRules(ctx, []ImportedRuleRow{{
				RuleType: "DOMAIN-SUFFIX", RuleParam: "imp.com", RuleConfig: 1,
				Source: "airport:导入", OriginKey: "imp1", CreatedAt: now, UpdatedAt: now,
			}})
			return err
		}},
		{name: "BatchDeleteRules", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddCustomRule(t, d, ctx, "DOMAIN", "bd1.com")
			mustAddCustomRule(t, d, ctx, "DOMAIN", "bd2.com")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			rules, err := d.ListRulesFiltered(ctx, SourceFilter{All: true})
			if err != nil {
				return err
			}
			ids := make([]int64, 0, len(rules))
			for _, r := range rules {
				ids = append(ids, r.ID)
			}
			_, err = d.BatchDeleteRules(ctx, ids)
			return err
		}},

		// --- scripts ---
		{name: "CreateScript", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.CreateScript(ctx, Script{Name: "脚本建", Kind: "filter", Code: "return nodes"})
			return err
		}},
		{name: "UpdateScript", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddScript(t, d, ctx, "脚本改")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			scripts, err := d.ListScripts(ctx)
			if err != nil {
				return err
			}
			s := scripts[0]
			s.Code = "return nodes[:1]"
			return d.UpdateScript(ctx, s)
		}},
		{name: "DeleteScript", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddScript(t, d, ctx, "脚本删")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			scripts, err := d.ListScripts(ctx)
			if err != nil {
				return err
			}
			return d.DeleteScript(ctx, scripts[0].ID)
		}},

		// --- pipelines ---
		{name: "CreatePipeline", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.CreatePipeline(ctx, Pipeline{Name: "管线建", StepsJSON: "[]"})
			return err
		}},
		{name: "UpdatePipeline", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddPipeline(t, d, ctx, "管线改")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			ps, err := d.ListPipelines(ctx)
			if err != nil {
				return err
			}
			p := ps[0]
			p.StepsJSON = `[{"name":"s"}]`
			return d.UpdatePipeline(ctx, p)
		}},
		{name: "DeletePipeline", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddPipeline(t, d, ctx, "管线删")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			ps, err := d.ListPipelines(ctx)
			if err != nil {
				return err
			}
			return d.DeletePipeline(ctx, ps[0].ID)
		}},

		// --- rule_providers ---
		{name: "UpsertRuleProvider 插入分支", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.UpsertRuleProvider(ctx, RuleProvider{Name: "规则集", RawJSON: "{}", Source: "airport:rp"})
		}},
		{name: "UpsertRuleProvider 更新分支", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.UpsertRuleProvider(ctx, RuleProvider{Name: "规则集二", RawJSON: "{}", Source: "airport:rp2"})
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.UpsertRuleProvider(ctx, RuleProvider{Name: "规则集二", RawJSON: `{"k":1}`, Source: "airport:rp2"})
		}},
		{name: "SetRuleProviderEnabled", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.UpsertRuleProvider(ctx, RuleProvider{Name: "规则集三", RawJSON: "{}", Source: "airport:rp3"})
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			providers, err := d.ListRuleProviders(ctx)
			if err != nil {
				return err
			}
			ok, err := d.SetRuleProviderEnabled(ctx, providers[0].ID, false)
			if err != nil {
				return err
			}
			if !ok {
				t.Fatal("SetRuleProviderEnabled 返回 false，期望命中一行")
			}
			return nil
		}},

		// --- source_pipelines ---
		{name: "SetSourcePipeline 写入分支", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.SetSourcePipeline(ctx, "airport:管线", "清洗")
		}},
		{name: "SetSourcePipeline 清空分支", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.SetSourcePipeline(ctx, "airport:管线2", "清洗")
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			return d.SetSourcePipeline(ctx, "airport:管线2", "")
		}},

		// --- profiles ---
		{name: "AddProfile", mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			_, err := d.AddProfile(ctx, "方案建", "", "", "", true, 0)
			return err
		}},
		{name: "UpdateProfile", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddProfile(t, d, ctx, "方案改")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			profiles, err := d.ListProfiles(ctx)
			if err != nil {
				return err
			}
			return d.UpdateProfile(ctx, profiles[0].ID, "方案改名", "备注", "tpl.ini", "管线", false, 0)
		}},
		{name: "DeleteProfile", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddProfile(t, d, ctx, "方案删")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			profiles, err := d.ListProfiles(ctx)
			if err != nil {
				return err
			}
			return d.DeleteProfile(ctx, profiles[0].ID)
		}},
		{name: "SetProfileRenderOptions", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddProfile(t, d, ctx, "方案渲染")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			profiles, err := d.ListProfiles(ctx)
			if err != nil {
				return err
			}
			return d.SetProfileRenderOptions(ctx, profiles[0].ID, strPtr("clash"), nil, nil)
		}},
		{name: "SetProfileSources", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddProfile(t, d, ctx, "方案来源")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			profiles, err := d.ListProfiles(ctx)
			if err != nil {
				return err
			}
			return d.SetProfileSources(ctx, profiles[0].ID, idSlicePtr([]int64{1}), nil)
		}},
		{name: "SetProfileFinalGroup", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddProfile(t, d, ctx, "方案终组")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			profiles, err := d.ListProfiles(ctx)
			if err != nil {
				return err
			}
			return d.SetProfileFinalGroup(ctx, profiles[0].ID, subcfg.FinalGroupConfig{Enabled: true, Mode: "custom", Name: "最终"})
		}},
		{name: "SetProfileRuleSources", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddProfile(t, d, ctx, "方案规则源")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			profiles, err := d.ListProfiles(ctx)
			if err != nil {
				return err
			}
			return d.SetProfileRuleSources(ctx, profiles[0].ID, []string{"规则源A"})
		}},
		{name: "SetProfileLayers", setup: func(t *testing.T, d *DB, ctx context.Context) error {
			mustAddProfile(t, d, ctx, "方案层")
			return nil
		}, mutate: func(t *testing.T, d *DB, ctx context.Context) error {
			profiles, err := d.ListProfiles(ctx)
			if err != nil {
				return err
			}
			return d.SetProfileLayers(ctx, profiles[0].ID, []string{"airport", "custom"}, []string{"airport"})
		}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d, ctx := newTestDB(t)
			if tc.setup != nil {
				if err := tc.setup(t, d, ctx); err != nil {
					t.Fatalf("铺底失败: %v", err)
				}
			}
			before, err := d.ArtifactVersion(ctx)
			if err != nil {
				t.Fatalf("读前值失败: %v", err)
			}
			if err := tc.mutate(t, d, ctx); err != nil {
				t.Fatalf("变更 %s 失败: %v", tc.name, err)
			}
			after, err := d.ArtifactVersion(ctx)
			if err != nil {
				t.Fatalf("读后值失败: %v", err)
			}
			if after != before+1 {
				t.Fatalf("%s 后 ArtifactVersion = %d，期望恰好 %d（前值 %d）", tc.name, after, before+1, before)
			}
		})
	}
}

// TestBumpArtifactVersion_NoBump 覆盖例外清单（store 层）：PutSourceCache 写
// source_cache 表，不在 bump 清单内，执行后版本不变。
func TestBumpArtifactVersion_NoBump(t *testing.T) {
	d, ctx := newTestDB(t)

	before, err := d.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读前值失败: %v", err)
	}
	if err := d.PutSourceCache(ctx, "http://up.example/sub", []byte("机场原文")); err != nil {
		t.Fatalf("PutSourceCache: %v", err)
	}
	after, err := d.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读后值失败: %v", err)
	}
	if after != before {
		t.Fatalf("PutSourceCache 后 ArtifactVersion = %d，期望不变（前值 %d）", after, before)
	}
}
