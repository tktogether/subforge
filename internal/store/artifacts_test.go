package store

// 订阅产物（sub_artifacts）与版本号原语的测试。
// 沿用 store_test.go 的 newTestDB 开库方式（临时文件库，不用 :memory:）。

import (
	"testing"
	"time"
)

// TestArtifactVersion_Bump 覆盖版本号原语：新库为 0，Bump 原子 +1 且返回新值。
func TestArtifactVersion_Bump(t *testing.T) {
	d, ctx := newTestDB(t)

	// 新库从未 bump：config 键缺失 → 0
	v0, err := d.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("新库 ArtifactVersion 失败: %v", err)
	}
	if v0 != 0 {
		t.Fatalf("新库 ArtifactVersion = %d，期望 0", v0)
	}

	// Bump 三次：返回值依次 1、2、3（首贴为 1）
	for i := int64(1); i <= 3; i++ {
		got, err := d.BumpArtifactVersion(ctx)
		if err != nil {
			t.Fatalf("第 %d 次 BumpArtifactVersion 失败: %v", i, err)
		}
		if got != i {
			t.Fatalf("第 %d 次 BumpArtifactVersion 返回 %d，期望 %d", i, got, i)
		}
	}

	// 再读一遍：与 Bump 返回值一致
	v, err := d.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("ArtifactVersion 失败: %v", err)
	}
	if v != 3 {
		t.Fatalf("三次 Bump 后 ArtifactVersion = %d，期望 3", v)
	}
}

// TestSubArtifact_CRUD 覆盖 Save → Get 往返与 Delete → (nil, nil)。
func TestSubArtifact_CRUD(t *testing.T) {
	d, ctx := newTestDB(t)

	// 缓存未命中：不存在返回 (nil, nil)
	got, err := d.GetSubArtifact(ctx, "tok1")
	if err != nil {
		t.Fatalf("空库 GetSubArtifact 失败: %v", err)
	}
	if got != nil {
		t.Fatalf("空库 GetSubArtifact = %+v，期望 nil", got)
	}

	gen := time.Unix(1700000000, 0).UTC()
	in := SubArtifact{
		Token:       "tok1",
		ProfileID:   7,
		Version:     2,
		Body:        []byte("proxies: [a, b]"),
		ETag:        `"v2"`,
		GeneratedAt: gen,
	}
	if err := d.SaveSubArtifact(ctx, in); err != nil {
		t.Fatalf("SaveSubArtifact: %v", err)
	}

	out, err := d.GetSubArtifact(ctx, "tok1")
	if err != nil {
		t.Fatalf("GetSubArtifact: %v", err)
	}
	if out == nil {
		t.Fatal("Save 后 GetSubArtifact = nil，期望有值")
	}
	if out.Token != in.Token || out.ProfileID != in.ProfileID || out.Version != in.Version {
		t.Fatalf("GetSubArtifact 字段不符: %+v", out)
	}
	if string(out.Body) != string(in.Body) {
		t.Fatalf("Body 往返不一致: %q vs %q", out.Body, in.Body)
	}
	if out.ETag != in.ETag {
		t.Fatalf("ETag 往返不一致: %q vs %q", out.ETag, in.ETag)
	}
	if !out.GeneratedAt.Equal(in.GeneratedAt) {
		t.Fatalf("GeneratedAt 往返不一致: %v vs %v", out.GeneratedAt, in.GeneratedAt)
	}

	// Delete 后 Get → (nil, nil)
	if err := d.DeleteSubArtifact(ctx, "tok1"); err != nil {
		t.Fatalf("DeleteSubArtifact: %v", err)
	}
	got2, err := d.GetSubArtifact(ctx, "tok1")
	if err != nil {
		t.Fatalf("Delete 后 GetSubArtifact 失败: %v", err)
	}
	if got2 != nil {
		t.Fatalf("Delete 后 GetSubArtifact = %+v，期望 nil", got2)
	}
}

