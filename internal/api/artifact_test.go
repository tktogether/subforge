// artifact_test.go — 订阅产物预生成（任务 5）：/subscribe 命中路径、惰性回填与
// ETag/304 的端到端行为测试。全部走 httptest + 临时 SQLite（uiSubServer 装配），
// 六个场景对应任务简报；命中路径回的是首次实时编译的原始字节（含时间戳行与
// mihomo 内核注入的随机 User-Agent 行，随首次编译一起定格），因此 live-vs-hit
// 逐字节比较即可，无需归一化。版本号断言一律用相对值（+1），不写绝对数。
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/subforge"
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

// TestSubscribeArtifactHit 默认组合拉取两次：第一次实时编译并回填产物（响应带
// ETag，公式 = sha256(version + "\n" + token) 带引号）；第二次命中——直接回产物的
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
	// ETag 公式锚定（白盒）：hex = sha256(version + "\n" + token)，整体带引号。
	// cur1 读在 pull1 之后：拉取与断言之间没有任何写操作，两者即 pull1 时的版本。
	cur1, err := mgr.ArtifactVersion(ctx)
	if err != nil {
		t.Fatalf("读产物版本号: %v", err)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s", cur1, "sub-token-1")))
	wantETag := `"` + hex.EncodeToString(sum[:]) + `"`
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
// 不回填产物（产物表行数不变），也不设 ETag。
func TestSubscribeArtifactQueryOverrideMiss(t *testing.T) {
	ts, mgr := artifactSetup(t)
	ctx := context.Background()

	st, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, body)
	}
	if body == "" {
		t.Fatalf("拉取得到空响应体")
	}
	if got := hdr.Get("ETag"); got != "" {
		t.Fatalf("非默认组合不应设 ETag，got %q", got)
	}
	art, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art != nil {
		t.Fatalf("带覆盖参数拉取不应回填产物，got version=%d", art.Version)
	}
}

// TestSubscribeArtifactModeOff artifact_mode=0：关闭后无 ETag 头，两次拉取都走
// 实时路径（产物表始终没有行——回填不发生，命中也没有候选）。
func TestSubscribeArtifactModeOff(t *testing.T) {
	ts, mgr := artifactSetup(t)
	ctx := context.Background()
	if err := mgr.SetSettings(ctx, map[string]string{subforge.ArtifactModeKey: "0"}); err != nil {
		t.Fatalf("关闭产物模式: %v", err)
	}

	for i := 1; i <= 2; i++ {
		st, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
		if st != http.StatusOK {
			t.Fatalf("第 %d 次拉取 status=%d body=%s", i, st, body)
		}
		if body == "" {
			t.Fatalf("第 %d 次拉取得到空响应体", i)
		}
		if got := hdr.Get("ETag"); got != "" {
			t.Fatalf("第 %d 次拉取（模式关闭）不应设 ETag，got %q", i, got)
		}
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
// 关闭，产物预生成一并禁用 → 无 ETag 头、不回填。
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
	if got := hdr.Get("ETag"); got != "" {
		t.Fatalf("刷新间隔为 0 不应设 ETag，got %q", got)
	}
	art, err := mgr.GetSubArtifact(ctx, "sub-token-1")
	if err != nil {
		t.Fatalf("读产物行: %v", err)
	}
	if art != nil {
		t.Fatalf("刷新间隔为 0 不应回填产物，got version=%d", art.Version)
	}
}
