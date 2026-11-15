// artifact_rebuild_test.go — 订阅产物后台重建器（任务 7）：rebuildArtifacts 单轮语义
// 的端到端测试。装配沿用 profileServer（暴露 *Server 供直调 rebuildArtifacts），
// 测试一律同步调用单轮方法，不依赖轮询 goroutine（StartArtifactRebuilder 的契约
// 用已取消的 ctx 单测）。
//
// 比较口径：重建产物与实时编译的 body 有两行天然不同——注释头的「生成时间」行
// （每次渲染现取、秒级精度）与 mihomo 内核给 ws 链接注入的随机 User-Agent 行
// ——归一化两行后再比较。正则复用既有特征测试的 wsUARx（subscribe_query_test.go）
// 与 generatedAtRe（token_fields_test.go），不重造。
// 版本号断言一律相对（rebuild 后 == 当前版本号），不写绝对数。
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/subforge"
)

// rebuildEnv 起一个「启用机场 + 绑定/未绑定两个令牌」的服务，并暴露 *Server。
// artifactPoll 设成极小值只为后续 StartArtifactRebuilder 的生命周期测试兜底；
// 单轮语义测试不启动 goroutine。
func rebuildEnv(t *testing.T) (*httptest.Server, *Server, *subforge.Manager, subforge.Profile) {
	t.Helper()
	ts, srv, mgr := profileServer(t)
	srv.artifactPoll = 10 * time.Millisecond
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	profile, err := mgr.AddProfile(ctx, "方案A", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("插入 profile: %v", err)
	}
	if err := mgr.CreateSubTokenWithProfile(ctx, "tok-bound", "绑定", "", "", "", "", "", profile.ID); err != nil {
		t.Fatalf("插入绑定令牌: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-plain", "不绑", "", "", "", "", ""); err != nil {
		t.Fatalf("插入普通令牌: %v", err)
	}
	return ts, srv, mgr, *profile
}

// normArtifactBody 折叠订阅正文里两行非确定内容（生成时间行、ws 链接随机
// User-Agent 行），供重建产物与实时 body 的稳定比较。
func normArtifactBody(s string) string {
	return generatedAtRe.ReplaceAllString(wsUARx.ReplaceAllString(s, "User-Agent: UA"), "生成时间: TS")
}

// TestRebuildArtifacts_WarmsAllTokens 两个令牌（一个绑方案一个不绑）→ rebuildArtifacts
// 后产物表两行；归属（ProfileID）、版本号与 ETag 正确；body 与实时编译一致
// （生成时间行与随机 User-Agent 行归一化后）。
func TestRebuildArtifacts_WarmsAllTokens(t *testing.T) {
	ts, srv, mgr, profile := rebuildEnv(t)
	ctx := context.Background()

	srv.rebuildArtifacts(ctx)

	for _, token := range []string{"tok-bound", "tok-plain"} {
		art, err := mgr.GetSubArtifact(ctx, token)
		if err != nil {
			t.Fatalf("读产物 %s: %v", token, err)
		}
		if art == nil {
			t.Fatalf("rebuildArtifacts 后令牌 %s 应有产物", token)
		}
	}
	bound, _ := mgr.GetSubArtifact(ctx, "tok-bound")
	plain, _ := mgr.GetSubArtifact(ctx, "tok-plain")

	// 归属：绑定令牌记方案 id，未绑定记 0。
	if bound.ProfileID != profile.ID {
		t.Fatalf("绑定令牌产物 profile_id = %d，应为 %d", bound.ProfileID, profile.ID)
	}
	if plain.ProfileID != 0 {
		t.Fatalf("未绑定令牌产物 profile_id = %d，应为 0", plain.ProfileID)
	}

	// 版本号与 ETag：rebuild 落库用本轮读到的当前版本号，ETag 与命中路径同公式。
	cur, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if bound.Version != cur || plain.Version != cur {
		t.Fatalf("产物版本 %d/%d 应等于当前版本 %d", bound.Version, plain.Version, cur)
	}
	if bound.ETag != computeArtifactETag(cur, "tok-bound") {
		t.Fatalf("绑定令牌 ETag %s 与公式不符 %s", bound.ETag, computeArtifactETag(cur, "tok-bound"))
	}

	// 重建产物 vs 实时编译：target=clash.meta 是非默认组合，绕开产物命中拿到
	// 真正的实时 body；显式 target 与默认回落同目标，Params 等价、输出等价。
	for _, token := range []string{"tok-bound", "tok-plain"} {
		art, _ := mgr.GetSubArtifact(ctx, token)
		status, _, live := uiGet(t, ts, "/subscribe?token="+token+"&target=clash.meta")
		if status != http.StatusOK {
			t.Fatalf("实时拉取 %s status=%d body=%s", token, status, live)
		}
		if normArtifactBody(string(art.Body)) != normArtifactBody(live) {
			t.Fatalf("令牌 %s 重建产物与实时编译不一致：len %d vs %d", token, len(art.Body), len(live))
		}
	}
}

// TestRebuildArtifacts_DeletesOnRequiredFailure 机场 failure_policy=error 指向不可达
// 地址 → rebuild 后该令牌无产物；再拉取 → 502。分两步覆盖 rebuildOneToken 的两条
// 失败删除路径：健康源 + 必失败源并存时 engine.Run 部分成功、requiredFailedSources
// 非空；把健康源停用后所有来源失败、engine.Run 报错。两条路径都必须删产物。
func TestRebuildArtifacts_DeletesOnRequiredFailure(t *testing.T) {
	// 不可达上游：先起再关，端口必然拒绝连接，失败原因确定。
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	ts, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()
	critical, err := mgr.AddAirport(ctx, "关键机场", deadURL+"/sub", "", true)
	if err != nil {
		t.Fatalf("插入机场: %v", err)
	}
	if err := mgr.SetAirportFailurePolicy(ctx, critical.ID, subforge.AirportFailureError); err != nil {
		t.Fatalf("设置失败策略: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-dead", "死源", "", "", "", "", ""); err != nil {
		t.Fatalf("插入令牌: %v", err)
	}

	// 阶段一：健康源（fixture）+ 必失败源并存 → requiredFailedSources 非空 → 删产物。
	srv.rebuildArtifacts(ctx)
	art, err := mgr.GetSubArtifact(ctx, "tok-dead")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if art != nil {
		t.Fatalf("必失败源（error 策略）失败的令牌不应留产物，got version=%d", art.Version)
	}

	// 再拉取：产物已被删，实时编译同样命中必失败源 → 502。
	st, _, body := uiGet(t, ts, "/subscribe?token=tok-dead")
	if st != http.StatusBadGateway {
		t.Fatalf("拉取 status=%d body=%s，error 策略下应 502", st, body)
	}

	// 阶段二：停用健康源 → 剩余唯一来源必失败 → engine.Run 报错 → 同样删产物。
	healthy, herr := mgr.ListAirports(ctx)
	if herr != nil {
		t.Fatalf("列机场: %v", herr)
	}
	for _, a := range healthy {
		if a.ID == critical.ID {
			continue
		}
		if _, err := mgr.UpdateAirport(ctx, a.ID, a.Name, a.SubscriptionURL, a.Remarks, false); err != nil {
			t.Fatalf("停用机场 %s: %v", a.Name, err)
		}
	}
	srv.rebuildArtifacts(ctx)
	art2, err := mgr.GetSubArtifact(ctx, "tok-dead")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if art2 != nil {
		t.Fatalf("engine.Run 失败的令牌不应留产物，got version=%d", art2.Version)
	}
}

// TestRebuildArtifacts_PicksUpVersionBump 先 rebuild，改分组（写路径自动 bump 全局
// 版本号）→ rebuildArtifacts 再次执行 → 产物 version == 当前版本。
func TestRebuildArtifacts_PicksUpVersionBump(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()

	srv.rebuildArtifacts(ctx)
	art1, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil || art1 == nil {
		t.Fatalf("首轮 rebuild 后应有产物: %v", err)
	}
	cur1, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if art1.Version != cur1 {
		t.Fatalf("首轮产物版本 %d 应等于当前版本 %d", art1.Version, cur1)
	}

	// 改分组：真实写路径，自动 bump 版本号（相对断言：+1）。
	if _, err := mgr.AddProxyGroup(ctx, "新分组", "select", ".*", "", 0, 0); err != nil {
		t.Fatalf("加分组: %v", err)
	}
	cur2, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if cur2 != cur1+1 {
		t.Fatalf("加分组后版本号应 +1：%d → %d", cur1, cur2)
	}

	srv.rebuildArtifacts(ctx)
	art2, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil || art2 == nil {
		t.Fatalf("二轮 rebuild 后应有产物: %v", err)
	}
	if art2.Version != cur2 {
		t.Fatalf("二轮产物版本 %d 应等于 bump 后的当前版本 %d", art2.Version, cur2)
	}
}

// TestRebuildArtifacts_DisabledModeSkips artifact_mode=0 → rebuildArtifacts 整轮跳过，
// 产物表仍空。
func TestRebuildArtifacts_DisabledModeSkips(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()
	if err := mgr.SetSettings(ctx, map[string]string{subforge.ArtifactModeKey: "0"}); err != nil {
		t.Fatalf("关闭产物模式: %v", err)
	}

	srv.rebuildArtifacts(ctx)

	for _, token := range []string{"tok-bound", "tok-plain"} {
		art, err := mgr.GetSubArtifact(ctx, token)
		if err != nil {
			t.Fatalf("读产物 %s: %v", token, err)
		}
		if art != nil {
			t.Fatalf("模式关闭时 rebuildArtifacts 不应落产物，令牌 %s got version=%d", token, art.Version)
		}
	}
}

// TestStartArtifactRebuilderLifecycle StartArtifactRebuilder 的启动契约：
// subManager 为 nil → 空操作；ctx 已取消 → 预热轮不执行、goroutine 立即退出
// （产物表保持空）。轮询 goroutine 的正常路径不在此测——单轮语义已由上面
// 同步调用覆盖。
func TestStartArtifactRebuilderLifecycle(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 已取消：预热轮与轮询都必须直接退出
	srv.StartArtifactRebuilder(ctx)

	time.Sleep(100 * time.Millisecond)
	art, err := mgr.GetSubArtifact(context.Background(), "tok-plain")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if art != nil {
		t.Fatalf("已取消的 ctx 不应触发预热重建，got version=%d", art.Version)
	}

	// nil subManager：空操作，不 panic、不 spawn。
	(&Server{}).StartArtifactRebuilder(context.Background())
}
