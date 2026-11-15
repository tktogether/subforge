// artifact_test.go — 订阅产物预生成（任务 5）：/subscribe 命中路径、惰性回填与
// ETag/304 的端到端行为测试。全部走 httptest + 临时 SQLite（uiSubServer 装配），
// 六个场景对应任务简报；命中路径回的是首次实时编译的原始字节（含时间戳行与
// mihomo 内核注入的随机 User-Agent 行，随首次编译一起定格），因此 live-vs-hit
// 逐字节比较即可，无需归一化。版本号断言一律用相对值（+1），不写绝对数。
//
// ETag 断言一律锚定内容指纹公式：computeBodyETag(body) = 剥离易变行后的 body
// sha256（白盒复用生产函数，不重造公式）。带外漂移回归（TestSubscribeArtifact-
// OutOfBandDrift）证明 ETag 随内容变化而非版本号。
package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/traffic"
)

// artifactSetup 起一个带机场（假上游 fixture）与订阅令牌的服务。
// 模式与刷新间隔都走默认（产物预生成默认开启：artifact_mode 未配置=开、刷新间隔默认 30 分钟）。
func artifactSetup(t *testing.T) (*httptest.Server, *subforge.Manager) {
	t.Helper()
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	return ts, mgr
}

// artifactGet 发一个可带 If-None-Match 的 GET，返回状态码、响应头与正文。
// ifNoneMatch 为空串时不设该头。
func artifactGet(t *testing.T, ts *httptest.Server, path, ifNoneMatch string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("构造请求: %v", err)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应: %v", err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

// TestStripVolatileLines 单元：只剥两类易变行——「生成时间」注释行与 mihomo
// 内核给 ws 链接注入的随机 User-Agent 行——其余内容原样保留；两个只差易变行的
// body 剥离后逐字节一致（这正是内容指纹 ETag 的正确性前提），不含易变行的输入
// 原样返回。
func TestStripVolatileLines(t *testing.T) {
	base := "# 此配置由 SubForge 自动生成\n" +
		"# 生成时间: 2026-11-15T10:00:00Z\n" +
		"proxies:\n  - name: vless-ws\n    ws-opts:\n      headers:\n" +
		"        User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36\n"
	other := "# 此配置由 SubForge 自动生成\n" +
		"# 生成时间: 2026-11-15T23:59:59+08:00\n" +
		"proxies:\n  - name: vless-ws\n    ws-opts:\n      headers:\n" +
		"        User-Agent: Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0\n"
	s1, s2 := stripVolatileLines([]byte(base)), stripVolatileLines([]byte(other))
	if !bytes.Equal(s1, s2) {
		t.Fatalf("只差易变行的两个 body 剥离后应逐字节一致:\n%q\nvs\n%q", s1, s2)
	}
	for _, want := range []string{"# 此配置由 SubForge 自动生成", "proxies:", "vless-ws", "headers:"} {
		if !strings.Contains(string(s1), want) {
			t.Fatalf("剥离后不应丢失非易变内容 %q:\n%s", want, s1)
		}
	}
	if strings.Contains(string(s1), "生成时间") || strings.Contains(string(s1), "Mozilla") {
		t.Fatalf("易变内容应被剥除:\n%s", s1)
	}
	plain := []byte("proxies:\n  - name: ss-node\n")
	if got := stripVolatileLines(plain); !bytes.Equal(got, plain) {
		t.Fatalf("无易变行的 body 应原样保留，got %q", got)
	}
}

// TestSubscribeArtifactHit 默认组合拉取两次：第一次实时编译并回填产物（响应带
// 内容指纹 ETag = computeBodyETag(body)，带引号）；第二次命中——直接回产物的
// 原始字节，因此两次 body 逐字节一致（无需剥离「生成时间」行）。
func TestSubscribeArtifactHit(t *testing.T) {
	ts, mgr := artifactSetup(t)
	ctx := context.Background()

	st1, hdr1, body1 := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if st1 != http.StatusOK {
		t.Fatalf("第一次拉取 status=%d body=%s", st1, body1)
	}
	if body1 == "" {
		t.Fatalf("第一次拉取得到空响应体")
	}
	etag1 := hdr1.Get("ETag")
	if etag1 == "" {
		t.Fatalf("第一次拉取（回填）应带 ETag 头")
	}
	// ETag 公式锚定（白盒）：剥离易变行后的 body sha256，整体带引号。
	// cur1 读在 pull1 之后：拉取与断言之间没有任何写操作，两者即 pull1 时的版本。
	cur1, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	wantETag := computeBodyETag([]byte(body1))
	if etag1 != wantETag {
		t.Fatalf("ETag 公式不符: got %s want %s（cur=%d）", etag1, wantETag, cur1)
	}
	// 回填的产物行：body 与首次响应逐字节一致，版本号与 ETag 都对得上。
	art, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art == nil {
		t.Fatalf("第一次拉取后应已回填产物")
	}
	if string(art.Body) != body1 {
		t.Fatalf("产物 body 与首次响应不一致: len %d vs %d", len(art.Body), len(body1))
	}
	if art.Version != cur1 {
		t.Fatalf("产物版本 %d 应等于回填时的全局版本 %d", art.Version, cur1)
	}
	if art.ETag != etag1 {
		t.Fatalf("产物 ETag %s 应等于响应头 ETag %s", art.ETag, etag1)
	}

	// 第二次拉取：命中——产物就是首次编译的原始字节，逐字节一致即证明走了缓存
	// （两次实时编译因随机 User-Agent 行与生成时间几乎必然不同）。
	st2, hdr2, body2 := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if st2 != http.StatusOK {
		t.Fatalf("第二次拉取 status=%d body=%s", st2, body2)
	}
	if body2 != body1 {
		t.Fatalf("第二次拉取应命中产物回原字节: len %d vs %d", len(body2), len(body1))
	}
	if got := hdr2.Get("ETag"); got != etag1 {
		t.Fatalf("第二次拉取 ETag = %q，应与首次一致 %q", got, etag1)
	}
}

// TestSubscribeArtifact304 第一次拉取拿到 ETag 后：带 If-None-Match 再拉 → 304 无
// body；If-None-Match: * → 304；不匹配的标签与逗号列表也按 RFC 语义处理。
func TestSubscribeArtifact304(t *testing.T) {
	ts, _ := artifactSetup(t)
	path := "/subscribe?token=sub-token-1"

	st1, hdr1, body1 := uiGet(t, ts, path)
	if st1 != http.StatusOK || body1 == "" {
		t.Fatalf("第一次拉取 status=%d body=%s", st1, body1)
	}
	etag := hdr1.Get("ETag")
	if etag == "" {
		t.Fatalf("第一次拉取应带 ETag 头")
	}

	// 命中 → 304 且无 body。
	st2, _, body2 := artifactGet(t, ts, path, etag)
	if st2 != http.StatusNotModified {
		t.Fatalf("If-None-Match 命中应回 304，got %d body=%s", st2, body2)
	}
	if body2 != "" {
		t.Fatalf("304 不应带 body，got %d 字节", len(body2))
	}
	// * 匹配任意标签 → 304。
	st3, _, body3 := artifactGet(t, ts, path, "*")
	if st3 != http.StatusNotModified || body3 != "" {
		t.Fatalf("If-None-Match: * 应回 304 无 body，got %d body=%d 字节", st3, len(body3))
	}
	// 不匹配的标签 → 回 200 全文（仍是命中路径，body 为产物原字节）。
	st4, _, body4 := artifactGet(t, ts, path, `"nope"`)
	if st4 != http.StatusOK || body4 != body1 {
		t.Fatalf("If-None-Match 不匹配应回 200 原文，got %d len=%d", st4, len(body4))
	}
	// 逗号分隔列表里包含匹配项 → 304。
	st5, _, body5 := artifactGet(t, ts, path, `"nope", `+etag)
	if st5 != http.StatusNotModified || body5 != "" {
		t.Fatalf("逗号列表命中应回 304 无 body，got %d body=%d 字节", st5, len(body5))
	}
}

// TestSubscribeArtifactInvalidatedOnChange 拉取回填后改一条规则（走 mgr 方法，
// 写路径自动 bump 版本号），再拉 → miss：实时重编译出现新规则，产物行按新版本回填。
func TestSubscribeArtifactInvalidatedOnChange(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	group, err := mgr.AddProxyGroup(ctx, "AutoSelect", "select", ".*", "", 0, 0)
	if err != nil {
		t.Fatalf("插入 group: %v", err)
	}
	rule, err := mgr.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", group.ID, true, true)
	if err != nil {
		t.Fatalf("插入 rule: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	st1, _, body1 := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if st1 != http.StatusOK {
		t.Fatalf("第一次拉取 status=%d body=%s", st1, body1)
	}
	// 用完整规则行断言：fixture 的节点服务器名都带 example.com，裸子串会撞车。
	if !strings.Contains(body1, "DOMAIN-SUFFIX,example.com") {
		t.Fatalf("首次响应应包含原规则 DOMAIN-SUFFIX,example.com")
	}
	art1, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art1 == nil {
		t.Fatalf("第一次拉取后应已回填产物")
	}
	verBefore, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if art1.Version != verBefore {
		t.Fatalf("回填版本 %d 应等于当前全局版本 %d", art1.Version, verBefore)
	}

	// 改一条规则：真实写路径自动 bump 全局版本号（相对断言：+1）。
	if err := mgr.UpdateRule(ctx, rule.ID, "DOMAIN-SUFFIX", "example.org", group.ID, true, true); err != nil {
		t.Fatalf("改规则: %v", err)
	}
	verAfter, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	if verAfter != verBefore+1 {
		t.Fatalf("改规则后版本号应 +1：%d → %d", verBefore, verAfter)
	}

	// 再拉 → miss：实时重编译，新规则出现、旧规则消失。
	st2, _, body2 := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if st2 != http.StatusOK {
		t.Fatalf("第二次拉取 status=%d body=%s", st2, body2)
	}
	if !strings.Contains(body2, "DOMAIN-SUFFIX,example.org") {
		t.Fatalf("失效后拉取应包含新规则 DOMAIN-SUFFIX,example.org")
	}
	if strings.Contains(body2, "DOMAIN-SUFFIX,example.com") {
		t.Fatalf("失效后拉取不应再包含旧规则 DOMAIN-SUFFIX,example.com")
	}
	// 产物表已按新版本回填。
	art2, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art2 == nil || art2.Version != verAfter {
		t.Fatalf("第二次拉取后产物应按新版本 %d 回填，got %+v", verAfter, art2)
	}
}

// TestSubscribeArtifactQueryOverrideMiss 带 target= 覆盖参数拉取：非默认组合
// 不回填产物（产物表行数不变），但响应同样带内容指纹 ETag（剥离易变行后的
// body 哈希对非默认组合同样成立）；第二次相同拉取带 If-None-Match → 304。
func TestSubscribeArtifactQueryOverrideMiss(t *testing.T) {
	ts, mgr := artifactSetup(t)
	ctx := context.Background()

	path := "/subscribe?token=sub-token-1&target=clash.meta"
	st, hdr, body := uiGet(t, ts, path)
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, body)
	}
	if body == "" {
		t.Fatalf("拉取得到空响应体")
	}
	etag := hdr.Get("ETag")
	if etag == "" {
		t.Fatalf("非默认组合也应带内容指纹 ETag")
	}
	if want := computeBodyETag([]byte(body)); etag != want {
		t.Fatalf("非默认组合 ETag %s 应为 body 剥离哈希 %s", etag, want)
	}
	art, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art != nil {
		t.Fatalf("带覆盖参数拉取不应回填产物，got version=%d", art.Version)
	}

	// 第二次相同拉取（非默认组合、实时路径）：If-None-Match 命中 → 304 无 body。
	st2, _, body2 := artifactGet(t, ts, path, etag)
	if st2 != http.StatusNotModified {
		t.Fatalf("非默认组合 If-None-Match 命中应回 304，got %d body=%s", st2, body2)
	}
	if body2 != "" {
		t.Fatalf("304 不应带 body，got %d 字节", len(body2))
	}
}

