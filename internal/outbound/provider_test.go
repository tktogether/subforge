package outbound

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestProviderRoutesThroughNode 直连故意失败，请求仍成功只能说明走了节点出口。
func TestProviderRoutesThroughNode(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "via-node")
	}))
	t.Cleanup(target.Close)
	port := target.Listener.Addr().(*net.TCPAddr).Port
	socksHost, socksPort := startTestSocks5(t)

	direct := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("direct blocked")
	}}
	link := fmt.Sprintf("socks5://%s:%d", socksHost, socksPort)
	p := New(direct, 5*time.Second, func(context.Context, string) (Node, error) {
		return Node{ID: 1, Name: "test", Link: link}, nil
	}, true)
	client, err := p.Client(context.Background(), "")
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	resp, err := client.Get("http://localhost:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatalf("经节点请求: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "via-node" {
		t.Fatalf("响应 = %q", body)
	}
}

// TestProviderDirectWhenNoNode 未配置节点时走直连。
func TestProviderDirectWhenNoNode(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "direct")
	}))
	t.Cleanup(target.Close)

	p := New(&http.Transport{}, 5*time.Second, func(context.Context, string) (Node, error) {
		return Node{}, nil
	}, true)
	client, err := p.Client(context.Background(), "")
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("直连请求: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "direct" {
		t.Fatalf("响应 = %q", body)
	}
}

// TestProviderRoundTrip 直接作为共享客户端 Transport 时走默认出网选择。
func TestProviderRoundTrip(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "default-route")
	}))
	t.Cleanup(target.Close)

	p := New(&http.Transport{}, 5*time.Second, func(context.Context, string) (Node, error) {
		return Node{}, nil
	}, true)
	client := &http.Client{Timeout: 5 * time.Second, Transport: p}
	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "default-route" {
		t.Fatalf("响应 = %q", body)
	}
}

// TestProviderBlocksPrivateLiteral 节点出网不放开字面量私网地址。
func TestProviderBlocksPrivateLiteral(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(target.Close)
	port := target.Listener.Addr().(*net.TCPAddr).Port
	socksHost, socksPort := startTestSocks5(t)

	p := New(&http.Transport{}, 5*time.Second, func(context.Context, string) (Node, error) {
		return Node{ID: 1, Name: "test", Link: fmt.Sprintf("socks5://%s:%d", socksHost, socksPort)}, nil
	}, false)
	client, err := p.Client(context.Background(), "")
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	_, err = client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/")
	if err == nil || !strings.Contains(err.Error(), "私网") {
		t.Fatalf("应拦截字面量私网地址，err=%v", err)
	}
}

// TestProviderPerSourceOutbound 不同来源选择不同出口：节点键走节点，默认/直连走直连。
func TestProviderPerSourceOutbound(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "via-node")
	}))
	t.Cleanup(target.Close)
	port := target.Listener.Addr().(*net.TCPAddr).Port
	socksHost, socksPort := startTestSocks5(t)
	link := fmt.Sprintf("socks5://%s:%d", socksHost, socksPort)

	direct := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("direct blocked")
	}}
	resolve := func(_ context.Context, key string) (Node, error) {
		if key == "1" {
			return Node{ID: 1, Name: "test", Link: link}, nil
		}
		return Node{}, nil
	}
	p := New(direct, 5*time.Second, resolve, true)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		nodeClient, err := p.Client(ctx, "1")
		if err != nil {
			t.Fatalf("Client(1): %v", err)
		}
		resp, err := nodeClient.Get("http://localhost:" + strconv.Itoa(port) + "/")
		if err != nil {
			t.Fatalf("节点出口请求 %d: %v", i, err)
		}
		resp.Body.Close()
	}

	for _, key := range []string{"", "direct"} {
		c, err := p.Client(ctx, key)
		if err != nil {
			t.Fatalf("Client(%q): %v", key, err)
		}
		if _, err := c.Get("http://localhost:" + strconv.Itoa(port) + "/"); err == nil || !strings.Contains(err.Error(), "direct blocked") {
			t.Fatalf("key=%q 应走直连, err=%v", key, err)
		}
	}
}

// TestProviderErrorsAndRebuild 解析失败、取客户端失败，以及同 ID 换链接时重建 Transport。
func TestProviderErrorsAndRebuild(t *testing.T) {
	ctx := context.Background()
	direct := &http.Transport{}
	p := New(direct, time.Second, func(context.Context, string) (Node, error) {
		return Node{}, errors.New("设置不可用")
	}, true)
	if _, err := p.Client(ctx, ""); err == nil {
		t.Fatal("取客户端应报错")
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	if _, err := p.RoundTrip(req); err == nil {
		t.Fatal("RoundTrip 应报错")
	}

	p2 := New(direct, time.Second, func(context.Context, string) (Node, error) {
		return Node{ID: 1, Name: "bad", Link: "not-a-link"}, nil
	}, true)
	if _, err := p2.Client(ctx, "1"); err == nil {
		t.Fatal("非法节点链接应报错")
	}

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "via-node")
	}))
	t.Cleanup(target.Close)
	port := target.Listener.Addr().(*net.TCPAddr).Port
	socksA, portA := startTestSocks5(t)
	socksB, portB := startTestSocks5(t)
	var calls int32
	p3 := New(direct, 5*time.Second, func(context.Context, string) (Node, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return Node{ID: 1, Name: "a", Link: fmt.Sprintf("socks5://%s:%d", socksA, portA)}, nil
		}
		return Node{ID: 1, Name: "b", Link: fmt.Sprintf("socks5://%s:%d", socksB, portB)}, nil
	}, true)
	for i := 0; i < 2; i++ {
		client, err := p3.Client(ctx, "1")
		if err != nil {
			t.Fatalf("Client %d: %v", i, err)
		}
		resp, err := client.Get("http://localhost:" + strconv.Itoa(port) + "/")
		if err != nil {
			t.Fatalf("请求 %d: %v", i, err)
		}
		resp.Body.Close()
	}
}

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
	upstream, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(p)))))
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
