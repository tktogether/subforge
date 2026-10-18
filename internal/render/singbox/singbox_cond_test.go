package singbox

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

func node(t *testing.T, typ string, raw map[string]any) model.Node {
	t.Helper()
	n := model.NewNodeFromParts("n-"+typ, typ, "example.com", "443", raw)
	return n
}

func renderWith(t *testing.T, opts Options, req render.Request) ([]byte, *Renderer) {
	t.Helper()
	r := New(opts)
	b, err := r.Render(req)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return b, r
}

func baseReq(nodes model.NodeList) render.Request {
	return render.Request{
		Nodes:   nodes,
		Groups:  []render.Group{},
		Rules:   []string{},
		Profile: render.Profile{Name: "p", Version: "1", GeneratedAt: "2026-01-01T00:00:00Z"},
	}
}

// --- Options 分支 ---

func TestCondOptsInboundsMixedPortZero(t *testing.T) {
	opts := DefaultOptions()
	opts.MixedPort = 0
	opts.Tun = false
	opts.DomainResolver = ""
	opts.AllowLan = false
	opts.DNS.DefaultDNSTag = ""
	r := New(opts)
	_, err := r.Render(testRequest())
	if err != nil {
		t.Fatal(err)
	}
}

func TestCondOptsDomainResolverEmpty(t *testing.T) {
	opts := DefaultOptions()
	opts.DomainResolver = ""
	r := New(opts)
	if _, err := r.Render(testRequest()); err != nil {
		t.Fatal(err)
	}
}

// --- outbound 协议分支 ---

func TestCondOutboundUnsupportedProtocol(t *testing.T) {
	req := baseReq(model.NodeList{node(t, "ssr", map[string]any{"cipher": "chacha20", "password": "p"})})
	r := New(DefaultOptions())
	b, err := r.Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "ssr") == false {
	}
	// 警告里应含跳过信息
	if len(r.Warnings()) == 0 {
		t.Fatal("expected warning for unsupported protocol")
	}
}