// TestSubArtifact_VersionGuard 覆盖版本守卫：旧版本写入被拒，新版本覆盖。
func TestSubArtifact_VersionGuard(t *testing.T) {
	d, ctx := newTestDB(t)
	gen := time.Unix(1700000000, 0).UTC()

	// 先存 version=5
	if err := d.SaveSubArtifact(ctx, SubArtifact{Token: "tok", Version: 5, Body: []byte("v5"), ETag: "e5", GeneratedAt: gen}); err != nil {
		t.Fatalf("SaveSubArtifact(version=5): %v", err)
	}

	// 存 version=3：写入被守卫挡住（不报错），仍读到 version=5 的旧行
	if err := d.SaveSubArtifact(ctx, SubArtifact{Token: "tok", Version: 3, Body: []byte("v3"), ETag: "e3", GeneratedAt: gen}); err != nil {
		t.Fatalf("SaveSubArtifact(version=3) 不应报错: %v", err)
	}
	got, err := d.GetSubArtifact(ctx, "tok")
	if err != nil {
		t.Fatalf("GetSubArtifact(version=3 写入后): %v", err)
	}
	if got == nil || got.Version != 5 || string(got.Body) != "v5" || got.ETag != "e5" {
		t.Fatalf("旧版本写入未被守卫挡住: %+v", got)
	}

	// 存 version=6：覆盖为新行
	if err := d.SaveSubArtifact(ctx, SubArtifact{Token: "tok", Version: 6, Body: []byte("v6"), ETag: "e6", GeneratedAt: gen}); err != nil {
		t.Fatalf("SaveSubArtifact(version=6): %v", err)
	}
	got2, err := d.GetSubArtifact(ctx, "tok")
	if err != nil {
		t.Fatalf("GetSubArtifact(version=6 写入后): %v", err)
	}
	if got2 == nil || got2.Version != 6 || string(got2.Body) != "v6" || got2.ETag != "e6" {
		t.Fatalf("新版本写入未覆盖旧行: %+v", got2)
	}
}

// TestDeleteSubToken_CascadesArtifact 覆盖 DeleteSubToken 的级联：删令牌时产物行一并消失。
func TestDeleteSubToken_CascadesArtifact(t *testing.T) {
	d, ctx := newTestDB(t)

	// 建令牌：store 层没有 AddSubToken，沿用 TestListSubTokens 的 raw INSERT 方式
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name) VALUES (?, ?)", "tok-c", "级联测试"); err != nil {
		t.Fatalf("INSERT sub_tokens 失败: %v", err)
	}
	if err := d.SaveSubArtifact(ctx, SubArtifact{Token: "tok-c", Version: 1, Body: []byte("body"), ETag: "e1", GeneratedAt: time.Unix(1700000000, 0).UTC()}); err != nil {
		t.Fatalf("SaveSubArtifact: %v", err)
	}

	// 删令牌：产物级联消失
	if err := d.DeleteSubToken(ctx, "tok-c"); err != nil {
		t.Fatalf("DeleteSubToken: %v", err)
	}
	got, err := d.GetSubArtifact(ctx, "tok-c")
	if err != nil {
		t.Fatalf("级联删除后 GetSubArtifact 失败: %v", err)
	}
	if got != nil {
		t.Fatalf("级联删除后产物仍存在: %+v", got)
	}
	var n int
	if err := d.Raw().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sub_artifacts WHERE token = ?", "tok-c").Scan(&n); err != nil {
		t.Fatalf("COUNT sub_artifacts 失败: %v", err)
	}
	if n != 0 {
		t.Fatalf("级联删除后 sub_artifacts 仍有 %d 行", n)
	}

	// 令牌本身也已删除
	var tok string
	if err := d.Raw().QueryRowContext(ctx,
		"SELECT token FROM sub_tokens WHERE token = ?", "tok-c").Scan(&tok); err == nil {
		t.Fatal("级联删除后令牌仍存在")
	}
}
