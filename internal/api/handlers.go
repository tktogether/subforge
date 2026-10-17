// 处理器：/sub 转换、/getruleset 规则集、/inspect 参数回显。
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/ruleset"
)

// statusFor 把执行错误映射成 HTTP 状态码。
// 区分开三种性质：请求本身不对（400/404）、格式没实现（501）、上游有问题（502）。
func statusFor(err error) int {
	switch {
	case errors.Is(err, convert.ErrNoSource):
		return http.StatusBadRequest
	case errors.Is(err, convert.ErrNoNodes):
		return http.StatusNotFound
	case errors.Is(err, registry.ErrUnsupportedFormat):
		return http.StatusNotImplemented
	default:
		return http.StatusBadGateway
	}
}

// apiOnlyParams 是只属于 API 层的参数，不能进 Params.Raw，更不能被当成
// 上游订阅的认证信息转发出去。
var apiOnlyParams = map[string]bool{
	"token":     true,
	"apikey":    true,
	"accesskey": true,
}

// handleConvert 是 /sub /profile /render 的共同处理器。
//
// 流程：解析 query -> （target 缺省时按 UA 推断）-> Engine 执行 -> 按格式写响应。
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	q := sanitizeQuery(r.URL.Query())
	params, err := convert.ParseQuery(q)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err)
		return
	}

	// target 缺省时按 User-Agent 推断，这是客户端订阅地址的通行做法；
	// 推断不出就退回 clash.meta。
	if params.Target == "" {
		params.Target = guessTarget(r.UserAgent())
	}

	// list 模式走同一条 Engine 路径（target=list 的分享链接渲染器），不需要特殊处理。

	result, err := s.engine.Run(r.Context(), params)
	if err != nil {
		s.fail(w, r, statusFor(err), err)
		return
	}
	if len(result.Failures) > 0 {
		s.logger.Printf("rid=%s 部分来源失败(%d): %s",
			dash(r.Header.Get("X-Request-ID")), len(result.Failures),
			strings.Join(result.Failures, " | "))
	}

	w.Header().Set("Content-Type", contentTypeFor(result.Format))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filenameFor(params, result.Format)))
	w.Header().Set("X-Convert-Format", result.Format)
	w.Header().Set("X-Node-Count", fmt.Sprintf("%d", result.Nodes))
	if len(result.Failures) > 0 {
		w.Header().Set("X-Convert-Warning", fmt.Sprintf("%d 个来源失败", len(result.Failures)))
	}
	if len(result.Warnings) > 0 {
		// 模板提示（不认识的分组/规则写法）与渲染阶段被跳过的项合在一起。
		// 响应头只放计数——条目可能含 emoji，不是合法的 header 值。
		w.Header().Set("X-Convert-Skipped", fmt.Sprintf("%d", len(result.Warnings)))
		s.logger.Printf("rid=%s 跳过/忽略 %d 项: %s", dash(r.Header.Get("X-Request-ID")),
			len(result.Warnings), strings.Join(result.Warnings, " | "))
	}
	if n := countCached(result); n > 0 {
		w.Header().Set("X-Convert-Cache", fmt.Sprintf("命中 %d 个", n))
	}
	// 客户端 /sub 也记录历史，Dashboard 才能看到真实转换。
	// Groups/Rules 取自 Engine 的生效值：Dashboard 的状态概览要显示"分组有没有生效"，
	// 只记节点数看不出来。这两个计数与 /api/convert 路径同源，两条路不会漂移。
	if s.historyBuf != nil {
		s.historyBuf.Record(HistoryEntry{
			Target:   params.Target,
			Format:   result.Format,
			Nodes:    result.Nodes,
			RawNodes: result.RawNodes,
			Groups:   result.Groups,
			Rules:    result.Rules,
			Failures: result.Failures,
			Warnings: result.Warnings,
			Cached:   countCached(result),
			URLCount: len(params.URLs),
		})
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Body)
}

