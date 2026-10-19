package subforge

// sub_token_config_test.go — Manager 层订阅令牌五字段的读写与转发测试。
// 关注点：CreateSubToken 的 7 列写入、GetSubToken 的 COALESCE 读出、
// UpdateSubToken 部分更新语义与 sql.ErrNoRows 透传（HTTP 层 404 依赖它）。

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/store"
)

func TestManagerSubTokenFieldsRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if err := m.CreateSubToken(ctx, "tok1", "名字", "list", "p1", "c1", "vless", "vmess"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}

	got, err := m.GetSubToken(ctx, "tok1")
	if err != nil {
		t.Fatalf("GetSubToken: %v", err)
	}
	if got.Token != "tok1" || got.Name != "名字" ||
		got.Target != "list" || got.Pipeline != "p1" || got.Config != "c1" ||
		got.Include != "vless" || got.Exclude != "vmess" {
		t.Fatalf("GetSubToken = %+v，与写入不符", got)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt 为零值，期望库默认时间戳")
	}

	list, err := m.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	if len(list) != 1 || list[0].Include != "vless" || list[0].Exclude != "vmess" {
		t.Fatalf("ListSubTokens = %+v", list)
	}
}

func TestManagerGetSubTokenNotFound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	got, err := m.GetSubToken(ctx, "missing")
	if err == nil {
		t.Fatalf("GetSubToken(不存在) 应报错，got %+v", got)
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("错误信息应含 token 名: %v", err)
	}
	if got != nil {
		t.Errorf("失败时 token 应为 nil，got %+v", got)
	}
}

func TestManagerUpdateSubToken(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.CreateSubToken(ctx, "tok1", "n1", "list", "p1", "c1", "vless", "vmess"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}

	// 只改 target，其余保留。
	target := "clash.meta"
	if err := m.UpdateSubToken(ctx, "tok1", store.SubTokenUpdate{Target: &target}); err != nil {
		t.Fatalf("UpdateSubToken: %v", err)
	}
	got, err := m.GetSubToken(ctx, "tok1")
	if err != nil {
		t.Fatalf("GetSubToken: %v", err)
	}
	if got.Target != "clash.meta" || got.Pipeline != "p1" || got.Config != "c1" ||
		got.Include != "vless" || got.Exclude != "vmess" || got.Name != "n1" {
		t.Fatalf("部分更新误伤其他列: %+v", got)
	}

	// 空串清空 exclude。
	empty := ""
	if err := m.UpdateSubToken(ctx, "tok1", store.SubTokenUpdate{Exclude: &empty}); err != nil {
		t.Fatalf("UpdateSubToken(清空): %v", err)
	}
	got, _ = m.GetSubToken(ctx, "tok1")
	if got.Exclude != "" {
		t.Errorf("Exclude=%q，期望清空", got.Exclude)
	}

	// 不存在的 token：sql.ErrNoRows 必须透传（HTTP 层据此回 404）。
	if err := m.UpdateSubToken(ctx, "missing", store.SubTokenUpdate{Target: &target}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateSubToken(不存在) 错误=%v，期望 sql.ErrNoRows", err)
	}
}
