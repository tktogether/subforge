package fetch

import (
	"os"
	"strings"

	"github.com/tktogether/subforge/internal/buildinfo"
)

// clashCompatToken 是上游机场用来识别「Clash 内核客户端」的标识。
//
// 机场普遍按 User-Agent 决定返回什么内容：识别为 Clash 内核时返回完整 Clash
// 配置（proxies / proxy-groups / rules 齐全）并在响应头带上 Subscription-Userinfo；
// 识别不出时往往只返回分享链接清单。规则与流量信息只存在于前一种响应里，
// 因此默认 UA 以 Clash 内核标识开头，再附加本程序标识便于上游与用户排查。
//
// 实测（2026-10-20，某机场同一订阅地址）：以本程序旧 UA 取回 14,692 字节的
// base64 链接串、无 Subscription-Userinfo 头；以 Clash 内核 UA 取回 528,937
// 字节的完整配置，含 45 个节点、10,465 条规则与上传/下载/总量/到期四个流量字段。
const clashCompatToken = "mihomo/1.19.31"

// fallbackClashUserAgent 是部分机场只认 Clash.Meta 标识时使用的回退 UA。
// 只在返回 Clash YAML 且缺少 Subscription-Userinfo 时尝试一次，
// 避免对正常机场重复请求。
const fallbackClashUserAgent = "clash.meta/1.18.0"

// DefaultFallbackUserAgent 返回缺少流量头时补取完整配置的备用 UA。
func DefaultFallbackUserAgent() string { return fallbackClashUserAgent }

// UserAgentEnv 是覆盖默认 User-Agent 的环境变量名。
//
// 少数上游按其它客户端标识返回不同内容，或用户希望以本程序标识去请求时，
// 设置该变量即可整体替换默认值（设置后不再附加 Clash 内核标识）。
const UserAgentEnv = "SUBFORGE_USER_AGENT"

// DefaultUserAgent 返回默认的请求 User-Agent。
//
// 格式为「Clash 内核标识 + 本程序标识 + 仓库地址」，例如
//
//	mihomo/1.19.31 SubForge/v0.1.6 (+https://github.com/tktogether/subforge)
//
// 若设置了 SUBFORGE_USER_AGENT，则原样使用该值。
func DefaultUserAgent() string {
	if v := strings.TrimSpace(os.Getenv(UserAgentEnv)); v != "" {
		return v
	}
	return clashCompatToken + " SubForge/" + buildinfo.Version + " (+https://github.com/tktogether/subforge)"
}
