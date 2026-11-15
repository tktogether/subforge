package subforge

// Manager 产物门面与 store 外 bump 点的用例（订阅产物预生成 任务 3）。
//
//   - ArtifactsEnabled：未配置 = 默认开；mode 关 / 刷新间隔 0 / 读配置出错都按禁用；
//   - 门面透传：Bump/Get/Save/Delete 与 m.db 直调等值；
//   - store 外 bump 点：规则源版本文件落盘（recordCurrentVersions）与回滚（RollbackRuleSource）
//     是文件操作，store 的 touchArtifacts 够不着，Manager 层显式 bump。

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/manifest"
	"github.com/tktogether/subforge/internal/store"
)

// TestArtifactsEnabledDefaults 覆盖开关判断：未配置默认开，mode 关或刷新间隔 0 都禁用。
func TestArtifactsEnabledDefaults(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if !m.ArtifactsEnabled(ctx) {
		t.Fatal("未配置时默认应开启")
	}

	// mode 显式关闭 → 禁用。
	if err := m.SetSettings(ctx, map[string]string{ArtifactModeKey: "0"}); err != nil {
		t.Fatalf("SetSettings artifact_mode=0: %v", err)
	}
	if m.ArtifactsEnabled(ctx) {
		t.Fatal("artifact_mode=0 应禁用")
	}
	// 重新打开（其余保持默认）→ 恢复启用。
	if err := m.SetSettings(ctx, map[string]string{ArtifactModeKey: "1"}); err != nil {
		t.Fatalf("SetSettings artifact_mode=1: %v", err)
	}
	if !m.ArtifactsEnabled(ctx) {
		t.Fatal("artifact_mode=1 且刷新间隔默认 30 分钟应启用")
	}

	// 刷新间隔 0（缓存关闭）→ 禁用，即使 mode 开着。
	if err := m.SetSettings(ctx, map[string]string{SourceRefreshKey: "0"}); err != nil {
		t.Fatalf("SetSettings refresh=0: %v", err)
	}
	if m.ArtifactsEnabled(ctx) {
		t.Fatal("refresh=0 应禁用（产物依赖缓存 TTL 失效语义）")
	}

	// 两者都关 → 禁用。
	if err := m.SetSettings(ctx, map[string]string{ArtifactModeKey: "off"}); err != nil {
		t.Fatalf("SetSettings artifact_mode=off: %v", err)
	}
	if m.ArtifactsEnabled(ctx) {
		t.Fatal("mode=off 且 refresh=0 应禁用")
	}
}

// TestArtifactsEnabledOnConfigError 覆盖读配置出错按禁用处理：关掉底层库后 GetConfig 必失败。
func TestArtifactsEnabledOnConfigError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m, err := NewManager(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if m.ArtifactsEnabled(ctx) {
		t.Fatal("读配置出错应按禁用处理")
	}
}

// TestArtifactFacadePassthrough 覆盖门面透传：Manager 方法与 m.db 直调行为等值。
func TestArtifactFacadePassthrough(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	// 版本号：未 bump 时为 0，Manager.BumpArtifactVersion 每次 +1，ArtifactVersion 与 db 一致。
	v, err := m.ArtifactVersion(ctx)
	if err != nil || v != 0 {
		t.Fatalf("初始版本 = %d err=%v, want 0", v, err)
	}
	if err := m.BumpArtifactVersion(ctx); err != nil {
		t.Fatalf("BumpArtifactVersion: %v", err)
	}
	dbV, err := m.db.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("db.ArtifactVersion: %v", err)
	}
	v, err = m.ArtifactVersion(ctx)
	if err != nil || v != dbV || v != 1 {
		t.Fatalf("bump 后版本 = %d (db=%d) err=%v, want 1", v, dbV, err)
	}

	// 产物读写删：Save 后 Get 等值，Delete 后回到 (nil, nil)。
	want := store.SubArtifact{
		Token:       "tok-1",
		ProfileID:   7,
		Version:     1,
		Body:        []byte("body-bytes"),
		ETag:        `"v1"`,
		GeneratedAt: time.Unix(1700000000, 0).UTC(),
	}
	if err := m.SaveSubArtifact(ctx, want); err != nil {
		t.Fatalf("SaveSubArtifact: %v", err)
	}
	got, err := m.GetSubArtifact(ctx, want.Token)
	if err != nil {
		t.Fatalf("GetSubArtifact: %v", err)
	}
	if got == nil {
		t.Fatal("Save 后 Get 不应为 nil")
	}
	if got.Token != want.Token || got.ProfileID != want.ProfileID || got.Version != want.Version ||
		got.ETag != want.ETag || !got.GeneratedAt.Equal(want.GeneratedAt) ||
		!bytes.Equal(got.Body, want.Body) {
		t.Fatalf("Get = %+v, want %+v", got, &want)
	}
	// db 直读同值，证明透传等值。
	dbGot, err := m.db.GetSubArtifact(ctx, want.Token)
	if err != nil || dbGot == nil || !bytes.Equal(dbGot.Body, want.Body) {
		t.Fatalf("db.GetSubArtifact = %+v err=%v", dbGot, err)
	}

	// 不存在的令牌：(nil, nil)。
	miss, err := m.GetSubArtifact(ctx, "no-such-token")
	if err != nil || miss != nil {
		t.Fatalf("未命中 = %+v err=%v, want nil,nil", miss, err)
	}

	if err := m.DeleteSubArtifact(ctx, want.Token); err != nil {
		t.Fatalf("DeleteSubArtifact: %v", err)
	}
	if got, err := m.GetSubArtifact(ctx, want.Token); err != nil || got != nil {
		t.Fatalf("删除后 Get = %+v err=%v, want nil,nil", got, err)
	}
}

