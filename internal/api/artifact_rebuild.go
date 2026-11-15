// 订阅产物后台重建器（订阅产物预生成 任务 7）：常驻轮询 goroutine 让每个令牌的
// 预生成产物保持新鲜，/subscribe 拉取直接回产物、永远不必等一次编译。
//
// 与 Task 5 的惰性回填互补：回填只在客户端拉取时顺带发生（冷令牌要等一个请求），
// 重建器在后台按 artifactPoll 轮询，把「产物缺失 / 版本落后 / 年龄超限」的令牌
// 主动补齐。单轮逻辑 rebuildArtifacts 拆成独立方法，测试可以同步直调。
package api

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/subforge"
)

// defaultArtifactPoll 是后台重建器的默认轮询间隔（artifactPoll 零值时的兜底）。
const defaultArtifactPoll = 5 * time.Second

// StartArtifactRebuilder 启动订阅产物后台重建轮询。
//
// subManager 为 nil（未启用订阅管理）时是空操作：不 spawn、不报错——
// serve.go 在 -data-dir 未配置时同样可以无脑调用。
// ctx 取 serve 生命周期，取消后轮询 goroutine 退出。
// 启动即跑一轮（预热），之后每 artifactPoll（零值按 defaultArtifactPoll）跑一轮。
func (s *Server) StartArtifactRebuilder(ctx context.Context) {
	if s.subManager == nil {
		return
	}
	poll := s.artifactPoll
	if poll <= 0 {
		poll = defaultArtifactPoll
	}
	go func() {
		// 预热：不等第一个 tick，服务一拉起产物就已就绪（rebuildArtifacts
		// 开头检查 ctx，已取消时直接跳过）。
		s.rebuildArtifacts(ctx)
		ticker := time.NewTicker(poll)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.rebuildArtifacts(ctx)
			}
		}
	}()
}

// rebuildArtifacts 执行一轮产物重建（StartArtifactRebuilder 的循环体，也可从测试
// 同步直调）：开关关闭则整轮跳过；否则读当前版本号，逐令牌检查——绑定停用方案的
// 令牌跳过；产物缺失、版本 != 当前版本、或年龄超过 SourceRefreshInterval 的令牌
// 执行 rebuildOneToken。
//
// 并发不变量：本方法可能与拉取路径的惰性回填（handleSubscribe → backfillSubArtifact）
// 并发执行，未来也可能与 Task 8 的刷新任务联动触发——无需 single-flight：
// SaveSubArtifact 的版本守卫（excluded.version >= sub_artifacts.version）保证旧版本
// 渲染结果不会覆盖新产物，同版本重写幂等；两个入口都按「engine.Run 之前读到的
// cur」落库，竞态的最坏结果是同版本互相覆盖一次等价 body。
func (s *Server) rebuildArtifacts(ctx context.Context) {
	if s.subManager == nil || ctx.Err() != nil {
		return
	}
	m := s.subManager
	// 开关关闭（artifact_mode=0 或刷新间隔为 0）→ 整轮跳过，读配置失败也按禁用。
	if !m.ArtifactsEnabled(ctx) {
		return
	}
	cur, err := m.ArtifactVersion(ctx)
	if err != nil {
		s.logger.Printf("[subscribe] 后台重建读产物版本号失败，本轮跳过: %v", err)
		return
	}
	tokens, err := m.ListSubTokens(ctx)
	if err != nil {
		s.logger.Printf("[subscribe] 后台重建列令牌失败，本轮跳过: %v", err)
		return
	}
	// 年龄阈值与命中语义同源：产物依赖「间隔内上游不变」，超龄即视为过期。
	refresh, err := m.SourceRefreshInterval(ctx)
	if err != nil || refresh <= 0 {
		// ArtifactsEnabled 已保证 refresh > 0；防御性兜底按「不做年龄判断」处理。
		refresh = 0
	}
	for _, t := range tokens {
		if ctx.Err() != nil {
			return
		}
		// 绑定停用方案的令牌跳过：buildSubscription 会 400，重建只会白跑并误删产物。
		if t.ProfileID > 0 {
			p, perr := m.GetProfile(ctx, t.ProfileID)
			if perr != nil || !p.Enabled {
				s.logger.Printf("[subscribe] 令牌 “%s” 绑定的方案不可用，本轮跳过", dash(t.Name))
				continue
			}
		}
		art, aerr := m.GetSubArtifact(ctx, t.Token)
		if aerr != nil {
			s.logger.Printf("[subscribe] 后台重建读令牌 “%s” 产物失败，本轮跳过: %v", dash(t.Name), aerr)
			continue
		}
		stale := art == nil || art.Version != cur
		if !stale && refresh > 0 && time.Since(art.GeneratedAt) > refresh {
			stale = true
		}
		if !stale {
			continue
		}
		s.rebuildOneToken(ctx, cur, t)
	}
}

