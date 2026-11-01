// Package outbound 决定服务端出站请求走直连还是某个自建节点。
//
// 提供两种接法：
//   - Provider 本身是 http.RoundTripper，挂在共享 http.Client 上（规则集、模板）；
//   - Client(ctx, outbound) 按来源的出网选择返回客户端（机场、规则源拉取）。
//
// 设置改了不需要重启：每次请求都重新解析当前选择。
package outbound

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tktogether/subforge/internal/netguard"
	"github.com/tktogether/subforge/internal/parse"
)

// Node 是解析后的出网节点；Link 为空表示直连。
type Node struct {
	ID   int64
	Name string
	Link string
}

// Resolver 把来源的出网选择（空=默认，direct=直连，数字=节点 ID）解析成节点。
type Resolver func(ctx context.Context, outbound string) (Node, error)

// Provider 缓存节点 Transport，并按选择分派请求。
type Provider struct {
	direct       *http.Transport
	timeout      time.Duration
	resolve      Resolver
	allowPrivate bool

	mu    sync.Mutex
	cache map[int64]*cachedNode
}

type cachedNode struct {
	key    string
	tr     *http.Transport
	dialer *parse.NodeDialer
}

// New 构造 Provider。direct 是直连出口（通常由 netguard 构造，带私网拦截）。
func New(direct *http.Transport, timeout time.Duration, resolve Resolver, allowPrivate bool) *Provider {
	return &Provider{
		direct: direct, timeout: timeout, resolve: resolve, allowPrivate: allowPrivate,
		cache: map[int64]*cachedNode{},
	}
}

// RoundTrip 让 Provider 直接充当共享客户端的 Transport，使用默认出网选择。
func (p *Provider) RoundTrip(req *http.Request) (*http.Response, error) {
	tr, err := p.transport(req.Context(), "")
	if err != nil {
		return nil, err
	}
	return tr.RoundTrip(req)
}

// Client 返回按来源出网选择分派请求的客户端。
// outbound 为空=默认，direct=直连，数字=自建节点 ID；解析失败或节点失效由 Resolver 决定。
func (p *Provider) Client(ctx context.Context, outbound string) (*http.Client, error) {
	tr, err := p.transport(ctx, outbound)
	if err != nil {
		return nil, err
	}
	return &http.Client{Timeout: p.timeout, Transport: tr}, nil
}

func (p *Provider) transport(ctx context.Context, outbound string) (*http.Transport, error) {
	node, err := p.resolve(ctx, outbound)
	if err != nil {
		return nil, fmt.Errorf("读取出网节点设置: %w", err)
	}
	if strings.TrimSpace(node.Link) == "" {
		return p.direct, nil
	}
	key := fmt.Sprintf("%d:%s", node.ID, node.Link)

	p.mu.Lock()
	defer p.mu.Unlock()
	if c := p.cache[node.ID]; c != nil && c.key == key {
		return c.tr, nil
	}

	mapping, err := parse.NodeMapping(node.Link)
	if err != nil {
		return nil, fmt.Errorf("出网节点 %q: %w", node.Name, err)
	}
	dialer, err := parse.NewNodeDialer(mapping)
	if err != nil {
		return nil, fmt.Errorf("出网节点 %q: %w", node.Name, err)
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 节点远端解析域名，本地只能拦字面量私网地址；域名解析结果由节点侧负责。
			if !p.allowPrivate {
				if host, _, splitErr := net.SplitHostPort(addr); splitErr == nil {
					if ip := net.ParseIP(host); ip != nil && netguard.IsBlockedIP(ip) {
						return nil, fmt.Errorf("拒绝经节点访问私网/回环地址 %s", host)
					}
				}
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:        8,
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if old := p.cache[node.ID]; old != nil {
		old.tr.CloseIdleConnections()
		_ = old.dialer.Close()
	}
	p.cache[node.ID] = &cachedNode{key: key, tr: tr, dialer: dialer}
	return tr, nil
}
