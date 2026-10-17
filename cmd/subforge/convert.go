// convert 命令：本地把订阅（URL 或文件）转换成目标客户端配置。
//
// 这里只做参数归一与结果输出，转换流程在 internal/convert，与 HTTP API 共用。
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/buildinfo"
	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/transform"
)

// multiFlag 允许同一 flag 出现多次。
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

const convertUsage = `subforge convert - 本地订阅转换

用法:
  subforge convert [-url <订阅地址>]... [-file <本地文件>]... [选项]

输入:
  -url      订阅地址，可重复（分享链接 / 整体 Base64 / Clash YAML 自动识别）
  -file     本地订阅或配置文件，可重复
  -config   subconverter 风格的 INI 模板（[common] / [node_pref] / [rename] /
            [emoji] / [clash_proxy_group] / [ruleset] / [pipeline:<名>]）；
            命令行参数优先级更高
  -pipeline 选中 -config 模板里的 [pipeline:<名>] 命名管线；不给时走默认装配
            （filter -> node_pref -> rename -> emoji -> dedupe -> sort）

处理:
  -include  节点名白名单正则，可重复
  -exclude  节点名黑名单正则，可重复
  -type     仅保留指定协议类型，可重复（如 -type vless -type hysteria2）
  -rename  名称正则替换 "正则,替换"，可重复
  -emoji   名称正则加 emoji "正则,emoji"，可重复
  -sort     排序键：original / name / type / server / port

输出:
  -format   输出格式（默认 clash.meta；不给时取 -config 模板里 [common] 的 target）
  -list     输出分享链接清单而不是客户端配置（等价 target=list，优先于 -format）
  -out      输出文件路径，默认 stdout
  -mixed-port clash.meta mixed-port（默认 7890）
  -name     配置名，写入产物注释头

其他:
  -header   附加请求头 "K: V"，可重复
  -cache-dir 上游内容磁盘缓存目录（默认不缓存）
  -timeout  上游请求超时秒数（默认 30）
  -override  本地覆盖层文件（YAML / INI）：改代理组、prepend/append 规则、
            按名正则改节点 server/port；用 "subforge diff" 预览改动
`

func cmdConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	var (
		urls, files, include, exclude, types, rename, emoji, headers   multiFlag
		out, format, sortKey, name, cacheDir, configFile, overrideFile string
		pipeline                                                       string
		mixedPort, timeoutSeconds                                      int
		list                                                           bool
	)
	fs.Var(&urls, "url", "订阅地址，可重复")
	fs.Var(&files, "file", "本地订阅或配置文件，可重复")
	fs.Var(&include, "include", "节点名白名单正则，可重复")
	fs.Var(&exclude, "exclude", "节点名黑名单正则，可重复")
	fs.Var(&types, "type", "仅保留指定协议类型，可重复")
	fs.Var(&rename, "rename", "名称正则替换 \"正则,替换\"，可重复")
	fs.Var(&emoji, "emoji", "名称正则加 emoji \"正则,emoji\"，可重复")
	fs.StringVar(&out, "out", "", "输出文件路径，默认 stdout")
	fs.StringVar(&format, "format", "", "输出格式；不给时取 -config 模板 [common] 的 target，再退回 clash.meta")
	fs.BoolVar(&list, "list", false, "输出分享链接清单而不是客户端配置（优先于 -format）")
	fs.StringVar(&sortKey, "sort", "", "排序键：original / name / type / server / port")
	fs.StringVar(&name, "name", "", "配置名，写入产物注释头")
	fs.StringVar(&configFile, "config", "", "subconverter 风格 INI 模板")
	fs.StringVar(&pipeline, "pipeline", "", "选中 -config 模板里的 [pipeline:<名>] 命名管线")
	fs.StringVar(&cacheDir, "cache-dir", "", "上游内容磁盘缓存目录（默认不缓存）")
	fs.StringVar(&overrideFile, "override", "", "本地覆盖层文件（YAML / INI）")
	fs.IntVar(&mixedPort, "mixed-port", 7890, "clash.meta mixed-port")
	fs.IntVar(&timeoutSeconds, "timeout", 30, "上游请求超时秒数")
	fs.Usage = func() { fmt.Print(convertUsage) }

	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(urls) == 0 && len(files) == 0 {
		fs.Usage()
		return fmt.Errorf("至少需要一个 -url 或 -file")
	}

	params, err := buildCLIParams(list, format, urls, include, exclude, types, rename, emoji, files, headers, sortKey, pipeline)
	if err != nil {
		return err
	}

	if overrideFile != "" {
		// 覆盖层在参数校验之后、拉订阅之前加载：文件写错（未知段落、重复定义、
		// 非法正则）当场报，不浪费一轮上游请求。
		ov, err := config.LoadOverrideFile(overrideFile)
		if err != nil {
			return err
		}
		params.Override = ov
		fmt.Fprintf(os.Stderr, "覆盖层: %s（%d 组 / %d 规则项 / %d 节点项）\n",
			overrideFile, len(ov.Groups),
			len(ov.Rules.Prepend)+len(ov.Rules.Append), len(ov.Proxies))
	}

	// 先把参数校验做完再动网络：格式、正则、排序键写错了就该立刻报，不能白拉一遍订阅。
	// Engine.Run 内部还会再编一次管线，这点重复开销可以忽略。
	if _, err := convert.BuildPipeline(params); err != nil {
		return err
	}
	if list {
		// -list 决定输出形态，优先级高于 -format；两者都给时不校验 format，避免误报。
		if format != "" {
			fmt.Fprintf(os.Stderr, "提示: -list 已生效，忽略 -format %q\n", format)
		}
	} else if format != "" {
		if _, err := registry.Lookup(format, clashmeta.Options{MixedPort: mixedPort}); err != nil {
			return err
		}
	}

	timeout := time.Duration(timeoutSeconds) * time.Second

	fetcher, err := newFetcher(cacheDir, timeout)
	if err != nil {
		return err
	}

	// 配置模板在拉订阅之前就要读完，所以上下文在这里建。
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var baseTpl *config.Template
	if configFile != "" {
		// 带 :// 的当远程模板拉回来，否则按本地路径读。与 HTTP API 的 config 参数
		// 同一套语义，命令行里直接贴一个 GitHub 链接就能用。
		baseTpl, err = config.LoadSource(ctx, configFile,
			config.HTTPLoader(&http.Client{Timeout: timeout}, nil))
		if err != nil {
			return fmt.Errorf("加载配置模板 %s: %w", configFile, err)
		}
		fmt.Fprintf(os.Stderr, "配置模板: %s\n", baseTpl.Origin())
	}

	opts := clashmeta.Options{MixedPort: mixedPort}
	engine := &convert.Engine{
		Fetcher:     fetcher,
		RendererFor: func(target string) (render.Renderer, error) { return registry.Lookup(target, opts) },
		Profile: render.Profile{
			Name:    name,
			Version: buildinfo.Full(),
		},
		BaseTemplate: func() *config.Template { return baseTpl },
		LoadTemplate: func(ctx context.Context, ref string) (*config.Template, error) {
			// 超时跟着 -timeout 走；配置体积上限在 HTTPLoader 内部。
			return config.LoadSource(ctx, ref, config.HTTPLoader(&http.Client{Timeout: timeout}, nil))
		},
	}

	start := time.Now()
	result, err := engine.Run(ctx, params)
	if err != nil {
		return err
	}
	if len(result.Failures) > 0 {
		fmt.Fprintf(os.Stderr, "警告: %d 个来源失败\n  %s\n",
			len(result.Failures), strings.Join(result.Failures, "\n  "))
	}
	if len(result.Warnings) > 0 {
		// 两类都算：模板里不认识的分组/规则定义被忽略，和渲染阶段单个节点降级。
		// 前一类更致命——产物会是"节点齐、规则空"，所以不能省这条提示。
		fmt.Fprintf(os.Stderr, "提示: 跳过/忽略 %d 项\n  %s\n",
			len(result.Warnings), strings.Join(result.Warnings, "\n  "))
	}
	fmt.Fprintf(os.Stderr, "管线: %d -> %d 节点\n", result.RawNodes, result.Nodes)

	var dest io.Writer = os.Stdout
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		dest = f
	}
	if _, err := dest.Write(result.Body); err != nil {
		return err
	}
	if out != "" {
		fmt.Fprintf(os.Stderr, "已写入 %s\n", out)
	}
	fmt.Fprintf(os.Stderr, "完成: %s, %d 字节, %d 节点, %v\n",
		result.Format, len(result.Body), result.Nodes, time.Since(start).Round(time.Millisecond))
	return nil
}

