// 订阅产物缓存与版本号原语。
//
// /subscribe 的产物预生成：配置变更时 BumpArtifactVersion 全局版本号 +1，
// 渲染产物按令牌落 sub_artifacts 表（带当时的版本号），命中且版本未变时直接回包。
// 版本守卫保证旧版本的过期渲染结果不会覆盖新版本产物。
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// ArtifactVersionConfigKey 是订阅产物全局版本号的 config 键。
const ArtifactVersionConfigKey = "subscribe.artifact_version"

// SubArtifact 是 sub_artifacts 表的一行：某个令牌的预生成订阅产物。
type SubArtifact struct {
	Token       string    `json:"token"`
	ProfileID   int64     `json:"profile_id"`
	Version     int64     `json:"version"`
	Body        []byte    `json:"body"`
	ETag        string    `json:"etag"`
	GeneratedAt time.Time `json:"generated_at"`
}

// ArtifactVersion 读取订阅产物全局版本号；config 键缺失（从未 bump）返回 0。
func (d *DB) ArtifactVersion(ctx context.Context) (int64, error) {
	var raw string
	err := d.db.QueryRowContext(ctx,
		"SELECT value FROM config WHERE key = ?", ArtifactVersionConfigKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return parseArtifactVersion(raw)
}

// BumpArtifactVersion 原子地把订阅产物版本号 +1，返回新值（首次 bump 为 1）。
//
// 刻意不经过任何既有 config 写方法：这里用独立 UPSERT（CAST 让文本列按整数 +1），
// 写、读两条语句各自提交，避免 config 层与本层互相调用形成递归。
func (d *DB) BumpArtifactVersion(ctx context.Context) (int64, error) {
	if _, err := d.db.ExecContext(ctx,
		"INSERT INTO config (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) "+
			"ON CONFLICT(key) DO UPDATE SET value = CAST(value AS INTEGER) + 1, updated_at = CURRENT_TIMESTAMP",
		ArtifactVersionConfigKey, "1"); err != nil {
		return 0, fmt.Errorf("递增订阅产物版本号: %w", err)
	}
	var raw string
	if err := d.db.QueryRowContext(ctx,
		"SELECT value FROM config WHERE key = ?", ArtifactVersionConfigKey).Scan(&raw); err != nil {
		return 0, fmt.Errorf("读取订阅产物版本号: %w", err)
	}
	return parseArtifactVersion(raw)
}

// parseArtifactVersion 把 config 表里的版本文本转成整数。
func parseArtifactVersion(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("订阅产物版本号 %q 不是整数: %w", raw, err)
	}
	return v, nil
}

// GetSubArtifact 返回令牌的预生成产物；不存在返回 (nil, nil)（缓存未命中是常态，不算错误）。
func (d *DB) GetSubArtifact(ctx context.Context, token string) (*SubArtifact, error) {
	var (
		a      SubArtifact
		genSec int64
	)
	err := d.db.QueryRowContext(ctx,
		"SELECT token, profile_id, version, body, etag, generated_at FROM sub_artifacts WHERE token = ?", token).
		Scan(&a.Token, &a.ProfileID, &a.Version, &a.Body, &a.ETag, &genSec)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.GeneratedAt = time.Unix(genSec, 0).UTC()
	return &a, nil
}

// SaveSubArtifact 按版本守卫写入产物：仅当新版本 >= 已存版本时才落库，
// 旧版本（过期渲染结果）静默丢弃，防止并发旧渲染覆盖新产物。同版本重写视为幂等刷新。
// GeneratedAt 为零值时取当前时间（与 AddFetchAudit 一致）。
func (d *DB) SaveSubArtifact(ctx context.Context, a SubArtifact) error {
	genSec := a.GeneratedAt.Unix()
	if a.GeneratedAt.IsZero() {
		genSec = time.Now().Unix()
	}
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO sub_artifacts (token, profile_id, version, body, etag, generated_at) VALUES (?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT(token) DO UPDATE SET profile_id = excluded.profile_id, version = excluded.version, "+
			"body = excluded.body, etag = excluded.etag, generated_at = excluded.generated_at "+
			"WHERE excluded.version >= sub_artifacts.version",
		a.Token, a.ProfileID, a.Version, a.Body, a.ETag, genSec)
	return err
}

// DeleteSubArtifact 删除令牌的预生成产物；行不存在时同样成功（幂等）。
func (d *DB) DeleteSubArtifact(ctx context.Context, token string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM sub_artifacts WHERE token = ?", token)
	return err
}