// TestSubscribeArtifactModeOff artifact_mode=0：关闭后无命中候选、不回填（产物表
// 始终没有行），两次拉取都走实时路径；但内容指纹 ETag 与产物开关无关——开关关闭
// 时同输入重编译剥离易变行后同样逐字节一致，响应照样带 ETag 且两次相同，
// If-None-Match 命中 → 304。
func TestSubscribeArtifactModeOff(t *testing.T) {
	ts, mgr := artifactSetup(t)
	ctx := context.Background()
	if err := mgr.SetSettings(ctx, map[string]string{subforge.ArtifactModeKey: "0"}); err != nil {
		t.Fatalf("关闭产物模式: %v", err)
	}

	path := "/subscribe?token=sub-token-1"
	etag1 := ""
	for i := 1; i <= 2; i++ {
		st, hdr, body := uiGet(t, ts, path)
		if st != http.StatusOK {
			t.Fatalf("第 %d 次拉取 status=%d body=%s", i, st, body)
		}
		if body == "" {
			t.Fatalf("第 %d 次拉取得到空响应体", i)
		}
		etag := hdr.Get("ETag")
		if etag == "" {
			t.Fatalf("第 %d 次拉取（模式关闭）应带内容指纹 ETag", i)
		}
		if want := computeBodyETag([]byte(body)); etag != want {
			t.Fatalf("第 %d 次拉取 ETag %s 应为 body 剥离哈希 %s", i, etag, want)
		}
		if i == 1 {
			etag1 = etag
		} else if etag != etag1 {
			t.Fatalf("同输入两次重编译的 ETag 应相同：%s vs %s", etag1, etag)
		}
	}
	// If-None-Match 命中 → 304（模式关闭不影响 304 语义）。
	st304, _, body304 := artifactGet(t, ts, path, etag1)
	if st304 != http.StatusNotModified || body304 != "" {
		t.Fatalf("模式关闭下 If-None-Match 命中应回 304 无 body，got %d body=%d 字节", st304, len(body304))
	}
	art, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art != nil {
		t.Fatalf("模式关闭不应回填产物，got version=%d", art.Version)
	}
}

