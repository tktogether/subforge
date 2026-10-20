package render

import "testing"

// TestResolveRuleTargets 覆盖规则目标校正的判定顺序：
// 有效目标原样保留 → 同义名映射 → 关键词表映射 → 兜底分组。
func TestResolveRuleTargets(t *testing.T) {
	groups := []string{"🚀 手动选择", "🎥 Netflix", "🍎 苹果服务", "🎯 全球直连", "🐟 漏网之鱼"}

	cases := []struct {
		name  string
		in    []string
		want  []string
		warns int
	}{
		{
			name: "有效目标与内置出站不动",
			in: []string{
				"DOMAIN,keep.example,🚀 手动选择",
				"GEOIP,CN,DIRECT,no-resolve",
				"MATCH,🐟 漏网之鱼",
			},
			want: []string{
				"DOMAIN,keep.example,🚀 手动选择",
				"GEOIP,CN,DIRECT,no-resolve",
				"MATCH,🐟 漏网之鱼",
			},
			warns: 0,
		},
		{
			name:  "同义名：表情与前缀不同也能对上",
			in:    []string{"DOMAIN-SUFFIX,netflix.com,📺 Netflix"},
			want:  []string{"DOMAIN-SUFFIX,netflix.com,🎥 Netflix"},
			warns: 1,
		},
		{
			name:  "关键词表：英文名映射到中文分组",
			in:    []string{"DOMAIN-SUFFIX,icloud.com,Apple"},
			want:  []string{"DOMAIN-SUFFIX,icloud.com,🍎 苹果服务"},
			warns: 1,
		},
		{
			name:  "关键词表：国内直连落到直连分组",
			in:    []string{"DOMAIN-SUFFIX,qq.com,🌏 国内直连"},
			want:  []string{"DOMAIN-SUFFIX,qq.com,🎯 全球直连"},
			warns: 1,
		},
		{
			name:  "关键词表：广告类直接 REJECT",
			in:    []string{"DOMAIN-SUFFIX,ads.example,🛑 广告拦截"},
			want:  []string{"DOMAIN-SUFFIX,ads.example,REJECT"},
			warns: 1,
		},
		{
			name:  "找不到同义分组时落到兜底分组（MATCH 的目标）",
			in:    []string{"DOMAIN-SUFFIX,unknown.example,✈️ 某个没建过的组", "MATCH,🐟 漏网之鱼"},
			want:  []string{"DOMAIN-SUFFIX,unknown.example,🐟 漏网之鱼", "MATCH,🐟 漏网之鱼"},
			warns: 1,
		},
		{
			name: "FINAL 改写成 MATCH（mihomo 没有 FINAL 类型）",
			in: []string{
				"DOMAIN,final.example,🐟 漏网之鱼",
				"FINAL,🐟 漏网之鱼",
			},
			want: []string{
				"DOMAIN,final.example,🐟 漏网之鱼",
				"MATCH,🐟 漏网之鱼",
			},
			warns: 0,
		},
		{
			name:  "逻辑规则的目标在最后一个字段",
			in:    []string{"AND,((DOMAIN,a.example),(NETWORK,UDP)),🎥 Netflix"},
			want:  []string{"AND,((DOMAIN,a.example),(NETWORK,UDP)),🎥 Netflix"},
			warns: 0,
		},
		{
			name:  "逻辑规则的目标也要校正",
			in:    []string{"AND,((DOMAIN,a.example),(NETWORK,UDP)),📺 Netflix"},
			want:  []string{"AND,((DOMAIN,a.example),(NETWORK,UDP)),🎥 Netflix"},
			warns: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, warns := ResolveRuleTargets(tc.in, groups, nil)
			if len(got) != len(tc.want) {
				t.Fatalf("规则条数 = %d, want %d（%v）", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("第 %d 条 = %q, want %q", i+1, got[i], tc.want[i])
				}
			}
			if len(warns) != tc.warns {
				t.Errorf("警告条数 = %d, want %d（%v）", len(warns), tc.warns, warns)
			}
		})
	}
}

// TestResolveRuleTargetsEmpty 空规则直接原样返回，不产生警告。
func TestResolveRuleTargetsEmpty(t *testing.T) {
	got, warns := ResolveRuleTargets(nil, []string{"A"}, nil)
	if len(got) != 0 || len(warns) != 0 {
		t.Fatalf("空输入应原样返回，得到 %v / %v", got, warns)
	}
}
