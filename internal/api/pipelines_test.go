package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/tktogether/subforge/internal/config"
)

func newPipelineTpl(t *testing.T) *config.Template {
	t.Helper()
	tpl, err := config.LoadBody([]byte(`[pipeline:clean]`+"\n"+`step = op.dedupe`+"\n"+`on_error = warn`+"\n"), "test.ini")
	if err != nil {
		t.Fatalf("LoadBody: %v", err)
	}
	return tpl
}

func TestAPIPipelines_NoManager_ReturnsArray(t *testing.T) {
	ts := newTestServer(t, nil)
	status, _, body := uiGet(t, ts, "/api/pipelines")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("body not array: %v body=%s", err, body)
	}
	if len(arr) != 0 {
		t.Fatalf("want empty array, got %d", len(arr))
	}
}

func TestAPIPipelines_TemplateSource(t *testing.T) {
	tpl := newPipelineTpl(t)
	ts, _ := uiSubServerCfg(t, func(c *Config) {
		c.Engine.BaseTemplate = func() *config.Template { return tpl }
	})
	status, _, body := uiGet(t, ts, "/api/pipelines")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("want 1, got %d body=%s", len(arr), body)
	}
	if arr[0]["source"] != "template" {
		t.Fatalf("source = %v, want template", arr[0]["source"])
	}
	if ro, _ := arr[0]["read_only"].(bool); !ro {
		t.Fatalf("read_only = %v, want true", arr[0]["read_only"])
	}
	if arr[0]["name"] != "clean" {
		t.Fatalf("name = %v, want clean", arr[0]["name"])
	}
}

func TestAPIPipelines_DBOverridesTemplate(t *testing.T) {
	tpl := newPipelineTpl(t)
	ts, mgr := uiSubServerCfg(t, func(c *Config) {
		c.Engine.BaseTemplate = func() *config.Template { return tpl }
	})
	if _, err := mgr.CreatePipeline(t.Context(), "clean", "db desc", "[]", true); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	status, _, body := uiGet(t, ts, "/api/pipelines")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("want 1 (dedup), got %d body=%s", len(arr), body)
	}
	if arr[0]["source"] != "ui" {
		t.Fatalf("source = %v, want ui (DB priority)", arr[0]["source"])
	}
}

func TestAPIPipelinesCreate_201(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/pipelines", `{"name":"p1","steps":[{"op":"op.dedupe"}]}`)
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	p, _ := got["pipeline"].(map[string]any)
	if p["name"] != "p1" {
		t.Fatalf("name = %v, want p1", p["name"])
	}
}

func TestAPIPipelinesCreate_Conflict_409(t *testing.T) {
	ts, mgr := uiSubServer(t)
	if _, err := mgr.CreatePipeline(t.Context(), "dup", "d", "[]", true); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	status, _, body := uiPostJSON(t, ts, "/api/pipelines", `{"name":"dup"}`)
	if status != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", status, body)
	}
}
func TestAPIPipelinesCreate_MissingName_400(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/pipelines", `{"name":"   "}`)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestAPIPipelines_GetByID_OK(t *testing.T) {
	ts, mgr := uiSubServer(t)
	p, err := mgr.CreatePipeline(t.Context(), "p2", "d", "[]", true)
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	status, _, body := uiGet(t, ts, fmt.Sprintf("/api/pipelines/%d", p.ID))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pp, _ := got["pipeline"].(map[string]any)
	if pp["name"] != "p2" {
		t.Fatalf("name = %v, want p2", pp["name"])
	}
}

func TestAPIPipelines_GetByID_404(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiGet(t, ts, "/api/pipelines/999999")
	if status != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestAPIPipelines_Put_OK(t *testing.T) {
	ts, mgr := uiSubServer(t)
	p, err := mgr.CreatePipeline(t.Context(), "p3", "d", "[]", true)
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	status, _, body := uiDoJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/pipelines/%d", p.ID), `{"name":"p3b","steps_json":"[]"}`)
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got map[string]any
	_ = json.Unmarshal([]byte(body), &got)
	pp, _ := got["pipeline"].(map[string]any)
	if pp["name"] != "p3b" {
		t.Fatalf("name = %v, want p3b", pp["name"])
	}
}

func TestAPIPipelines_Delete_OK(t *testing.T) {
	ts, mgr := uiSubServer(t)
	p, err := mgr.CreatePipeline(t.Context(), "p4", "d", "[]", true)
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	status, _, body := uiDoJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/pipelines/%d", p.ID), "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	status2, _, _ := uiGet(t, ts, fmt.Sprintf("/api/pipelines/%d", p.ID))
	if status2 != http.StatusNotFound {
		t.Fatalf("after delete status=%d, want 404", status2)
	}
}

func TestAPIPipelines_WrongMethod_405(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, _ := uiDoJSON(t, ts, http.MethodGet, "/api/pipelines", "")
	// GET /api/pipelines is allowed (200); use DELETE on collection instead.
	status2, _, _ := uiDoJSON(t, ts, http.MethodDelete, "/api/pipelines", "")
	if status2 == http.StatusOK {
		t.Fatalf("DELETE /api/pipelines should not be 200, got %d (GET=%d)", status2, status)
	}
}