// TestSubscribeArtifactRefreshZero refresh_minutes=0：刷新间隔为 0 表示缓存整体
// 关闭，产物预生成一并禁用 → 不回填；响应仍带内容指纹 ETag（实时编译的剥离哈希）。
func TestSubscribeArtifactRefreshZero(t *testing.T) {
	ts, mgr := artifactSetup(t)
	ctx := context.Background()
	if err := mgr.SetSettings(ctx, map[string]string{subforge.SourceRefreshKey: "0"}); err != nil {
		t.Fatalf("关闭刷新间隔: %v", err)
	}

	st, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, body)
	}
	if body == "" {
		t.Fatalf("拉取得到空响应体")
	}
	etag := hdr.Get("ETag")
	if etag == "" {
		t.Fatalf("刷新间隔为 0 也应带内容指纹 ETag")
	}
	if want := computeBodyETag([]byte(body)); etag != want {
		t.Fatalf("ETag %s 应为 body 剥离哈希 %s", etag, want)
	}
	art, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art != nil {
		t.Fatalf("刷新间隔为 0 不应回填产物，got version=%d", art.Version)
	}
}

// --- 任务 6：/subscribe gzip 压缩 ---

// subscribeGetRaw 发一个可自定义 Accept-Encoding 与 If-None-Match 的 GET，返回
// 状态码、响应头与原始响应体。调用方手动设置 Accept-Encoding 时 net/http 传输层
// 不会自动解压（transport 只在它自己补该头时才负责解压），拿到什么就是什么——
// 验证 gzip 响应必须走这条路。
func subscribeGetRaw(t *testing.T, ts *httptest.Server, path, acceptEncoding, ifNoneMatch string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("构造请求: %v", err)
	}
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应: %v", err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

