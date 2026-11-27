package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestAPIScriptsTrimAndConflict 脚本名写入去首尾空白；创建/改名撞名返回 409。
func TestAPIScriptsTrimAndConflict(t *testing.T) {
	ts, _ := uiSubServer(t)

	status, _, body := uiPostJSON(t, ts, "/api/scripts",
		`{"name":"  s1  ","kind":"filter","code":"var x = 1;","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("创建脚本状态 = %d；body=%s", status, body)
	}
	var created struct {
		Script struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"script"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if created.Script.Name != "s1" {
		t.Fatalf("脚本名未去空白: %q", created.Script.Name)
	}

	status, _, body = uiPostJSON(t, ts, "/api/scripts",
		`{"name":"s1","kind":"filter","code":"var x = 1;","enabled":true}`)
	if status != http.StatusConflict {
		t.Fatalf("重名创建状态 = %d，期望 409；body=%s", status, body)
	}

	status, _, body = uiPostJSON(t, ts, "/api/scripts",
		`{"name":"s2","kind":"filter","code":"var x = 1;","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("创建 s2 状态 = %d；body=%s", status, body)
	}
	var second struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	if err := json.Unmarshal([]byte(body), &second); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	status, _, body = uiDoJSON(t, ts, http.MethodPut,
		fmt.Sprintf("/api/scripts/%d", second.Script.ID),
		`{"name":"s1","kind":"filter","code":"var x = 1;","enabled":true}`)
	if status != http.StatusConflict {
		t.Fatalf("重名改名状态 = %d，期望 409；body=%s", status, body)
	}
}
