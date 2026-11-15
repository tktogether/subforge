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

	"github.com/tktogether/subforge/internal/store"
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

// --- 任务 10：条件覆盖补测——重建器各错误/跳过臂的真实行为用例 ---
//
// 故障注入手段全部落在测试自建的临时 SQLite 上（mgr.DB().Raw() 直连同一连接池）：
// 触发器 RAISE(ABORT) 让 DELETE/INSERT 必失败、脏数据让 Scan 必失败、DROP TABLE
// 让整表读必失败——都是真实数据库层面的失败，不 mock。每个用例断言的可见结果：
// 跳过/保留/重建，而不是「跑过了就行」。

// rawExec 通过 Manager 底层连接池执行一条 SQL（本节故障注入专用）。
func rawExec(t *testing.T, mgr *subforge.Manager, query string, args ...any) {
	t.Helper()
	if _, err := mgr.DB().Raw().Exec(query, args...); err != nil {
		t.Fatalf("执行注入 SQL %q: %v", query, err)
	}
}

// forbidArtifactDelete 建 BEFORE DELETE 触发器，令 DeleteSubArtifact 必失败
// （RAISE(ABORT)；行存在时才触发，删不存在的行依旧成功——与真实删除语义一致）。
func forbidArtifactDelete(t *testing.T, mgr *subforge.Manager) {
	t.Helper()
	rawExec(t, mgr,
		"CREATE TRIGGER subforge_test_no_del BEFORE DELETE ON sub_artifacts "+
			"BEGIN SELECT RAISE(ABORT, 'test: 产物删除被禁'); END;")
}

// forbidArtifactSave 建 BEFORE INSERT 触发器，令 SaveSubArtifact 必失败。
func forbidArtifactSave(t *testing.T, mgr *subforge.Manager) {
	t.Helper()
	rawExec(t, mgr,
		"CREATE TRIGGER subforge_test_no_ins BEFORE INSERT ON sub_artifacts "+
			"BEGIN SELECT RAISE(ABORT, 'test: 产物写入被禁'); END;")
}

// TestStartArtifactRebuilder_ZeroPollUsesDefault artifactPoll 为零值（未配置）时按
// defaultArtifactPoll 兜底启动（StartArtifactRebuilder 的 poll<=0 真臂）。ctx 已取消：
// 预热轮与轮询 goroutine 都立即退出，产物表保持空。
func TestStartArtifactRebuilder_ZeroPollUsesDefault(t *testing.T) {
	// 注意：直接用 profileServer，不经过 rebuildEnv——后者会把 poll 设成 10ms（假臂）。
	_, srv, mgr := profileServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-plain", "不绑", "", "", "", "", ""); err != nil {
		t.Fatalf("插入令牌: %v", err)
	}
	if srv.artifactPoll != 0 {
		t.Fatalf("前置条件：profileServer 的 artifactPoll 应为零值，got %v", srv.artifactPoll)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	srv.StartArtifactRebuilder(cancelled)

	time.Sleep(100 * time.Millisecond)
	art, err := mgr.GetSubArtifact(context.Background(), "tok-plain")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if art != nil {
		t.Fatalf("零值 poll + 已取消 ctx 不应触发预热重建，got version=%d", art.Version)
	}
}

// TestRebuildArtifacts_SkipsRoundOnVersionReadError 版本号 config 值被写坏（非整数）
// → ArtifactVersion 报错 → 整轮跳过：已有产物原样保留（不重建、不误删）。
func TestRebuildArtifacts_SkipsRoundOnVersionReadError(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()

	// 先正常重建一轮，两个令牌都有产物。
	srv.rebuildArtifacts(ctx)
	cur, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}

	// 写坏版本号：真实的脏数据（手动改库），ArtifactVersion 解析必报错。
	rawExec(t, mgr, "UPDATE config SET value = 'not-an-int' WHERE key = ?", store.ArtifactVersionConfigKey)
	srv.rebuildArtifacts(ctx)

	for _, token := range []string{"tok-bound", "tok-plain"} {
		art, err := mgr.GetSubArtifact(ctx, token)
		if err != nil {
			t.Fatalf("读产物 %s: %v", token, err)
		}
		if art == nil || art.Version != cur {
			t.Fatalf("版本号读失败后整轮应跳过：令牌 %s 的产物应保持 version=%d 原样，got %+v", token, cur, art)
		}
	}
}