// buildCLIParams 把命令行 flag 归一成 query 参数形状，再走与 HTTP 入口同一条解析路径。
// 这样才能让 INI 模板参与"参数 < 命令行"的覆盖合并；直接填结构体的话 Raw 是空的，
// 合并时会把命令行传入的设置静默丢掉。
func buildCLIParams(list bool, format string, urls, include, exclude, types, rename, emoji, files, headers multiFlag, sortKey, pipeline string) (convert.Params, error) {
	raw := map[string][]string{}
	add := func(key string, vals ...string) {
		for _, v := range vals {
			if v != "" {
				raw[key] = append(raw[key], v)
			}
		}
	}

	add("target", format)
	// -list 转成 query 参数走同一条解析路径，这样模板里的 list 也能与命令行合并。
	if list {
		add("list", "1")
	}
	add("url", urls...)
	// 一次 flag 里写多个值时按逗号/分号拆开，与 query 参数的形状保持一致。
	for _, v := range include {
		add("include", convert.SplitMulti([]string{v}, ",", ";")...)
	}
	for _, v := range exclude {
		add("exclude", convert.SplitMulti([]string{v}, ",", ";")...)
	}
	for _, v := range types {
		add("type", convert.SplitMulti([]string{v}, ",", ";")...)
	}
	// rename / emoji 先逐条校验，配置写错在拉订阅之前就能报出来。
	for _, v := range rename {
		pair, err := parsePair("rename", v)
		if err != nil {
			return convert.Params{}, err
		}
		add("rename", pair.Pattern+","+pair.Replacement)
	}
	for _, v := range emoji {
		pair, err := parsePair("emoji", v)
		if err != nil {
			return convert.Params{}, err
		}
		add("emoji", pair.Pattern+","+pair.Replacement)
	}
	if sortKey != "" {
		add("sort", strings.ToLower(strings.TrimSpace(sortKey)))
	}
	// 命名管线走与 HTTP 入口同一个参数名（pipeline=），模板里的 [pipeline:<名>]
	// 因此对 CLI 与 /sub 同样可选中。
	add("pipeline", strings.TrimSpace(pipeline))

	params, err := convert.ParseMulti(raw)
	if err != nil {
		return convert.Params{}, err
	}
	// 这两项不是 query 参数，ParseMulti 不负责，单独带上。
	params.Files = files
	params.Headers = headerMap(headers)
	if params.Target == "" {
		params.Target = "clash.meta"
	}
	return params, nil
}

// parsePair 把 "正则,替换" 拆成 RegexPair。正则原文保留（空格有语义），
// 没有逗号时替换为空串，语义是"仅保留匹配"。
func parsePair(kind, raw string) (transform.RegexPair, error) {
	pat, repl, _ := strings.Cut(raw, ",")
	if strings.TrimSpace(pat) == "" {
		return transform.RegexPair{}, fmt.Errorf("%s 参数 %q 缺少正则", kind, raw)
	}
	pair := transform.RegexPair{Pattern: pat, Replacement: repl}
	if _, err := transform.NewRename([]transform.RegexPair{pair}); err != nil {
		return transform.RegexPair{}, fmt.Errorf("%s: %w", kind, err)
	}
	return pair, nil
}

// newFetcher 按命令行参数构造 Fetcher；cacheDir 为空时不做磁盘缓存。
func newFetcher(cacheDir string, timeout time.Duration) (*fetch.Fetcher, error) {
	opts := []fetch.Option{fetch.WithConfig(fetch.Config{
		UserAgent: "SubForge/" + buildinfo.Version + " (+https://github.com/tktogether/subforge)",
		Timeout:   timeout,
		Parallel:  8,
		Retries:   2,
		Backoff:   250 * time.Millisecond,
		MaxBody:   64 << 20,
		Redirects: 5,
	})}
	if cacheDir != "" {
		cache, err := fetch.Open(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("缓存目录 %s: %w", cacheDir, err)
		}
		opts = append(opts, fetch.WithCache(cache))
	}
	return fetch.New(opts...), nil
}

// headerMap 把 "K: V" 形式的重复 flag 转成请求头 map。
func headerMap(list multiFlag) map[string]string {
	if len(list) == 0 {
		return nil
	}
	out := make(map[string]string, len(list))
	for _, h := range list {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "忽略非法 -header %q（应为 \"K: V\"）\n", h)
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}
