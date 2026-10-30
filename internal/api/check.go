package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"strconv"

	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/subscheck"
)

type checkSnapshot struct {
	Results []subscheck.Result
	At      time.Time
}

// lastChecks 按 *Server 实例隔离测活结果；进程重启即失效，符合 plan「不落库」。
// 用 sync.Map 是为了不改 server.go 的 Server 结构体。
var lastChecks sync.Map

type checkReq struct {
	IDs json.RawMessage `json:"ids"`
}

// handleAPICheck 实现 POST /api/airports/check。
func (s *Server) handleAPICheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req checkReq
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil && err != io.EOF {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
	}
	airports, err := m.ListAirports(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	byID := map[int64]subforge.Airport{}
	for _, a := range airports {
		byID[a.ID] = a
	}
	var ids []int64
	hasIds := req.IDs != nil
	if hasIds {
		if err := json.Unmarshal(req.IDs, &ids); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		if len(ids) == 0 {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("没有可检测的机场"))
			return
		}
	}
	var targets []subforge.Airport
	if hasIds {
		for _, id := range ids {
			a, ok := byID[id]
			if !ok {
				s.writeAPIError(w, r, http.StatusBadRequest, errors.New("机场 "+strconv.FormatInt(id, 10)+" 不存在"))
				return
			}
			targets = append(targets, a)
		}
	} else {
		for _, a := range airports {
			if a.Enabled && a.SubscriptionURL != "" {
				targets = append(targets, a)
			}
		}
	}
	urls := make([]string, 0, len(targets))
	for _, a := range targets {
		urls = append(urls, a.SubscriptionURL)
	}
	results := subscheck.Check(r.Context(), urls, subscheck.Options{})
	if results == nil {
		results = []subscheck.Result{}
	}
	okN, failN := subscheck.Summary(results)
	at := time.Now().UTC()
	lastChecks.Store(s, &checkSnapshot{Results: results, At: at})
	s.writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"ok":      okN,
		"fail":    failN,
		"at":      at.Format(time.RFC3339),
	})
}

// handleAPICheckLast 实现 GET /api/airports/check/last。
func (s *Server) handleAPICheckLast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET"))
		return
	}
	if _, ok := s.subManagerOr503(w, r); !ok {
		return
	}
	v, ok := lastChecks.Load(s)
	if !ok {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"results": []subscheck.Result{},
			"at":      "",
		})
		return
	}
	snap := v.(*checkSnapshot)
	results := snap.Results
	if results == nil {
		results = []subscheck.Result{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"at":      snap.At.Format(time.RFC3339),
	})
}
