package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/scheduler"
)

// tasksFakeJob 是测试用的可控任务。
type tasksFakeJob struct{ name string }

func (f *tasksFakeJob) Name() string                { return f.name }
func (f *tasksFakeJob) Spec() string                { return "@every 1s" }
func (f *tasksFakeJob) Run(_ context.Context) error { return nil }

func newTasksTestServer(t *testing.T) (*httptest.Server, *Server, *scheduler.Scheduler) {
	t.Helper()
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test", Timeout: 5 * time.Second,
		Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	cfg := Config{
		Engine: &convert.Engine{
			Fetcher: fetcher,
			RendererFor: func(target string) (render.Renderer, error) {
				return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	}
	srv := New(cfg)
	sch := scheduler.New()
	srv.SetScheduler(sch)
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts, srv, sch
}

func TestAPITasksNoScheduler(t *testing.T) {
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test", Timeout: 5 * time.Second,
		Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	cfg := Config{
		Engine: &convert.Engine{
			Fetcher: fetcher,
			RendererFor: func(target string) (render.Renderer, error) {
				return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	}
	srv := New(cfg)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	srv.handleAPITasks(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !tasksContainsStr(w.Body.String(), "调度器未启用") {
		t.Fatalf("body missing error text: %s", w.Body.String())
	}
}

func TestAPITasksEmptyHistory(t *testing.T) {
	ts, srv, _ := newTasksTestServer(t)
	_ = ts
	_ = srv
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	srv.handleAPITasks(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if got.Tasks == nil {
		t.Fatal("tasks 为 null，want []")
	}
	if len(got.Tasks) != 0 {
		t.Fatalf("len=%d, want 0", len(got.Tasks))
	}
}

func TestAPITasksAfterJob(t *testing.T) {
	_, srv, sch := newTasksTestServer(t)
	job := &tasksFakeJob{name: "job1"}
	if err := sch.AddJob(job); err != nil {
		t.Fatal(err)
	}
	sch.Start()
	defer sch.Stop()
	time.Sleep(2 * time.Second)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	srv.handleAPITasks(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if len(got.Tasks) < 1 {
		t.Fatalf("len=%d, want >=1", len(got.Tasks))
	}
	// 字段名与 JobResult json tag 对齐（无 tag → Go 字段名）
	task := got.Tasks[0]
	for _, key := range []string{"JobName", "Success", "Err", "StartedAt", "Duration"} {
		if _, ok := task[key]; !ok {
			t.Errorf("missing key %q in task: %v", key, task)
		}
	}
	if got.Tasks[0]["JobName"] != "job1" {
		t.Errorf("JobName=%v, want job1", got.Tasks[0]["JobName"])
	}
	if got.Tasks[0]["Success"] != true {
		t.Errorf("Success=%v, want true", got.Tasks[0]["Success"])
	}
}

func TestAPITasksWrongMethod(t *testing.T) {
	_, srv, _ := newTasksTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/tasks", nil)
	srv.handleAPITasks(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !tasksContainsStr(w.Body.String(), "仅支持 GET") {
		t.Fatalf("body missing error text: %s", w.Body.String())
	}
}

func TestAPITasksRealRoute(t *testing.T) {
	ts, _, sch := newTasksTestServer(t)
	job := &tasksFakeJob{name: "job2"}
	if err := sch.AddJob(job); err != nil {
		t.Fatal(err)
	}
	sch.Start()
	defer sch.Stop()
	time.Sleep(2 * time.Second)
	resp, err := http.Get(ts.URL + "/api/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var got struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Tasks) < 1 {
		t.Fatalf("len=%d, want >=1", len(got.Tasks))
	}
}

func TestAPITasksEmptyHistoryNotNull(t *testing.T) {
	_, srv, _ := newTasksTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	srv.handleAPITasks(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !tasksContainsStr(w.Body.String(), "\"tasks\":[]") {
		t.Fatalf("body not empty array: %s", w.Body.String())
	}
}

// containsStr is a small helper (already defined elsewhere? guard with local name).
func tasksContainsStr(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
