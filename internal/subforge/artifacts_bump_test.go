package subforge

// Manager 层产物相关表写路径 bump 接线测试（订阅产物预生成 任务 2）。
//
// 机场等表的写方法落在 Manager（经 m.db.Raw() 直写），store 层够不着，
// bump 接线在 Manager 方法内完成，所以用例也放这里：
//   - 每个被接线的变更方法一条用例：铺底（允许 bump）→ 记版本前值 → 变更 → 断言恰好 +1；
//   - 负例：机场 traffic 列更新方法（upload/download/total/expire/node_count/
//     traffic_at/traffic_err）不在 bump 清单，执行后版本不变。

import (
	"context"
	"testing"

	"github.com/tktogether/subforge/internal/traffic"
)

// managerBumpCase 是一条「变更方法 → 版本恰好 +1」用例。
type managerBumpCase struct {
	name   string
	setup  func(t *testing.T, m *Manager, ctx context.Context) error
	mutate func(t *testing.T, m *Manager, ctx context.Context) error
}

// mustManagerAirport 铺一条订阅并返回 id。
func mustManagerAirport(t *testing.T, m *Manager, ctx context.Context, name string) int64 {
	t.Helper()
	a, err := m.AddAirport(ctx, name, "http://up.example/"+name, "", true)
	if err != nil {
		t.Fatalf("铺底订阅 %s: %v", name, err)
	}
	return a.ID
}

// TestBumpArtifactVersion_OnManagerMutation 断言 Manager 层每个被接线的变更方法恰好 +1。
func TestBumpArtifactVersion_OnManagerMutation(t *testing.T) {
	cases := []managerBumpCase{
		{name: "AddAirport", mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			_, err := m.AddAirport(ctx, "机场建", "http://up.example/new", "", true)
			return err
		}},
		{name: "UpdateAirport", setup: func(t *testing.T, m *Manager, ctx context.Context) error {
			mustManagerAirport(t, m, ctx, "机场改")
			return nil
		}, mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			a, err := m.ListAirports(ctx)
			if err != nil {
				return err
			}
			_, err = m.UpdateAirport(ctx, a[0].ID, "机场改名", "http://up.example/changed", "备注", false)
			return err
		}},
		{name: "SetAirportOutboundNode", setup: func(t *testing.T, m *Manager, ctx context.Context) error {
			mustManagerAirport(t, m, ctx, "机场出网")
			return nil
		}, mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			a, err := m.ListAirports(ctx)
			if err != nil {
				return err
			}
			return m.SetAirportOutboundNode(ctx, a[0].ID, "direct")
		}},
		{name: "SetAirportFailurePolicy", setup: func(t *testing.T, m *Manager, ctx context.Context) error {
			mustManagerAirport(t, m, ctx, "机场策略")
			return nil
		}, mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			a, err := m.ListAirports(ctx)
			if err != nil {
				return err
			}
			return m.SetAirportFailurePolicy(ctx, a[0].ID, AirportFailureError)
		}},
		// DeleteAirport 的版本 bump 由末尾 SetSourcePipeline（清空来源管线，store 层已接线）承担，
		// 本用例锁定整链恰好 +1、不双 bump。
		{name: "DeleteAirport", setup: func(t *testing.T, m *Manager, ctx context.Context) error {
			mustManagerAirport(t, m, ctx, "机场删")
			return nil
		}, mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			a, err := m.ListAirports(ctx)
			if err != nil {
				return err
			}
			return m.DeleteAirport(ctx, a[0].ID)
		}},
		{name: "DeleteProxyGroup", setup: func(t *testing.T, m *Manager, ctx context.Context) error {
			_, err := m.AddProxyGroup(ctx, "分组删", "select", "", "", 0, 0)
			return err
		}, mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			groups, err := m.ListProxyGroups(ctx)
			if err != nil {
				return err
			}
			return m.DeleteProxyGroup(ctx, groups[0].ID)
		}},
		{name: "DeleteRule", setup: func(t *testing.T, m *Manager, ctx context.Context) error {
			_, err := m.AddRule(ctx, "DOMAIN", "del.com", 1, false, true)
			return err
		}, mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			rules, err := m.ListRules(ctx)
			if err != nil {
				return err
			}
			return m.DeleteRule(ctx, rules[0].ID)
		}},
		{name: "CreateSubToken", mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			return m.CreateSubToken(ctx, "tok-mgr", "令牌", "", "", "", "", "")
		}},
		{name: "setConfig", mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			return m.setConfig(ctx, "fetch.refresh_minutes", "15")
		}},
		{name: "deleteConfig", setup: func(t *testing.T, m *Manager, ctx context.Context) error {
			return m.setConfig(ctx, "fetch.refresh_minutes", "15")
		}, mutate: func(t *testing.T, m *Manager, ctx context.Context) error {
			return m.deleteConfig(ctx, "fetch.refresh_minutes")
		}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			m := newTestManager(t)
			if tc.setup != nil {
				if err := tc.setup(t, m, ctx); err != nil {
					t.Fatalf("铺底失败: %v", err)
				}
			}
			before, err := m.db.ArtifactVersion(ctx)
			if err != nil {
				t.Fatalf("读前值失败: %v", err)
			}
			if err := tc.mutate(t, m, ctx); err != nil {
				t.Fatalf("变更 %s 失败: %v", tc.name, err)
			}
			after, err := m.db.ArtifactVersion(ctx)
			if err != nil {
				t.Fatalf("读后值失败: %v", err)
			}
			if after != before+1 {
				t.Fatalf("%s 后 ArtifactVersion = %d，期望恰好 %d（前值 %d）", tc.name, after, before+1, before)
			}
		})
	}
}

// TestBumpArtifactVersion_TrafficNoBump 覆盖例外清单：机场 traffic 列更新方法
// （upload/download/total/expire/node_count/traffic_at/traffic_err）不 bump——
// 流量只影响 Subscription-Userinfo 响应头，响应头在 serve 时现算。
func TestBumpArtifactVersion_TrafficNoBump(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, err := m.AddAirport(ctx, "机场流量", "http://up.example/traffic", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}

	before, err := m.db.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读前值失败: %v", err)
	}

	// u != nil：覆盖流量列 + node_count + traffic_at
	u := traffic.Usage{Upload: 1, Download: 2, Total: 3, Expire: 4}
	if err := m.UpdateAirportTraffic(ctx, a.ID, &u, 7, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic(u!=nil): %v", err)
	}
	// u == nil：只写 traffic_err
	if err := m.UpdateAirportTraffic(ctx, a.ID, nil, 0, "上游超时"); err != nil {
		t.Fatalf("UpdateAirportTraffic(u==nil): %v", err)
	}
	// 只回写 node_count
	if err := m.UpdateAirportNodeCount(ctx, a.ID, 9); err != nil {
		t.Fatalf("UpdateAirportNodeCount: %v", err)
	}

	after, err := m.db.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读后值失败: %v", err)
	}
	if after != before {
		t.Fatalf("traffic 更新后 ArtifactVersion = %d，期望不变（前值 %d）", after, before)
	}
}