// TestRebuildArtifacts_SkipsRoundOnListSubTokensError 令牌表整表不可读（DROP）→
// ListSubTokens 报错 → 整轮跳过：已有产物原样保留。
func TestRebuildArtifacts_SkipsRoundOnListSubTokensError(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()

	srv.rebuildArtifacts(ctx)
	cur, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}

	rawExec(t, mgr, "DROP TABLE sub_tokens")
	srv.rebuildArtifacts(ctx)

	for _, token := range []string{"tok-bound", "tok-plain"} {
		art, err := mgr.GetSubArtifact(ctx, token)
		if err != nil {
			t.Fatalf("读产物 %s: %v", token, err)
		}
		if art == nil || art.Version != cur {
			t.Fatalf("令牌表读失败后整轮应跳过：令牌 %s 的产物应保持 version=%d 原样，got %+v", token, cur, art)
		}
	}
}

// TestRebuildArtifacts_SkipsTokenBoundToDisabledProfile 绑定停用方案的令牌整轮跳过
// （复合条件 !p.Enabled 真臂）：旧产物保留不被误删（跳过正是为了防误删），其它
// 令牌照常重建出新版本产物。
func TestRebuildArtifacts_SkipsTokenBoundToDisabledProfile(t *testing.T) {
	_, srv, mgr := profileServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	off, err := mgr.AddProfile(ctx, "停用方案", "", "", "", false, 0)
	if err != nil {
		t.Fatalf("插入停用方案: %v", err)
	}
	if err := mgr.CreateSubTokenWithProfile(ctx, "tok-off", "停用绑定", "", "", "", "", "", off.ID); err != nil {
		t.Fatalf("插入绑定令牌: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-plain", "不绑", "", "", "", "", ""); err != nil {
		t.Fatalf("插入普通令牌: %v", err)
	}

	// 给停用绑定的令牌预置一份旧产物（真实 SaveSubArtifact），再 bump 制造过期。
	cur, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if err := mgr.SaveSubArtifact(ctx, store.SubArtifact{
		Token: "tok-off", Version: cur, Body: []byte("stale-body"), ETag: computeArtifactETag(cur, "tok-off"),
	}); err != nil {
		t.Fatalf("预置旧产物: %v", err)
	}
	if err := mgr.BumpArtifactVersion(ctx); err != nil {
		t.Fatalf("bump 版本号: %v", err)
	}

	srv.rebuildArtifacts(ctx)

	// 停用绑定的令牌：被跳过，旧产物原样保留。
	art, err := mgr.GetSubArtifact(ctx, "tok-off")
	if err != nil {
		t.Fatalf("读产物 tok-off: %v", err)
	}
	if art == nil || art.Version != cur || string(art.Body) != "stale-body" {
		t.Fatalf("停用绑定的令牌应被跳过且旧产物原样保留 version=%d，got %+v", cur, art)
	}

	// 未绑定令牌：照常重建，落 bump 后的新版本产物。
	plain, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil {
		t.Fatalf("读产物 tok-plain: %v", err)
	}
	if plain == nil || plain.Version != cur+1 || len(plain.Body) == 0 {
		t.Fatalf("未绑定令牌应照常重建出 version=%d 的产物，got %+v", cur+1, plain)
	}
}

// TestRebuildArtifacts_SkipsTokenOnArtifactReadError 单个令牌的产物行损坏
// （profile_id 塞入非整数文本，Scan 必失败）→ 该令牌跳过且行原样保留（不误删），
// 其它令牌照常重建。
func TestRebuildArtifacts_SkipsTokenOnArtifactReadError(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()

	rawExec(t, mgr, "INSERT INTO sub_artifacts (token, profile_id, version, body, etag, generated_at) "+
		"VALUES ('tok-plain', 'corrupted', 1, 'old-body', 'e', 1)")
	if _, err := mgr.GetSubArtifact(ctx, "tok-plain"); err == nil {
		t.Fatalf("前置条件：损坏行应导致 GetSubArtifact 报错")
	}
	if err := mgr.BumpArtifactVersion(ctx); err != nil {
		t.Fatalf("bump 版本号: %v", err)
	}

	srv.rebuildArtifacts(ctx)

	// 损坏行原样保留（读失败的令牌绝不误删）。
	var version int64
	var body string
	if err := mgr.DB().Raw().QueryRow(
		"SELECT version, body FROM sub_artifacts WHERE token = 'tok-plain'").Scan(&version, &body); err != nil {
		t.Fatalf("读损坏行: %v", err)
	}
	if version != 1 || body != "old-body" {
		t.Fatalf("读失败的令牌应被跳过且损坏行原样保留，got version=%d body=%q", version, body)
	}

	// 健康令牌照常重建。
	bound, err := mgr.GetSubArtifact(ctx, "tok-bound")
	if err != nil {
		t.Fatalf("读产物 tok-bound: %v", err)
	}
	cur, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if bound == nil || bound.Version != cur {
		t.Fatalf("健康令牌应照常重建出 version=%d 的产物，got %+v", cur, bound)
	}
}

// TestRebuildArtifacts_RebuildsAgedArtifact 产物版本仍是当前版本、但年龄超过刷新
// 间隔 → 视为过期并重建（年龄臂真值）：重建后 GeneratedAt 刷新、版本号不变。
func TestRebuildArtifacts_RebuildsAgedArtifact(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()

	srv.rebuildArtifacts(ctx)
	old, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil || old == nil {
		t.Fatalf("首轮 rebuild 后应有产物: %v", err)
	}
	cur, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if old.Version != cur {
		t.Fatalf("前置条件：产物版本 %d 应等于当前版本 %d", old.Version, cur)
	}

	// 同版本重写把时间戳改到 2 小时前（默认刷新间隔 30 分钟）：版本守卫允许同版本
	// 幂等刷新，产物变成「版本最新但超龄」。
	aged := time.Now().Add(-2 * time.Hour).UTC()
	if err := mgr.SaveSubArtifact(ctx, store.SubArtifact{
		Token: "tok-plain", ProfileID: old.ProfileID, Version: cur,
		Body: old.Body, ETag: old.ETag, GeneratedAt: aged,
	}); err != nil {
		t.Fatalf("把产物时间戳改旧: %v", err)
	}

	srv.rebuildArtifacts(ctx)

	fresh, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if fresh == nil {
		t.Fatalf("超龄产物应被重建")
	}
	if fresh.Version != cur {
		t.Fatalf("重建后版本应仍是当前版本 %d，got %d", cur, fresh.Version)
	}
	if !fresh.GeneratedAt.After(aged) {
		t.Fatalf("超龄产物应被重建出新时间戳（晚于 %v），got %v", aged, fresh.GeneratedAt)
	}
}

// TestRebuildOneTokenBuildFailureKeepsArtifactOnDeleteError 构建失败（全部机场停用
// → 没有启用的订阅 → 400）时删旧产物；删除本身被触发器拒绝 → 只记日志、旧产物
// 原样保留（status!=0 真臂 + 删除失败臂两臂）。
func TestRebuildOneTokenBuildFailureKeepsArtifactOnDeleteError(t *testing.T) {
	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()

	srv.rebuildArtifacts(ctx)
	old, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil || old == nil {
		t.Fatalf("首轮 rebuild 后应有产物: %v", err)
	}
	staleCur := old.Version

	forbidArtifactDelete(t, mgr)
	// 停用全部机场：buildSubscription 的「没有启用的订阅或自建节点」→ 400。
	airports, err := mgr.ListAirports(ctx)
	if err != nil {
		t.Fatalf("列机场: %v", err)
	}
	for _, a := range airports {
		if _, err := mgr.UpdateAirport(ctx, a.ID, a.Name, a.SubscriptionURL, a.Remarks, false); err != nil {
			t.Fatalf("停用机场 %s: %v", a.Name, err)
		}
	}
	if err := mgr.BumpArtifactVersion(ctx); err != nil {
		t.Fatalf("bump 版本号: %v", err)
	}

	srv.rebuildArtifacts(ctx)

	// 构建失败路径尝试删旧产物，但删除失败：产物原样保留（未被删除、也未被重写）。
	kept, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if kept == nil {
		t.Fatalf("构建失败且删除失败时旧产物应保留（只记日志，下轮重建重试）")
	}
	if kept.Version != staleCur {
		t.Fatalf("旧产物应原样保留 version=%d，got version=%d（被重写说明没走构建失败路径）", staleCur, kept.Version)
	}
}

// TestRebuildArtifactsDeleteFailureKeepsArtifactOnRunFailures 引擎两段失败路径
// （必失败源失败 / engine.Run 报错）都尝试删旧产物，删除被触发器拒绝 → 只记日志、
// 产物保留（两条删除失败臂的真值）。
func TestRebuildArtifactsDeleteFailureKeepsArtifactOnRunFailures(t *testing.T) {
	// 不可达上游：先起再关，端口必然拒绝连接，失败原因确定。
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	_, srv, mgr, _ := rebuildEnv(t)
	ctx := context.Background()

	// 阶段〇：只有健康源，正常重建，tok-plain 有产物。
	srv.rebuildArtifacts(ctx)
	old, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil || old == nil {
		t.Fatalf("首轮 rebuild 后应有产物: %v", err)
	}
	staleCur := old.Version

	// 阶段一：加 error 策略的死源 → engine.Run 部分成功、requiredFailedSources 非空
	// → 尝试删旧产物；删除被拒绝 → 产物保留。
	critical, err := mgr.AddAirport(ctx, "关键机场", deadURL+"/sub", "", true)
	if err != nil {
		t.Fatalf("插入机场: %v", err)
	}
	if err := mgr.SetAirportFailurePolicy(ctx, critical.ID, subforge.AirportFailureError); err != nil {
		t.Fatalf("设置失败策略: %v", err)
	}
	forbidArtifactDelete(t, mgr)
	if err := mgr.BumpArtifactVersion(ctx); err != nil {
		t.Fatalf("bump 版本号: %v", err)
	}
	srv.rebuildArtifacts(ctx)
	kept, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if kept == nil || kept.Version != staleCur {
		t.Fatalf("必失败源失败 + 删除失败：旧产物应原样保留 version=%d，got %+v", staleCur, kept)
	}

	// 阶段二：停用健康源 → 剩余唯一来源必失败 → engine.Run 报错 → 尝试删除同样被
	// 拒绝 → 产物保留。
	airports, err := mgr.ListAirports(ctx)
	if err != nil {
		t.Fatalf("列机场: %v", err)
	}
	for _, a := range airports {
		if a.ID == critical.ID {
			continue
		}
		if _, err := mgr.UpdateAirport(ctx, a.ID, a.Name, a.SubscriptionURL, a.Remarks, false); err != nil {
			t.Fatalf("停用机场 %s: %v", a.Name, err)
		}
	}
	if err := mgr.BumpArtifactVersion(ctx); err != nil {
		t.Fatalf("bump 版本号: %v", err)
	}
	srv.rebuildArtifacts(ctx)
	kept2, err := mgr.GetSubArtifact(ctx, "tok-plain")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if kept2 == nil || kept2.Version != staleCur {
		t.Fatalf("engine.Run 失败 + 删除失败：旧产物应原样保留 version=%d，got %+v", staleCur, kept2)
	}
}

// TestRebuildArtifacts_StopsWhenContextCancelledMidLoop 令牌循环每轮先查 ctx：第一个
// 令牌的上游拉取中途取消 ctx（机场 HTTP handler 里触发），下一轮检查直接返回，
// 剩余令牌不再处理、不落产物。
func TestRebuildArtifacts_StopsWhenContextCancelledMidLoop(t *testing.T) {
	_, srv, mgr := profileServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}

	// 取消触发器机场：handler 一被拉取就取消 rebuild 的 ctx 再挂住——保证取消
	// 发生在第一个令牌的 engine.Run 期间，且该次拉取必然失败（fetchCtx 随父 ctx 死）。
	loopCtx, cancelLoop := context.WithCancel(context.Background())
	block := make(chan struct{})
	cancelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancelLoop()
		<-block
	}))
	t.Cleanup(func() { close(block); cancelSrv.Close() })
	if _, err := mgr.AddAirport(ctx, "取消机场", cancelSrv.URL+"/sub", "", true); err != nil {
		t.Fatalf("插入取消机场: %v", err)
	}

	if err := mgr.CreateSubToken(ctx, "tok-a", "甲", "", "", "", "", ""); err != nil {
		t.Fatalf("插入令牌甲: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-b", "乙", "", "", "", "", ""); err != nil {
		t.Fatalf("插入令牌乙: %v", err)
	}

	srv.rebuildArtifacts(loopCtx)

	for _, token := range []string{"tok-a", "tok-b"} {
		art, err := mgr.GetSubArtifact(context.Background(), token)
		if err != nil {
			t.Fatalf("读产物 %s: %v", token, err)
		}
		if art != nil {
			t.Fatalf("ctx 中途取消后循环应立即停止，令牌 %s 不应落产物，got version=%d", token, art.Version)
		}
	}
}
