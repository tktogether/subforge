// 订阅产物预生成（任务 5）：/subscribe 的命中路径、惰性回填与 ETag/304。
//
// 命中判定：默认组合 + 开关开启 + 产物存在 + 产物版本 == 全局版本号。
// ETag 只由（版本号, 令牌）派生：版本号已捕获全部正文输入（bump 清单 + 内容变化
// 才 bump + 失败 bump），不同 query 组合按 URL 各自缓存，跨组合不会串。
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/traffic"
)

// computeArtifactETag 计算预生成产物的 ETag：带引号的 "<hex>"，
// hex = sha256(version + "\n" + token)。
//
// 版本号捕获全部正文输入：DB 写路径统一 bump、上游内容变化才 bump、必失败源失败
// 也 bump，因此（版本, 令牌）对确定时产物 body 确定；令牌维度让不同 URL 的客户端
// 缓存互不串扰。etagMatch 一起构成 If-None-Match/304 的服务端语义。
func computeArtifactETag(version int64, token string) string {
	sum := sha256.Sum256([]byte(strconv.FormatInt(version, 10) + "\n" + token))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// etagMatch 报告 If-None-Match 是否命中 etag：值为逗号分隔的实体标签列表
// （容忍空白），"*" 匹配任意标签；空值视为未携带，返回 false。
func etagMatch(ifNoneMatch, etag string) bool {
	if strings.TrimSpace(ifNoneMatch) == "" {
		return false
	}
	for _, part := range strings.Split(ifNoneMatch, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || part == etag {
			return true
		}
	}
	return false
}

// trafficHeaderValue 聚合本次参与构建机场的流量余额，返回 Subscription-Userinfo
// 响应头值；无可聚合数据或策略为不透传（全 none）时返回空串，调用方不设头。
// 原 handleSubscribe 的流量聚合段原样搬入：实时路径与产物命中路径共用，
// 保证两条路径的响应头完全一致。
func (s *Server) trafficHeaderValue(ctx context.Context, usedAirports []subforge.Airport) string {
	// 读策略失败（DB 故障）时静默跳过——订阅本身仍然要能生成。
	strat, err := s.subManager.GetTrafficStrategy(ctx)
	if err != nil {
		return ""
	}
	// 只统计「已成功采集过」的机场（TrafficAt > 0）：
	// 从未采集成功的机场流量是零值，混进来会把 min/max/avg 拉偏。
	usages := make([]traffic.Usage, 0, len(usedAirports))
	nodeCounts := make([]int, 0, len(usedAirports))
	for _, a := range usedAirports {
		if a.TrafficAt <= 0 {
			continue
		}
		usages = append(usages, traffic.Usage{
			Upload: a.Upload, Download: a.Download, Total: a.Total, Expire: a.Expire,
		})
		nodeCounts = append(nodeCounts, a.NodeCount)
	}
	// Aggregate 在「无可聚合数据」或「策略为不透传（全 none）」时返回 ok=false。
	u, ok := traffic.Aggregate(usages, nodeCounts, strat)
	if !ok {
		return ""
	}
	return traffic.FormatWithStrategy(u, strat)
}

// requiredFailedSources 返回设为「失败时报错」且本次拉取失败的机场描述列表；
// 非空时 /subscribe 整份订阅回 502，客户端保留上一版可用配置。
// 原 handleSubscribe 的 requiredFailed 循环原样搬入，产物回填路径也用它判定
// 「本次结果是否 200 等价」（有必失败源失败就绝不回填）。
func requiredFailedSources(usedAirports []subforge.Airport, result convert.Result) []string {
	var requiredFailed []string
	for _, a := range usedAirports {
		if a.FailurePolicy != subforge.AirportFailureError {
			continue
		}
		if msg, failed := result.SourceErrors[subforge.AirportSourceKey(a.ID)]; failed {
			requiredFailed = append(requiredFailed, fmt.Sprintf("%s（%s）", a.Name, msg))
		}
	}
	return requiredFailed
}

// setSubscribeHeaders 写 /subscribe 的公共响应头：流量余额透传、展示名与下载
// 文件名。实时路径与产物命中路径共用，保证两条路径的响应头完全一致。
func (s *Server) setSubscribeHeaders(w http.ResponseWriter, ctx context.Context, b *subBuild) {
	if header := s.trafficHeaderValue(ctx, b.UsedAirports); header != "" {
		w.Header().Set("Subscription-Userinfo", header)
	}
	w.Header().Set("profile-title", profileTitleHeader(b.Name))
	w.Header().Set("Content-Disposition", subscriptionContentDisposition(b.Name))
}

// backfillSubArtifact 惰性回填：把本次实时编译的原始 body（含时间戳行）按当前
// 版本号 cur 落 sub_artifacts 表，供后续同版本拉取直接命中。只应在 200 等价结果
// （engine.Run 成功且 requiredFailedSources 为空）且默认组合时调用；Version 取
// engine.Run 之前读到的 cur，保存的版本守卫会丢弃并发的过期渲染结果。
// 写失败只记日志不阻断响应：缓存只是加速，最迟下个请求重新回填自愈。
//
// 返回是否落库成功（调用方据此决定是否给响应带 ETag——保存失败时下一次拉取
// 会重新编译，body 不保证逐字节一致，不能让客户端按该 ETag 缓存）。
func (s *Server) backfillSubArtifact(ctx context.Context, m *subforge.Manager, token string, cur int64, b *subBuild, body []byte) bool {
	// ProfileID 取令牌绑定的方案：buildSubscription 不透出方案内部状态，
	// 这里按令牌补读一次（读失败视为未绑定，产物照存，只丢归属信息）。
	var profileID int64
	if subTok, err := m.GetSubToken(ctx, token); err == nil {
		profileID = subTok.ProfileID
	}
	art := store.SubArtifact{
		Token:     token,
		ProfileID: profileID,
		Version:   cur,
		Body:      body,
		ETag:      computeArtifactETag(cur, token),
	}
	if err := m.SaveSubArtifact(ctx, art); err != nil {
		s.logger.Printf("[subscribe] 回填订阅产物失败（下个请求会重新回填）: %v", err)
		return false
	}
	return true
}
