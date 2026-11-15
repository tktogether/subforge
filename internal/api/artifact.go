// 订阅产物预生成（任务 5）：/subscribe 的命中路径、惰性回填与 ETag/304。
//
// 命中判定：默认组合 + 开关开启 + 产物存在 + 产物版本 == 全局版本号。
// ETag 是输入指纹，以「剥离易变行后的 body 哈希」实现（见 computeBodyETag）：
// 同输入重建 etag 不变（304 语义保留），内容带外漂移 etag 变（200 新正文），
// 非默认组合的实时编译响应同样适用。
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/traffic"
)

// volatileGeneratedAtRe 匹配注释头里的秒级生成时间（RFC3339，含「生成时间: 」
// 前缀）。每次渲染现取，同一输入的两次编译跨秒该字面必不同。原为特征测试的
// generatedAtRe（token_fields_test.go），现被内容指纹 ETag 共用，收编为生产代码。
var volatileGeneratedAtRe = regexp.MustCompile("生成时间: \\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:Z|[+-]\\d{2}:\\d{2})")

// volatileUALineRe 匹配 mihomo common/convert 给 ws 分享链接注入的随机浏览器
// User-Agent 整行（headers["User-Agent"] = RandUserAgent()，上游库行为）。该注入
// 发生在 engine.Run 内部的链接解析里，同一输入的两次编译该行天然不同。原为特征
// 测试的 wsUARx（subscribe_query_test.go），现被内容指纹 ETag 共用，收编为生产代码。
var volatileUALineRe = regexp.MustCompile(`(?m)^\s*User-Agent: Mozilla/5\.0.*$`)

// stripVolatileLines 剥除订阅正文里仅有的两类易变行：「生成时间」注释行与
// mihomo 内核注入的随机 User-Agent 行。剥除后同一输入的两次编译逐字节一致，
// 不同输入的差异只会落在易变行之外的确定内容上——这是内容指纹 ETag 的正确性
// 前提（剥离函数只作用于哈希计算，落库与响应的 body 原样保留）。
func stripVolatileLines(body []byte) []byte {
	stripped := volatileUALineRe.ReplaceAll(body, nil)
	return volatileGeneratedAtRe.ReplaceAll(stripped, nil)
}

// computeBodyETag 计算订阅响应的内容指纹 ETag：带引号的 "<hex>"，
// hex = sha256(stripVolatileLines(body))。
//
// 设计书 §3 的「ETag = 输入指纹」以剥离哈希实现：body 剥掉两行易变内容后是全部
// 输入（模板、来源原文、参数、令牌字段）的确定函数——同输入重建 etag 不变，
// If-None-Match/304 语义保留；上游内容带外漂移（手改模板文件、远端模板 URL
// 漂移，均不 bump 版本号）在一个 TTL 内经重建收敛进 body，etag 随之变化，
// 条件客户端拿到 200 新正文而非永远 304。两个不同输入若剥离后相同，说明两份
// 响应本就只差易变行，304 回给客户端的缓存与服务器会产出的正文逐字节等价，
// 语义恒正确。非默认组合的实时编译响应同样成立。
func computeBodyETag(body []byte) string {
	sum := sha256.Sum256(stripVolatileLines(body))
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
// buildSubscription 之前读到的 cur，保存的版本守卫会丢弃并发的过期渲染结果。
// 落库的 ETag 是 computeBodyETag(body)——与响应头同一个内容指纹，落库成败不
// 影响响应带 ETag 的正确性（同输入重编译必得同 strip-hash）。写失败只记日志
// 不阻断响应：缓存只是加速，最迟下个请求重新回填自愈。
func (s *Server) backfillSubArtifact(ctx context.Context, m *subforge.Manager, token string, cur int64, body []byte) {
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
		ETag:      computeBodyETag(body),
	}
	if err := m.SaveSubArtifact(ctx, art); err != nil {
		s.logger.Printf("[subscribe] 回填订阅产物失败（下个请求会重新回填）: %v", err)
	}
}