// TestRecordCurrentVersionsArtifactBump 覆盖规则源更新（文件落盘路径）的 bump 点：
// 结果列表为空或全部失败时不 bump；至少一个源成功时末尾恰好 bump 一次。
func TestRecordCurrentVersionsArtifactBump(t *testing.T) {
	ctx := context.Background()

	t.Run("空结果不 bump", func(t *testing.T) {
		m := newTestManager(t)
		before, err := m.ArtifactVersion(ctx)
		if err != nil {
			t.Fatalf("读前值: %v", err)
		}
		m.recordCurrentVersions(ctx, nil)
		m.recordCurrentVersions(ctx, []manifest.UpdateResult{})
		after, err := m.ArtifactVersion(ctx)
		if err != nil {
			t.Fatalf("读后值: %v", err)
		}
		if after != before {
			t.Fatalf("空结果后版本 = %d, want 不变 %d", after, before)
		}
	})

	t.Run("全部失败不 bump", func(t *testing.T) {
		m := newTestManager(t)
		results := []manifest.UpdateResult{
			{Source: "a", Err: errors.New("拉取失败")},
			{Source: "b", Path: "", Err: errors.New("写盘失败")},
			{Source: "", Path: filepath.Join(t.TempDir(), "x.txt")},
		}
		before, err := m.ArtifactVersion(ctx)
		if err != nil {
			t.Fatalf("读前值: %v", err)
		}
		m.recordCurrentVersions(ctx, results)
		after, err := m.ArtifactVersion(ctx)
		if err != nil {
			t.Fatalf("读后值: %v", err)
		}
		if after != before {
			t.Fatalf("全部失败后版本 = %d, want 不变 %d", after, before)
		}
	})

	t.Run("成功更新恰好 bump 一次", func(t *testing.T) {
		m := newTestManager(t)
		outDir := filepath.Join(t.TempDir(), "out")
		names := seedVersionFiles(t, outDir, "a", "txt", "20240103-000000")
		// 一个成功 + 一个失败：只要有一个成功就该 bump。
		results := []manifest.UpdateResult{
			{Source: "a", Version: "20240103-000000", Path: filepath.Join(outDir, names[0])},
			{Source: "b", Err: errors.New("拉取失败")},
		}
		before, err := m.ArtifactVersion(ctx)
		if err != nil {
			t.Fatalf("读前值: %v", err)
		}
		m.recordCurrentVersions(ctx, results)
		after, err := m.ArtifactVersion(ctx)
		if err != nil {
			t.Fatalf("读后值: %v", err)
		}
		if after != before+1 {
			t.Fatalf("成功更新后版本 = %d, want 恰好 %d", after, before+1)
		}
	})
}

// TestRollbackRuleSourceArtifactBump 覆盖回滚（当前版本指针是文件操作）的 bump 点：
// 成功回滚后 bump；「可回滚版本不足」的失败回滚不 bump。
func TestRollbackRuleSourceArtifactBump(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	seedVersions(t, outDir, "a", "txt")
	writeRuleManifest(t, m, srv.URL, "text", outDir)

	before, err := m.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读前值: %v", err)
	}
	if _, err := m.RollbackRuleSource(ctx, "a"); err != nil {
		t.Fatalf("RollbackRuleSource: %v", err)
	}
	after, err := m.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读后值: %v", err)
	}
	if after != before+1 {
		t.Fatalf("成功回滚后版本 = %d, want 恰好 %d", after, before+1)
	}

	// 已到最早版本，回滚失败不得 bump。
	if _, err := m.RollbackRuleSource(ctx, "a"); !errors.Is(err, ErrRollbackInsufficient) {
		t.Fatalf("第二次回滚应报可回滚版本不足, got %v", err)
	}
	after2, err := m.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读二次后值: %v", err)
	}
	if after2 != after {
		t.Fatalf("失败回滚后版本 = %d, want 不变 %d", after2, after)
	}
}

// TestUpdateRuleSourcesEndToEndBump 覆盖全量更新链路：经 recordCurrentVersions 恰好 +1。
func TestUpdateRuleSourcesEndToEndBump(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))

	before, err := m.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读前值: %v", err)
	}
	results, err := m.UpdateRuleSources(ctx)
	if err != nil {
		t.Fatalf("UpdateRuleSources: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v", results)
	}
	after, err := m.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读后值: %v", err)
	}
	if after != before+1 {
		t.Fatalf("全量更新后版本 = %d, want 恰好 %d", after, before+1)
	}
}

// TestBumpArtifactsAfterFileWriteHelper 覆盖文件路径 bump 助手：成功 +1 且不返回错误。
func TestBumpArtifactsAfterFileWriteHelper(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	before, err := m.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读前值: %v", err)
	}
	m.bumpArtifactsAfterFileWrite(ctx)
	after, err := m.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读后值: %v", err)
	}
	if after != before+1 {
		t.Fatalf("bump 后版本 = %d, want %d", after, before+1)
	}
}