// gzipDecode 校验 gzip 魔数（0x1f 0x8b）后解压，返回解码出的文本。
func gzipDecode(t *testing.T, data string) string {
	t.Helper()
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		t.Fatalf("响应体不是 gzip 数据（缺魔数 0x1f8b）: len=%d prefix=%q", len(data), data[:min(len(data), 16)])
	}
	zr, err := gzip.NewReader(strings.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer zr.Close()
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip 解压: %v", err)
	}
	return string(decoded)
}

// TestSubscribeGzip 验证 /subscribe 的 gzip 压缩与既有语义的共存：
//   - 手动设置 Accept-Encoding: gzip → 响应带 Content-Encoding: gzip，body 是
//     可解压的 gzip 流，解出来与明文 body 逐字节一致；
//   - 完全不带 Accept-Encoding → 无 Content-Encoding 头，明文响应；
//   - gzip 请求 + If-None-Match 命中 → 仍回 304 且无 body（压缩不得破坏产物 304 路径）。
//
// 第一次明文拉取即回填产物，之后两次 gzip 请求命中同一份产物，所以 gzip 解码
// 结果可以与明文 body 逐字节比较，无需归一化。
func TestSubscribeGzip(t *testing.T) {
	ts, _ := artifactSetup(t)
	path := "/subscribe?token=sub-token-1"

	// 明文基线：DisableCompression 让传输层彻底不发 Accept-Encoding，服务端
	// 不该自作主张压缩。
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("构造请求: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("明文 GET: %v", err)
	}
	defer resp.Body.Close()
	plainBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取明文响应: %v", err)
	}
	if resp.StatusCode != http.StatusOK || len(plainBody) == 0 {
		t.Fatalf("明文拉取 status=%d body=%d 字节", resp.StatusCode, len(plainBody))
	}
	if got := resp.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("未请求压缩不应带 Content-Encoding，got %q", got)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("明文拉取（回填）应带 ETag")
	}

	// gzip 请求：手动设 Accept-Encoding，响应体就是服务端发出的原始字节。
	st, hdr, gzBody := subscribeGetRaw(t, ts, path, "gzip", "")
	if st != http.StatusOK {
		t.Fatalf("gzip 拉取 status=%d body len=%d", st, len(gzBody))
	}
	if got := hdr.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("gzip 请求应回 Content-Encoding: gzip，got %q body prefix=%q", got, gzBody[:min(len(gzBody), 16)])
	}
	decoded := gzipDecode(t, gzBody)
	if decoded != string(plainBody) {
		t.Fatalf("gzip 解码结果与明文 body 不一致: len %d vs %d", len(decoded), len(plainBody))
	}

	// 压缩与 304 共存：gzip 请求 + If-None-Match 命中 → 304 且无 body。
	st304, _, body304 := subscribeGetRaw(t, ts, path, "gzip", etag)
	if st304 != http.StatusNotModified {
		t.Fatalf("gzip + If-None-Match 命中应回 304，got %d body=%d 字节", st304, len(body304))
	}
	if body304 != "" {
		t.Fatalf("304 不应带 body，got %d 字节", len(body304))
	}
}