// rebuildOneToken 重建单个令牌的预生成产物。与实时路径共用 buildSubscription 的
// 构建前段（空 query = 默认组合，产物本来就只为默认组合预生成），引擎执行用
// 独立预算的 context（SubscribeTimeout），失败（构建 status != 0 / engine.Run 报错 /
// requiredFailedSources 非空）一律删掉旧产物并记日志，成功才按本轮 cur 落库。
func (s *Server) rebuildOneToken(ctx context.Context, cur int64, subTok subforge.SubToken) {
	m := s.subManager
	token := subTok.Token
	// 构建前段：与 /subscribe 同一条逻辑（调用方已保证 subManager 非 nil）。
	// cleanup 关闭自建节点临时目录，临时文件要活到引擎跑完，先于任何返回 defer。
	b, cleanup, status, err := s.buildSubscription(ctx, token, url.Values{})
	defer cleanup()
	if status != 0 {
		s.logger.Printf("[subscribe] 后台重建令牌 “%s” 构建失败（status=%d）: %v，删除旧产物",
			dash(subTok.Name), status, err)
		if derr := m.DeleteSubArtifact(ctx, token); derr != nil {
			s.logger.Printf("[subscribe] 删除令牌 “%s” 旧产物失败: %v", dash(subTok.Name), derr)
		}
		return
	}

	// 拉取阶段单独限时（与实时路径同一预算口径）：死源不能把整轮重建拖住。
	fetchCtx, cancel := context.WithTimeout(ctx, s.cfg.SubscribeTimeout)
	defer cancel()
	result, rerr := s.engine.Run(fetchCtx, b.Params)
	if rerr != nil {
		s.logger.Printf("[subscribe] 后台重建令牌 “%s” 引擎执行失败: %v，删除旧产物", dash(subTok.Name), rerr)
		if derr := m.DeleteSubArtifact(ctx, token); derr != nil {
			s.logger.Printf("[subscribe] 删除令牌 “%s” 旧产物失败: %v", dash(subTok.Name), derr)
		}
		return
	}
	// 机场设为「失败时报错」时本次结果不是 200 等价：删旧产物，与实时路径的
	// 回填前置条件一致（非 200 等价结果绝不落产物）。
	if requiredFailed := requiredFailedSources(b.UsedAirports, result); len(requiredFailed) > 0 {
		s.logger.Printf("[subscribe] 后台重建令牌 “%s” 必失败源失败(%d): %s，删除旧产物",
			dash(subTok.Name), len(requiredFailed), strings.Join(requiredFailed, "；"))
		if derr := m.DeleteSubArtifact(ctx, token); derr != nil {
			s.logger.Printf("[subscribe] 删除令牌 “%s” 旧产物失败: %v", dash(subTok.Name), derr)
		}
		return
	}

	art := store.SubArtifact{
		Token:     token,
		ProfileID: subTok.ProfileID,
		Version:   cur,
		Body:      result.Body,
		ETag:      computeBodyETag(result.Body),
	}
	if err := m.SaveSubArtifact(ctx, art); err != nil {
		s.logger.Printf("[subscribe] 后台重建保存令牌 “%s” 产物失败: %v", dash(subTok.Name), err)
	}
}