// handleGetRuleset 返回规则集，对齐 subconverter 的 /getruleset?type=&name=&url=。
//
// type 是六种输出形态的编号（1=Surge list / 2=QX / 3=Clash domain provider /
// 4=Clash ipcidr provider / 5=Surge DOMAIN-SET / 6=Clash classical），
// 取值优先级 URL 参数 > 模板 [ruleset] 段落；规则内容按 rules > list >
// 模板 [common] rules > url 远程拉取的顺序取。
//
// 规则数据一律由调用方给出，本服务不内置任何第三方规则集（R3）。
func (s *Server) handleGetRuleset(w http.ResponseWriter, r *http.Request) {
	q := sanitizeQuery(r.URL.Query())
	params, err := convert.ParseQuery(q)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err)
		return
	}
	name := firstVal(params.Raw["name"])
	if name == "" {
		s.fail(w, r, http.StatusBadRequest, fmt.Errorf("缺少 name 参数（规则集名）"))
		return
	}

	tpl, err := s.engine.ResolveTemplates(r.Context(), params.Config)
	if err != nil {
		s.fail(w, r, http.StatusBadGateway, err)
		return
	}

	typeStr := strings.TrimSpace(firstVal(params.Raw["type"]))
	if typeStr == "" {
		typeStr = tpl.Ruleset[name]
	}
	if typeStr == "" {
		s.fail(w, r, http.StatusNotFound, fmt.Errorf(
			"规则集 %q 没有类型：请传 type，或在模板 [ruleset] 段落里配置 %s", name, name))
		return
	}
	format, err := ruleset.ParseFormat(typeStr)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err)
		return
	}

	var lines []string
	switch {
	case len(params.Rules) > 0:
		lines = params.Rules
	case strings.TrimSpace(firstVal(params.Raw["list"])) != "":
		lines = strings.Split(firstVal(params.Raw["list"]), ";")
	case len(tpl.Rules) > 0:
		lines = tpl.Rules
	case len(params.URLs) > 0:
		lines, err = s.fetchRulesetURLs(r.Context(), params.URLs)
		if err != nil {
			s.fail(w, r, http.StatusBadGateway, err)
			return
		}
	}
	if len(lines) == 0 {
		s.fail(w, r, http.StatusBadRequest, fmt.Errorf(
			"规则集 %q 没有规则内容：请传 rules，或在模板 [common] 段落写 rules，或用 url 指向远程规则文件", name))
		return
	}

	set, err := ruleset.NewSet(lines...)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err)
		return
	}
	if len(params.Include) > 0 || len(params.Exclude) > 0 {
		set, err = set.Filter(params.Include, params.Exclude)
		if err != nil {
			s.fail(w, r, http.StatusBadRequest, err)
			return
		}
	}
	if set.Empty() {
		s.fail(w, r, http.StatusNotFound, fmt.Errorf("规则集 %q 过滤后为空：检查 include / exclude", name))
		return
	}

	res, err := set.Render(format, name, ruleset.RenderOpts{
		Action: strings.TrimSpace(firstVal(params.Raw["out"])),
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ruleset.ErrMissingAction) {
			status = http.StatusBadRequest
		}
		s.fail(w, r, status, err)
		return
	}

	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", res.Filename))
	w.Header().Set("X-Ruleset-Format", format.Label())
	w.Header().Set("X-Ruleset-Count", fmt.Sprintf("%d", res.Out))
	if res.Skipped > 0 {
		// 显式报出来：DOMAIN-SET 之类的形态表达不了域名正则，静默丢掉没人会发现。
		w.Header().Set("X-Ruleset-Skipped", fmt.Sprintf("%d", res.Skipped))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res.Body)
}

// fetchRulesetURLs 逐个拉取远程规则文件并拼接；单个来源失败不中断整批。
// 用 config.HTTPLoader 而不是 fetch 包：那是给订阅内容用的，会把规则集文本
// 当订阅去识别形态。
func (s *Server) fetchRulesetURLs(ctx context.Context, urls []string) ([]string, error) {
	load := config.HTTPLoader(s.http, nil)
	var (
		lines    []string
		failures []string
	)
	for _, u := range urls {
		buf, err := load(ctx, u)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", u, err))
			continue
		}
		for _, l := range strings.Split(string(buf), "\n") {
			l = strings.TrimRight(l, "\r")
			if strings.TrimSpace(l) != "" {
				lines = append(lines, l)
			}
		}
	}
	if len(failures) > 0 {
		if len(lines) == 0 {
			return nil, fmt.Errorf("远程规则全部失败:\n  %s", strings.Join(failures, "\n  "))
		}
		s.logger.Printf("规则集部分来源失败(%d): %s", len(failures), strings.Join(failures, " | "))
	}
	return lines, nil
}

