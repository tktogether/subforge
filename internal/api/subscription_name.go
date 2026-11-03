package api

// 订阅名称：令牌名优先，其次方案名，最后服务名。
// 名称会同时进入配置注释头、Content-Disposition 与 profile-title 响应头。

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

func subscriptionDisplayName(tokenName, profileName, serverName string) string {
	for _, raw := range []string{tokenName, profileName, serverName} {
		if name := sanitizeHeaderText(raw); name != "" {
			return name
		}
	}
	return "SubForge"
}

func profileTitleHeader(name string) string {
	clean := sanitizeHeaderText(name)
	if isPrintableASCII(clean) {
		return clean
	}
	return "base64:" + base64.StdEncoding.EncodeToString([]byte(clean))
}

func subscriptionContentDisposition(name string) string {
	safe := sanitizeFileName(name)
	if isPrintableASCII(safe) {
		return fmt.Sprintf(`attachment; filename="%s"`, safe)
	}
	return fmt.Sprintf(`attachment; filename="subforge"; filename*=UTF-8''%s`,
		url.PathEscape(safe))
}

func sanitizeHeaderText(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func sanitizeFileName(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "subforge"
	}
	runes := []rune(out)
	if len(runes) > 80 {
		out = string(runes[:80])
	}
	return out
}

func isPrintableASCII(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}
