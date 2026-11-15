// 订阅产物 Manager 门面（订阅产物预生成 任务 3）。
//
// API 层与后续的订阅渲染不直接摸 store：版本号、产物读写一律从 Manager 走，
// 这里同时承载「开关判断」（ArtifactsEnabled）与 store 够不着的文件路径 bump 点——
// 规则源版本文件、模板/override 落盘都不经过 DB 写路径，touchArtifacts 接不到，
// 由 Manager 在落盘成功后显式 bump 一次让预生成产物失效。
package subforge

import (
	"context"
	"log"
	"strings"

	"github.com/tktogether/subforge/internal/store"
)

// ArtifactModeKey 是订阅产物缓存的开关设置：未配置 = 默认开。
const ArtifactModeKey = "subscribe.artifact_mode"

// --- Manager 门面：透传 store 原语 ---

// ArtifactVersion 返回订阅产物全局版本号（透传 store；从未 bump 返回 0）。
func (m *Manager) ArtifactVersion(ctx context.Context) (int64, error) {
	return m.db.ArtifactVersion(ctx)
}

// BumpArtifactVersion 把订阅产物全局版本号 +1（透传 store，原子操作）。
func (m *Manager) BumpArtifactVersion(ctx context.Context) error {
	_, err := m.db.BumpArtifactVersion(ctx)
	return err
}

// GetSubArtifact 返回令牌的预生成产物（透传 store）；不存在返回 (nil, nil)——
// 缓存未命中是常态，不算错误。
func (m *Manager) GetSubArtifact(ctx context.Context, token string) (*store.SubArtifact, error) {
	return m.db.GetSubArtifact(ctx, token)
}

// SaveSubArtifact 按版本守卫写入预生成产物（透传 store）。
func (m *Manager) SaveSubArtifact(ctx context.Context, a store.SubArtifact) error {
	return m.db.SaveSubArtifact(ctx, a)
}

// DeleteSubArtifact 删除令牌的预生成产物（透传 store；行不存在同样成功）。
func (m *Manager) DeleteSubArtifact(ctx context.Context, token string) error {
	return m.db.DeleteSubArtifact(ctx, token)
}

// ArtifactsEnabled 报告订阅产物预生成是否启用：
// subscribe.artifact_mode 开（未配置 = 默认开）且 SourceRefreshInterval > 0。
//
// 读配置出错一律按禁用处理：产物只是缓存，开关判断宁缺毋滥——
// 误判关闭最坏是多编译几次，误判开启则会拿过期产物回包。
func (m *Manager) ArtifactsEnabled(ctx context.Context) bool {
	v, err := m.GetConfig(ctx, ArtifactModeKey)
	if err != nil {
		return false
	}
	// 未配置 = 默认开；已配置的值沿用 settings.go 的 bool 解析口径
	// （写入侧会归一成 true/false，脏值按禁用处理）。
	if s := strings.TrimSpace(v); s != "" {
		nb, nerr := normalizeBoolForList(s)
		if nerr != nil || nb != "true" {
			return false
		}
	}
	// 刷新间隔为 0 表示缓存整体关闭，产物依赖的「间隔内不变」语义不存在，一并禁用。
	refresh, err := m.SourceRefreshInterval(ctx)
	if err != nil {
		return false
	}
	return refresh > 0
}

// bumpArtifactsAfterFileWrite 是 store 外（文件/落盘路径）的显式 bump 点。
//
// 失败只记日志不阻断：漏一次 bump 最迟一个 TTL 内由惰性重建自愈，
// 绝不能让 bump 失败连累落盘操作本身。
func (m *Manager) bumpArtifactsAfterFileWrite(ctx context.Context) {
	if _, err := m.db.BumpArtifactVersion(ctx); err != nil {
		log.Printf("[subforge] bump 订阅产物版本号失败（惰性重建会自愈）: %v", err)
	}
}