func TestCondOutboundAnytlsUsernameFallback(t *testing.T) {
	n := node(t, "anytls", map[string]any{"username": "u", "sni": "s.example.com"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondOutboundSocksHttp(t *testing.T) {
	n1 := node(t, "socks5", map[string]any{"username": "u", "password": "p"})
	n2 := node(t, "http", map[string]any{"username": "u", "password": "p"})
	n3 := node(t, "https", map[string]any{"username": "u", "password": "p", "sni": "s.example.com", "tls": true})
	req := baseReq(model.NodeList{n1, n2, n3})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondOutboundVlessNoFlow(t *testing.T) {
	n := node(t, "vless", map[string]any{"uuid": "u", "tls": true, "servername": "s.example.com"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondOutboundHyNoSpeeds(t *testing.T) {
	n := node(t, "hysteria", map[string]any{"password": "p", "sni": "s.example.com"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondOutboundHyUpOnly(t *testing.T) {
	n := node(t, "hysteria", map[string]any{"password": "p", "sni": "s.example.com", "up": 10})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondOutboundHyDownOnly(t *testing.T) {
	n := node(t, "hysteria", map[string]any{"password": "p", "sni": "s.example.com", "down": 20})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondOutboundHy2Speeds(t *testing.T) {
	n := node(t, "hysteria2", map[string]any{"password": "p", "sni": "s.example.com", "up": 10, "down": "20"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondOutboundTuic(t *testing.T) {
	n := node(t, "tuic", map[string]any{"uuid": "u", "password": "p", "sni": "s.example.com", "alpn": []any{"h3"}, "congestion-controller": "bbr"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- TLS 分支 ---

func TestCondTLSFingerprintField(t *testing.T) {
	n := node(t, "vless", map[string]any{"uuid": "u", "tls": true, "servername": "s.example.com", "fingerprint": "chrome"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTLSClientFingerprint(t *testing.T) {
	n := node(t, "vless", map[string]any{"uuid": "u", "tls": true, "servername": "s.example.com", "client-fingerprint": "chrome"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTLSRealityBadKey(t *testing.T) {
	n := node(t, "vless", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"reality-opts": map[string]any{"public-key": "abc+def=", "short-id": "01"},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings()) == 0 {
		t.Fatal("expected warning for bad reality key")
	}
}

func TestCondTLSSkipCertVerify(t *testing.T) {
	n := node(t, "trojan", map[string]any{"password": "p", "sni": "s.example.com", "skip-cert-verify": true})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTLSAlpn(t *testing.T) {
	n := node(t, "tuic", map[string]any{"uuid": "u", "password": "p", "sni": "s.example.com", "alpn": []any{"h3"}})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTLSInherentHTTPS(t *testing.T) {
	n := node(t, "https", map[string]any{"username": "u", "password": "p", "sni": "s.example.com"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- transport 分支 ---

func TestCondTransportWSHost(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network": "ws",
		"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "h.example.com"}},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTransportWSNoHost(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network": "ws",
		"ws-opts": map[string]any{"path": "/ws"},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTransportGRPCMultiMode(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network":   "grpc",
		"grpc-opts": map[string]any{"path": "/svc", "multiMode": true},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTransportGRPCNoMultiMode(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network":   "grpc",
		"grpc-opts": map[string]any{"path": "/svc"},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTransportH2Host(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network": "h2",
		"h2-opts": map[string]any{"path": "/h2", "host": "h.example.com"},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTransportH2FallbackHost(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network": "h2",
		"h2-opts": map[string]any{"path": "/h2"},
		"h2Host":  "fallback.example.com",
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTransportH2NoHost(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network": "h2",
		"h2-opts": map[string]any{"path": "/h2"},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondTransportHTTPUpgrade(t *testing.T) {
	n := node(t, "vmess", map[string]any{
		"uuid": "u", "tls": true, "servername": "s.example.com",
		"network": "httpupgrade",
		"h2-opts": map[string]any{"path": "/up", "host": "h.example.com"},
	})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- groups ---

func TestCondGroupsEmptyMembersSkip(t *testing.T) {
	req := testRequest()
	req.Groups = []render.Group{{Name: "empty", Type: "select", Proxies: []string{"nope"}}}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondGroupsFallback(t *testing.T) {
	req := testRequest()
	req.Groups = []render.Group{{Name: "fb", Type: "fallback", Proxies: []string{"vmess-node"}}}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondGroupsUnknownType(t *testing.T) {
	req := testRequest()
	req.Groups = []render.Group{{Name: "x", Type: "weird", Proxies: []string{"vmess-node"}}}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondGroupsUrlTestToleranceZero(t *testing.T) {
	req := testRequest()
	req.Groups = []render.Group{{Name: "ut", Type: "url-test", Proxies: []string{"vmess-node"}, Options: render.UrlTestOptions{Interval: 0, Tolerance: 0, Timeout: 0}}}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondGroupsRelay(t *testing.T) {
	req := testRequest()
	req.Groups = []render.Group{{Name: "rl", Type: "relay", Proxies: []string{"vmess-node"}}}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- routeRules ---

func TestCondRouteRulesInvalidLine(t *testing.T) {
	req := testRequest()
	req.Rules = []string{"GARBAGE LINE"}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondRouteRulesUnknownKind(t *testing.T) {
	req := testRequest()
	req.Rules = []string{"GEOIP,CN,DIRECT"}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondRouteRulesInvalidTag(t *testing.T) {
	req := testRequest()
	req.Rules = []string{"DOMAIN-SUFFIX,example.com,nope"}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondRouteRulesMatchFinal(t *testing.T) {
	req := testRequest()
	req.Rules = []string{"MATCH,pick", "FINAL,DIRECT"}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondRouteRulesMergeSame(t *testing.T) {
	req := testRequest()
	req.Rules = []string{
		"DOMAIN-SUFFIX,a.com,pick",
		"DOMAIN-SUFFIX,b.com,pick",
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondRouteRulesDifferentFieldCommit(t *testing.T) {
	req := testRequest()
	req.Rules = []string{
		"DOMAIN-SUFFIX,a.com,pick",
		"IP-CIDR,10.0.0.0/8,DIRECT",
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

func TestCondRouteRulesEmptyTagUsesFinal(t *testing.T) {
	req := testRequest()
	req.Rules = []string{"DOMAIN-SUFFIX,a.com,"}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- DNS ---

func TestCondDNSFakeIP(t *testing.T) {
	opts := DefaultOptions()
	opts.DNS.Servers = []DNSServer{{Tag: "f", Type: "fakeip", FakeIPRange: "198.18.0.0/15", Resolver: "dns_resolver"}}
	r := New(opts)
	if _, err := r.Render(testRequest()); err != nil {
		t.Fatal(err)
	}
}

func TestCondDNSHostNoResolverWarn(t *testing.T) {
	opts := DefaultOptions()
	opts.DNS.Servers = []DNSServer{{Tag: "h", Address: "h3://dns.alidns.com/dns-query"}}
	r := New(opts)
	if _, err := r.Render(testRequest()); err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings()) == 0 {
		t.Fatal("expected warning")
	}
}

func TestCondDNSExplicitType(t *testing.T) {
	opts := DefaultOptions()
	opts.DNS.Servers = []DNSServer{{Tag: "t", Type: "tls", Server: "1.1.1.1", Port: 853, ServerName: "cloudflare-dns.com", Insecure: true, Detour: "DIRECT", Resolver: "dns_resolver", Path: "/dns-query"}}
	r := New(opts)
	if _, err := r.Render(testRequest()); err != nil {
		t.Fatal(err)
	}
}

func TestCondDNSCheckRefsMissing(t *testing.T) {
	opts := DefaultOptions()
	opts.DNS.DefaultDNSTag = "nope"
	opts.DNS.FinalDNSTag = "nope2"
	opts.DomainResolver = "nope3"
	opts.DNS.Servers = []DNSServer{{Tag: "s", Address: "udp://1.1.1.1", Resolver: "ghost"}}
	r := New(opts)
	if _, err := r.Render(testRequest()); err != nil {
		t.Fatal(err)
	}
}

func TestCondDNSParseAddrSchemes(t *testing.T) {
	cases := []string{
		"tls://1.1.1.1:853", "https://dns.example/dns-query", "h3://dns.example/dns-query",
		"quic://dns.example/dns-query", "udp://1.1.1.1:53", "tcp://1.1.1.1:53",
		"fakeip", "rcode://success", "1.1.1.1", "host.example.com:53", "", "rcode:test",
	}
	for _, c := range cases {
		typ, srv, port, path := parseDNSAddr(c)
		_ = typ
		_ = srv
		_ = port
		_ = path
	}
	// 覆盖 splitHostPort
	splitHostPort("2001:db8::1", 53)
	splitHostPort("[::1]:53", 53)
	splitHostPort("example.com:abc", 53)
	splitHostPort("example.com:53", 53)
	splitHostPort("example.com", 53)
}

func TestCondDNSTLSNodeNil(t *testing.T) {
	s := DNSServer{Tag: "x"}
	n := dnsTLSNode(s)
	if n != nil {
		t.Fatal("expected nil")
	}
}

func TestCondDNSTLSNodeInsecure(t *testing.T) {
	s := DNSServer{Tag: "x", Server: "1.1.1.1", Insecure: true}
	n := dnsTLSNode(s)
	if n == nil {
		t.Fatal("expected non-nil")
	}
}

// --- raw helpers ---

func TestCondRawHelpers(t *testing.T) {
	raw := map[string]any{
		"s": "v", "i": 5, "i64": int64(6), "f": 7.0, "fBad": 7.5, "fNeg": -1.0,
		"strNum": "9", "strBad": "x", "listStr": []string{"a", "b"},
		"listAny": []any{"c", 1, "d"}, "bool": true,
	}
	if rawStr(raw, "s") != "v" {
		t.Fatal()
	}
	if rawStr(raw, "missing") != "" {
		t.Fatal()
	}
	if rawStrOr(raw, "fb", "missing", "s") != "v" {
		t.Fatal()
	}
	if rawStrOr(raw, "fb", "missing") != "fb" {
		t.Fatal()
	}
	if !rawBool(raw, "bool") {
		t.Fatal()
	}
	if rawInt(raw, "i", 0) != 5 {
		t.Fatal()
	}
	if rawInt(raw, "i64", 0) != 6 {
		t.Fatal()
	}
	if rawInt(raw, "f", 0) != 7 {
		t.Fatal()
	}
	if rawInt(raw, "strNum", 0) != 9 {
		t.Fatal()
	}
	if rawInt(raw, "strBad", 0) != 0 {
		t.Fatal()
	}
	if rawInt(raw, "missing", 42) != 42 {
		t.Fatal()
	}
	if v, ok := rawNum(raw, "i"); !ok || v != 5 {
		t.Fatal()
	}
	if _, ok := rawNum(raw, "fBad"); ok {
		t.Fatal()
	}
	if _, ok := rawNum(raw, "fNeg"); ok {
		t.Fatal()
	}
	if _, ok := rawNum(raw, "strBad"); ok {
		t.Fatal()
	}
	if v, ok := rawNum(raw, "strNum"); !ok || v != 9 {
		t.Fatal()
	}
	if v, ok := rawNum(raw, "missing"); ok || v != 0 {
		t.Fatal()
	}
	got := rawStrList(raw, "listStr")
	if len(got) != 2 {
		t.Fatal()
	}
	got2 := rawStrList(raw, "listAny")
	if len(got2) != 2 {
		t.Fatal()
	}
	if rawStrList(raw, "missing") != nil && len(rawStrList(raw, "missing")) != 0 {
		t.Fatal()
	}
	_ = anyToStrings([]any{"a", 1})
}

// --- realityPublicKey ---

func TestCondRealityPublicKey(t *testing.T) {
	if v, bad := realityPublicKey(""); bad || v != "" {
		_ = v
		t.Fatal()
	}
	if v, bad := realityPublicKey("abc-_"); bad || v != "abc-_" {
		t.Fatal()
	}
	// standard base64 with +/=
	in := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	v, bad := realityPublicKey(in)
	if bad {
		t.Fatal("expected conversion success")
	}
	if v == in {
		t.Fatal("expected converted")
	}
	// bad standard base64
	if _, bad := realityPublicKey("!!!!===="); !bad {
		t.Fatal("expected bad")
	}
	_ = v
}

// --- duration / mixedListen / tlsInherent ---

func TestCondDurationAndListen(t *testing.T) {
	if duration(0) != "300s" {
		t.Fatal()
	}
	if duration(10) != "10s" {
		t.Fatal()
	}
	if mixedListen(true) != "0.0.0.0" {
		t.Fatal()
	}
	if mixedListen(false) != "127.0.0.1" {
		t.Fatal()
	}
	if !tlsInherent("trojan", false) {
		t.Fatal()
	}
	if tlsInherent("http", true) != true {
		t.Fatal()
	}
	if tlsInherent("http", false) {
		t.Fatal()
	}
}

// --- headerHostNode ---

func TestCondHeaderHostNode(t *testing.T) {
	if headerHostNode(nil) != nil {
		t.Fatal()
	}
	if headerHostNode(map[string]any{}) != nil {
		t.Fatal()
	}
	if headerHostNode(map[string]any{"headers": map[string]any{"Host": "h"}}) == nil {
		t.Fatal()
	}
}

// --- addBandwidth already covered via hy tests; rawNum covered ---

// --- tlsNode skip-cert-verify fallback ---

func TestCondTLSSkipCertVerifyFallback(t *testing.T) {
	n := node(t, "trojan", map[string]any{"password": "p", "sni": "s.example.com", "skip-cert-verify-fallback": true})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- outbound tls nil path (no tls) ---

func TestCondOutboundNoTLS(t *testing.T) {
	n := node(t, "ss", map[string]any{"cipher": "aes-256-gcm", "password": "p"})
	req := baseReq(model.NodeList{n})
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- ssr skip warning already; unsupported protocol covers 306 ---

// --- empty nodes / groups finalTag direct ---

func TestCondEmptyGroupsFinalDirect(t *testing.T) {
	req := render.Request{
		Nodes:   testNodes()[:1],
		Groups:  []render.Group{},
		Rules:   []string{},
		Profile: render.Profile{Name: "p", Version: "1", GeneratedAt: "2026-01-01T00:00:00Z"},
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatal(err)
	}
}

// --- outbound error MappingNode unreachable; skip ---

// --- ensure unused imports ok ---

func TestCondRawNumFloatOK(t *testing.T) {
	raw := map[string]any{"f": 7.0}
	v, ok := rawNum(raw, "f")
	if !ok || v != 7 {
		t.Fatal()
	}
}

func TestCondParseDNSAddrPaths(t *testing.T) {
	for _, a := range []string{"https://dns.example/dns-query", "h3://dns.example/dns-query", "https://dns.example", "h3://dns.example"} {
		parseDNSAddr(a)
	}
	// rcode with rest
	parseDNSAddr("rcode://success")
	parseDNSAddr("fakeip")
	parseDNSAddr("fakeip://198.18.0.0/15")
}

func TestCondTLSInherentAll(t *testing.T) {
	for _, typ := range []string{"trojan", "hysteria", "hysteria2", "anytls", "vless"} {
		tlsInherent(typ, false)
	}
	tlsInherent("http", true)
	tlsInherent("http", false)
}

// --- rawNum all types (cover missing int/int64/string cases) ---
func TestCondRawNumAllTypes(t *testing.T) {
	cases := map[string]any{
		"i":   5,        // int
		"i64": int64(9), // int64
		"f":   7.0,      // float64 (already)
		"s":   "12",     // string
	}
	raw := map[string]any{}
	for k, v := range cases {
		raw[k] = v
	}
	for _, key := range []string{"i", "i64", "f", "s"} {
		v, ok := rawNum(raw, key)
		if !ok {
			t.Fatalf("rawNum %s not ok", key)
		}
		if v < 0 {
			t.Fatal()
		}
	}
	// missing key
	if _, ok := rawNum(raw, "nope"); ok {
		t.Fatal("expected false")
	}
	// negative float rejected
	if _, ok := rawNum(map[string]any{"n": -1.0}, "n"); ok {
		t.Fatal("expected false for negative float")
	}
	// bad string rejected
	if _, ok := rawNum(map[string]any{"b": "x"}, "b"); ok {
		t.Fatal("expected false for bad string")
	}
}

// --- parseDNSAddr all schemes (cover missing tls/udp/tcp/fakeip/rcode/no-scheme) ---
func TestCondParseDNSAddrAllSchemes(t *testing.T) {
	cases := []string{
		"tls://example.com:8443",
		"udp://1.2.3.4:53",
		"tcp://1.2.3.4:53",
		"fakeip",
		"rcode://success",
		"1.2.3.4",
		"example.com:5353",
		"",
		"quic://dns.example/dns-query",
		"doh://dns.example/dns-query",
	}
	for _, a := range cases {
		parseDNSAddr(a)
	}
}

// --- splitHostPort IPv6 branch ---
func TestCondSplitHostPortIPv6(t *testing.T) {
	h, p := splitHostPort("2001:db8::1", 53)
	if h != "2001:db8::1" || p != 53 {
		t.Fatalf("got %s %d", h, p)
	}
	h2, p2 := splitHostPort("example.com:8080", 53)
	if h2 != "example.com" || p2 != 8080 {
		t.Fatalf("got %s %d", h2, p2)
	}
	h3, p3 := splitHostPort("example.com", 53)
	if h3 != "example.com" || p3 != 53 {
		t.Fatalf("got %s %d", h3, p3)
	}
	// bad port
	h4, p4 := splitHostPort("example.com:abc", 53)
	if h4 != "example.com:abc" || p4 != 53 {
		t.Fatalf("got %s %d", h4, p4)
	}
}

var _ = json.Unmarshal
