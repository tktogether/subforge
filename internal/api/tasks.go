package api

import (
	"errors"
	"net/http"

	"github.com/tktogether/subforge/internal/scheduler"
)

// handleAPITasks 返回定时调度器的最近任务执行历史（内存态，进程重启清空）。
func (s *Server) handleAPITasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET"))
		return
	}
	if s.scheduler == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errors.New("调度器未启用"))
		return
	}
	res := s.scheduler.Results()
	if res == nil {
		res = []scheduler.JobResult{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"tasks": res})
}