// --- 任务 10：条件覆盖补测——artifact.go 两个错误臂的真实行为用例 ---

// TestTrafficHeaderValueSurvivesDBReadError 流量策略读取失败（ctx 已取消 → config
// 查询报错）时返回空串，调用方不设 Subscription-Userinfo 头；同样的流量数据在正常
// ctx 下返回非空——证明空串来自「读失败静默跳过」，而不是没有数据。
func TestTrafficHeaderValueSurvivesDBReadError(t *testing.T) {
	_, srv, mgr := profileServer(t)
	ctx := context.Background()
	a, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true)
	if err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.UpdateAirportTraffic(ctx, a.ID, &traffic.Usage{
		Upload: 100, Download: 200, Total: 1000, Expire: 1700000000,
	}, 5, ""); err != nil {
		t.Fatalf("写入流量: %v", err)
	}
	airports, err := mgr.ListAirports(ctx)
	if err != nil {
		t.Fatalf("列机场: %v", err)
	}

	if got := srv.trafficHeaderValue(ctx, airports); got == "" {
		t.Fatalf("有流量数据且读取正常时应返回非空的 Subscription-Userinfo 值")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := srv.trafficHeaderValue(cancelled, airports); got != "" {
		t.Fatalf("流量策略读取失败时应返回空串（调用方不设头），got %q", got)
	}
}

// TestSubscribeArtifactBackfillFailureKeepsETag 惰性回填落库失败（BEFORE INSERT
// 触发器拒绝写入，真实 SQLite 报错）→ 响应仍是 200 全文，且照样带内容指纹 ETag：
// 指纹只依赖剥离易变行后的 body，与落库成败无关——保存失败只损失一次命中加速，
// 下次拉取重新编译，同输入必得相同 strip-hash，客户端按该 ETag 缓存不会错。
func TestSubscribeArtifactBackfillFailureKeepsETag(t *testing.T) {
	ts, mgr := artifactSetup(t)
	forbidArtifactSave(t, mgr)

	st, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, body)
	}
	if body == "" {
		t.Fatalf("回填失败也应回实时编译全文")
	}
	etag := hdr.Get("ETag")
	if etag == "" {
		t.Fatalf("回填失败也应带内容指纹 ETag")
	}
	if want := computeBodyETag([]byte(body)); etag != want {
		t.Fatalf("ETag %s 应为 body 剥离哈希 %s", etag, want)
	}
	art, err := mgr.GetSubArtifact(context.Background(), "sub-token-1")
	if err != nil {
		t.Fatalf("读产物: %v", err)
	}
	if art != nil {
		t.Fatalf("写入被拒绝后产物表不应有行，got version=%d", art.Version)
	}
}

