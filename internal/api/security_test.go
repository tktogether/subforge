package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestSecretsRedactedFromLogs 订阅令牌既不能在错误 body 里回显，也不能进 /api/logs。
// 覆盖两种入口：query（/subscribe）与 path（/api/tokens/{token}）。
func TestSecretsRedactedFromLogs(t *testing.T) {
	ts, _, _ := profileServer(t)
	const secret = "SECRET-LEAK-20261030"
	for _, path := range []string{
		"/subscribe?token=" + secret,
		"/api/tokens/" + secret,
	} {
		st, _, body := uiGet(t, ts, path)
		if st != http.StatusNotFound {
			t.Fatalf("%s 状态=%d，应为 404；body=%s", path, st, body)
		}
		if strings.Contains(body, secret) {
			t.Fatalf("错误 body 回显了令牌：%s", body)
		}
	}
	st, _, logs := uiGet(t, ts, "/api/logs")
	if st != http.StatusOK {
		t.Fatalf("/api/logs 状态=%d", st)
	}
	if strings.Contains(logs, secret) {
		t.Fatalf("/api/logs 泄露令牌：%s", logs)
	}
}
