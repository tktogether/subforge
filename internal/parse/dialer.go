package parse

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	mihomoadapter "github.com/metacubex/mihomo/adapter"
	C "github.com/metacubex/mihomo/constant"
)

// NodeMapping 把单条分享链接转换成 Clash 节点 map（出网节点用）。
func NodeMapping(link string) (map[string]any, error) {
	nodes, err := ConvertSubscription([]byte(strings.TrimSpace(link)))
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, ErrEmptySubscription
	}
	return nodes[0], nil
}

// NodeDialer 用内核 outbound 适配器直接拨号，让服务端自身的出站请求
// （上游订阅、规则集、模板）复用控制台里已配置的节点，而不依赖外部代理。
//
// 它只把一个目标 host:port 交给节点协议，不启动监听端口、不跑规则与 DNS 模块。
type NodeDialer struct {
	proxy C.Proxy
}

// NewNodeDialer 从 Clash 节点 map 构造拨号器；节点非法时返回内核的校验错误。
func NewNodeDialer(mapping map[string]any) (*NodeDialer, error) {
	proxy, err := mihomoadapter.ParseProxy(mapping)
	if err != nil {
		return nil, fmt.Errorf("节点 %q: %w", NameOf(mapping), err)
	}
	return &NodeDialer{proxy: proxy}, nil
}

// DialContext 与 net.Dialer 同签名，可直接挂到 http.Transport。
// 域名原样交给节点远端解析：本地 DNS 的结果在这里没有意义，
// 节点协议需要域名才能走远端解析（也避免本地 DNS 污染影响拉取）。
func (d *NodeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("节点出网仅支持 TCP，收到 %q", network)
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("目标端口 %q 不合法", portText)
	}
	return d.proxy.DialContext(ctx, &C.Metadata{
		NetWork: C.TCP,
		Type:    C.HTTP,
		Host:    host,
		DstPort: uint16(port),
	})
}

// Close 释放适配器持有的资源（连接池、定时器等）。
func (d *NodeDialer) Close() error {
	if d == nil || d.proxy == nil {
		return nil
	}
	return d.proxy.Close()
}