// TestSubscribeArtifactOutOfBandDrift 带外内容漂移回归（内容指纹 ETag 的核心
// 价值）：换上游内容（模拟手改模板文件 / 远端模板 URL 漂移），全程不走任何
// bump 版本号的写路径；一个 TTL 之后后台重建把新内容收敛进产物，此时持有旧
// ETag 的条件拉取必须拿到 200 新正文——旧公式 etag=sha256(版本+令牌) 下版本号
// 未动，客户端会永远 304 看不到新内容。
func TestSubscribeArtifactOutOfBandDrift(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Fatalf("读取 fixture: %v", err)
	}

	// 可变上游：handler 吐 mutex 保护的字节切片，测试中途换内容即「带外漂移」。
	var mu sync.Mutex
	content := append([]byte(nil), buf...)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write(content)
	}))
	t.Cleanup(up.Close)

	ts, srv, mgr := profileServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", up.URL+"/sub.txt", "", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	// 第一次默认组合拉取：实时编译 + 回填产物，拿到初始 ETag。
	path := "/subscribe?token=sub-token-1"
	st1, hdr1, body1 := uiGet(t, ts, path)
	if st1 != http.StatusOK || !strings.Contains(body1, "ss-node") {
		t.Fatalf("第一次拉取 status=%d body=%s", st1, body1)
	}
	etag1 := hdr1.Get("ETag")
	if etag1 == "" {
		t.Fatalf("第一次拉取（回填）应带 ETag")
	}

	// 带外漂移：上游内容追加一个新节点；期间不写库、不 bump 版本号。
	drifted := append(append([]byte(nil), buf...),
		[]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmQxMjM@drift.example.com:8388#drift-node\n")...)
	mu.Lock()
	content = drifted
	mu.Unlock()

	// 把 source_cache 与产物的时钟拨老 2 小时（等价于等一个 TTL 过去），让下一轮
	// rebuildArtifacts 判定为超龄重建——这仍是纯时间流逝模拟，版本号不动。
	rawExec(t, mgr, "UPDATE source_cache SET fetched_at = fetched_at - 7200")
	art, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil || art == nil {
		t.Fatalf("第一次拉取后应已回填产物: %v", err)
	}
	if err := mgr.SaveSubArtifact(ctx, store.SubArtifact{
		Token: art.Token, ProfileID: art.ProfileID, Version: art.Version,
		Body: art.Body, ETag: art.ETag,
		GeneratedAt: time.Now().Add(-2 * time.Hour).UTC(),
	}); err != nil {
		t.Fatalf("把产物时间戳改旧: %v", err)
	}

	// 一轮后台重建：超龄 → 重编译（源缓存同样超龄 → 真正拉到漂移后的上游）。
	srv.rebuildArtifacts(ctx)

	// 持有旧 ETag 的条件拉取：必须 200 新正文（旧公式这里是 304——回归点）。
	st2, hdr2, body2 := artifactGet(t, ts, path, etag1)
	if st2 != http.StatusOK {
		t.Fatalf("带外漂移 + 重建后，旧 ETag 的条件拉取应回 200 新正文，got %d", st2)
	}
	if !strings.Contains(body2, "drift-node") {
		t.Fatalf("200 响应应包含漂移后的新节点 drift-node:\n%s", body2)
	}
	etag2 := hdr2.Get("ETag")
	if etag2 == "" || etag2 == etag1 {
		t.Fatalf("漂移后的 ETag 应变化：old=%s new=%s", etag1, etag2)
	}

	// 新 ETag 的条件拉取 → 304（304 语义在内容指纹下保留）。
	st3, _, body3 := artifactGet(t, ts, path, etag2)
	if st3 != http.StatusNotModified || body3 != "" {
		t.Fatalf("新 ETag 的条件拉取应回 304 无 body，got %d body=%d 字节", st3, len(body3))
	}
}