// firstVal 取第一个非空取值。
func firstVal(v []string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// handleInspect 回显解析后的参数，排查 query 写错时用。
// 不转发任何凭证，也不执行转换。
func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	params, err := convert.ParseQuery(sanitizeQuery(r.URL.Query()))
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"target":     params.Target,
		"urls":       params.URLs,
		"insert":     params.Insert,
		"include":    params.Include,
		"exclude":    params.Exclude,
		"types":      params.Types,
		"rename":     len(params.Rename),
		"emoji":      len(params.Emoji),
		"list":       params.List,
		"udp":        params.UDP,
		"tfo":        params.TFO,
		"scv":        params.SCV,
		"tls13":      params.TLS13,
		"sort":       params.Sort,
		"config":     params.Config,
		"ruleset":    params.Ruleset,
		"unknown":    unknownKeys(params),
		"extensions": params.Extensions,
		"time":       time.Now().UTC().Format(time.RFC3339),
	})
}

// sanitizeQuery 去掉只属于 API 层的参数（令牌等），避免它们混进转换参数或
// 被当成上游订阅的认证信息。
func sanitizeQuery(q map[string][]string) map[string][]string {
	out := make(map[string][]string, len(q))
	for k, v := range q {
		if apiOnlyParams[strings.ToLower(strings.TrimSpace(k))] {
			continue
		}
		out[k] = v
	}
	return out
}

// fail 写出错误响应，并在日志里带上上下文。
func (s *Server) fail(w http.ResponseWriter, r *http.Request, status int, err error) {
	s.logger.Printf("rid=%s %s %s -> %d: %v",
		dash(chimw.GetReqID(r.Context())), r.Method, r.URL.RequestURI(), status, err)
	s.writeJSON(w, status, map[string]any{"error": err.Error()})
}

// unknownKeys 列出参数里没有被任何字段消费掉的键。
func unknownKeys(p convert.Params) []string {
	known := map[string]bool{
		"target": true, "url": true, "insert": true, "include": true, "exclude": true,
		"type": true, "rename": true, "emoji": true, "list": true, "udp": true, "tfo": true,
		"scv": true, "tls13": true, "sort": true, "config": true, "ruleset": true,
	}
	out := []string{}
	for k := range p.Raw {
		if strings.HasPrefix(k, "x-") || known[k] {
			continue
		}
		out = append(out, k)
	}
	return out
}

// countCached 统计命中磁盘缓存的来源数。
func countCached(r convert.Result) int {
	n := 0
	for _, hit := range r.Cached {
		if hit {
			n++
		}
	}
	return n
}

// contentTypeFor 按目标格式给出 Content-Type。未知格式退回 octet-stream。
func contentTypeFor(target string) string {
	switch strings.ToLower(target) {
	case "", "clash", "clash.meta", "clashmeta", "mihomo", "yaml":
		return "text/yaml; charset=utf-8"
	case "list", "mixed", "surge", "surge4", "surge-ios", "surge-android":
		return "text/plain; charset=utf-8"
	case "v2ray", "base64":
		return "text/plain; charset=utf-8"
	case "sing-box", "mihomo-party":
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// filenameFor 给出下载文件名。format 是生效的输出格式——list 模式的产物不是 YAML。
//
// 优先级：请求显式给的 filename > 模板名 > 按格式派生。
// 显式 filename 排最前是因为它就是用户为了覆盖后两者才传的。
func filenameFor(p convert.Params, format string) string {
	if p.Filename != "" {
		return p.Filename
	}
	if p.Config != "" {
		return p.Config
	}
	switch strings.ToLower(format) {
	case "list":
		return "list.txt"
	case "mixed":
		return "mixed.txt"
	case "sing-box", "singbox":
		return "sing-box.json"
	}
	name := format
	if name == "" {
		name = "clash.meta"
	}
	if strings.HasSuffix(name, ".json") {
		// JSON 形态的 target 名字里已带扩展名，不能再换成 .yaml。
		return name
	}
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[:dot] + ".yaml"
	}
	return name + ".yaml"
}
