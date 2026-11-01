package parse

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// TestNodeDialerSocks5 用本地 SOCKS5 服务器验证「节点拨号」端到端可用：
// 目标用 localhost 域名，确认域名是原样交给节点（远端解析），而不是本地先解析。
func TestNodeDialerSocks5(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "through-node")
	}))
	t.Cleanup(target.Close)
	port := target.Listener.Addr().(*net.TCPAddr).Port

	host, socksPort := startTestSocks5(t)
	d, err := NewNodeDialer(map[string]any{
		"type": "socks5", "name": "test-node", "server": host, "port": socksPort,
	})
	if err != nil {
		t.Fatalf("NewNodeDialer: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{DialContext: d.DialContext},
	}
	resp, err := client.Get("http://localhost:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatalf("经节点请求: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应: %v", err)
	}
	if string(body) != "through-node" {
		t.Fatalf("响应 = %q", body)
	}
}

// TestNodeMapping 单条分享链接转节点 map；非法链接报错。
func TestNodeMapping(t *testing.T) {
	m, err := NodeMapping("ss://YWVzLTI1Ni1nY206cGFzcw@1.2.3.4:8388#n1")
	if err != nil {
		t.Fatalf("NodeMapping: %v", err)
	}
	if m["type"] != "ss" {
		t.Fatalf("type = %v", m["type"])
	}
	if _, err := NodeMapping("not-a-link"); err == nil {
		t.Fatal("非法链接应报错")
	}
}

// TestNodeDialerRejects 非 TCP、非法地址、非法端口与空拨号器的分支。
func TestNodeDialerRejects(t *testing.T) {
	d, err := NewNodeDialer(map[string]any{"type": "socks5", "name": "n", "server": "127.0.0.1", "port": 1})
	if err != nil {
		t.Fatalf("NewNodeDialer: %v", err)
	}
	defer func() { _ = d.Close() }()
	ctx := context.Background()
	if _, err := d.DialContext(ctx, "udp", "1.2.3.4:53"); err == nil {
		t.Fatal("UDP 应被拒绝")
	}
	if _, err := d.DialContext(ctx, "tcp", "bad-addr"); err == nil {
		t.Fatal("非法地址应报错")
	}
	if _, err := d.DialContext(ctx, "tcp", "1.2.3.4:not-a-port"); err == nil {
		t.Fatal("非法端口应报错")
	}
	var nilDialer *NodeDialer
	if err := nilDialer.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
	if err := (&NodeDialer{}).Close(); err != nil {
		t.Fatalf("空拨号器 Close: %v", err)
	}
}

// startTestSocks5 起一个只支持无认证 CONNECT 的最小 SOCKS5 服务器。
func startTestSocks5(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveTestSocks5(conn)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port
}

func serveTestSocks5(c net.Conn) {
	defer c.Close()
	head := make([]byte, 2)
	if _, err := io.ReadFull(c, head); err != nil || head[0] != 5 {
		return
	}
	if _, err := io.ReadFull(c, make([]byte, int(head[1]))); err != nil {
		return
	}
	if _, err := c.Write([]byte{5, 0}); err != nil {
		return
	}

	req := make([]byte, 4)
	if _, err := io.ReadFull(c, req); err != nil || req[0] != 5 || req[1] != 1 {
		return
	}
	var host string
	switch req[3] {
	case 1:
		b := make([]byte, 4)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case 3:
		l := make([]byte, 1)
		if _, err := io.ReadFull(c, l); err != nil {
			return
		}
		b := make([]byte, int(l[0]))
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = string(b)
	case 4:
		b := make([]byte, 16)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	default:
		return
	}
	p := make([]byte, 2)
	if _, err := io.ReadFull(c, p); err != nil {
		return
	}
	target := net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(p))))
	upstream, err := net.Dial("tcp", target)
	if err != nil {
		_, _ = c.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()
	if _, err := c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	done := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(upstream, c)
		done <- struct{}{}
	}()
	_, _ = io.Copy(c, upstream)
	<-done
}
