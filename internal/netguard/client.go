// Package netguard 提供带出站地址校验的 HTTP 客户端。
//
// 订阅、机场、规则源、远程模板都由服务端代用户发起 GET；如果没有限制，
// 一个持令牌的用户可以让服务端访问回环、内网或云元数据地址。默认拒绝这些
// 地址，只允许公网；确实需要访问本机/LAN 的部署显式开启（serve -allow-private-network）。
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewHTTPClient 构造一个默认拒绝私网/回环/链路本地地址的 HTTP 客户端。
// allowPrivate=true 时退回普通客户端（本地 fixture、内网订阅等显式场景）。
func NewHTTPClient(timeout time.Duration, allowPrivate bool) *http.Client {
	return &http.Client{Timeout: timeout, Transport: NewHTTPTransport(allowPrivate)}
}

// NewHTTPTransport 构造带出站地址校验的 Transport。
// 单独暴露给需要自定义 RoundTripper 的调用方（例如按请求在直连与节点出口之间切换）。
func NewHTTPTransport(allowPrivate bool) *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          32,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	if allowPrivate {
		transport.DialContext = dialer.DialContext
		return transport
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		// 任意解析结果落在私网就拒绝：避免 DNS 提供公网+内网混合记录绕过。
		for _, ip := range ips {
			if IsBlockedIP(ip.IP) {
				return nil, fmt.Errorf("拒绝访问私网/回环地址 %s", ip.IP)
			}
		}
		var lastErr error
		for _, ip := range ips {
			conn, derr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if derr == nil {
				return conn, nil
			}
			lastErr = derr
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("域名 %s 没有可用的解析结果", host)
		}
		return nil, lastErr
	}
	return transport
}

// IsBlockedIP 报告地址是否属于禁止出站的私网/回环/链路本地/未指定/组播范围。
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// 云元数据地址已被 LinkLocalUnicast 覆盖（169.254.0.0/16），这里再兜底一次。
	if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
		return true
	}
	// CGNAT 100.64.0.0/10 常被当作内网使用。
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

// ValidateHTTPURL 只允许 http/https 且带 host 的 URL。
func ValidateHTTPURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("URL 为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL 不合法: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https URL")
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("URL 缺少主机名")
	}
	return nil
}
