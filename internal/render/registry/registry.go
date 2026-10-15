// Package registry 按 subconverter 风格的 target 名取得渲染器。
//
// 渲染器注册表必须与 render 包分离：render/clashmeta 反向 import 了 render，
// 放在 render 里会构成循环依赖。
package registry

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/lineform"
	"github.com/tktogether/subforge/internal/render/mellow"
	"github.com/tktogether/subforge/internal/render/sharelink"
	"github.com/tktogether/subforge/internal/render/singbox"
	"github.com/tktogether/subforge/internal/render/sssub"
)

// ErrUnsupportedFormat 表示请求的 target 没有对应的渲染器。
// 包装在错误链里，调用方可以据此区分"格式没实现"与"上游拉取失败"。
var ErrUnsupportedFormat = errors.New("不支持的输出格式")

// Lookup 按 target 名取得渲染器；空 target 走默认（clash.meta）。
// 别名一律归一到规范渲染器名，X-Convert-Format 与文件名都从 Name() 派生，
// 因此同一个格式无论用哪个别名请求，产物头与落盘名都一致。
func Lookup(target string, clashOpts clashmeta.Options) (render.Renderer, error) {
	t := strings.ToLower(strings.TrimSpace(target))
	switch {
	case render.IsClashYAMLTarget(t):
		// 序列化格式由 target 决定，不由调用方传入的 Options 决定——
		// 复用同一个 Options 结构体时才不会串到 JSON 输出。
		clashOpts.JSON = false
		return clashmeta.New(clashOpts), nil
	case render.IsClashJSONTarget(t):
		// 同一棵构好的 yaml.Node 树换成 JSON 编码，配置内容不重复实现。
		clashOpts.JSON = true
		return clashmeta.New(clashOpts), nil
	case t == "list", t == "sub", t == "subscription":
		// 分享链接清单：filter/rename/sort 全部照常生效，只是输出形态换成纯链接。
		return sharelink.New("list", sharelink.Options{}), nil
	case render.IsMixedTarget(t):
		// Base64 混合订阅。renderer 名一律用规范名（mixed），
		// 不让 Name() 随调用方式变化：X-Convert-Format 与文件名都从它派生。
		return sharelink.New(render.LinkTargets[t].Renderer, sharelink.Options{Base64: true}), nil
	case render.IsLineFormTarget(t):
		renderer, _ := render.LineFormRenderer(t)
		return lineform.New(lineFormFormat(renderer), lineform.Options{}), nil
	case render.IsSingboxTarget(t):
		// sing-box 是独立内核，出站 schema 与 Clash 不同，自成渲染器。
		return singbox.New(singbox.DefaultOptions()), nil
	case render.IsTargetAlias(t):
		// 别名归一到规范渲染器名：X-Convert-Format 与落盘名都从 Name() 派生，
		// 因此不管用哪个写法请求，产物头与文件名都一致。
		return aliasRenderer(t)
	default:
		return nil, fmt.Errorf("%w %q（目前支持: %s）",
			ErrUnsupportedFormat, target, strings.Join(Available(), ", "))
	}
}

// aliasRenderer 按规范渲染器名构造 TargetAliases 那批 target 的渲染器。
// TargetAliases 只记名，不构造对象——渲染器的构造集中在这一处，
// 新增格式时不用去改 Lookup 的 switch。
func aliasRenderer(t string) (render.Renderer, error) {
	canonical, _ := render.TargetAliases[t]
	switch canonical {
	case "ss-sub":
		// SS-android 的两种入口格式共用一个渲染器，base64 与否由别名决定。
		opts := sssub.DefaultOptions()
		if t == "ss-sub-b64" {
			opts.Base64 = true
		}
		return sssub.New(opts), nil
	case "mellow":
		return mellow.New(mellow.DefaultOptions()), nil
	default:
		return nil, fmt.Errorf("%w %q", ErrUnsupportedFormat, t)
	}
}

// lineFormFormat 按规范渲染器名构造对应格式的 Format 描述。
// 渲染器与格式解耦在这里收口：registry 只做名单判断，语法细节留在 lineform 包内。
func lineFormFormat(renderer string) lineform.Format {
	switch renderer {
	case "surge":
		return lineform.SurgeFormat()
	case "quan":
		return lineform.QuanFormat()
	case "quanx":
		return lineform.QuanxFormat()
	case "loon":
		return lineform.LoonFormat()
	case "surfboard":
		return lineform.SurfboardFormat()
	default:
		return lineform.SurgeFormat()
	}
}

// Available 返回当前支持的 target 名，供错误信息与文档使用。
// 头部手写一份常用写法（顺序有意义），分享链接与配置形态两族各自从 render 的
// 真源表展开，这样新增 target 时错误信息不会漏。
func Available() []string {
	names := []string{
		"clash.meta", "surge", "quan", "quanx", "loon", "surfboard", "mellow",
		"list", "mixed", "ss", "ssd", "shadowsocks", "ssr", "shadowsocksr",
		"trojan", "v2ray", "sing-box", "singbox",
		"ss-sub", "ss-sub-json", "ss-sub-b64", "ssjson",
	}
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		seen[n] = true
	}
	for name := range render.LinkTargets {
		if name == "list" || name == "sub" || name == "subscription" {
			continue
		}
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for _, lt := range render.LineFormTargets {
		candidates := append([]string{lt.Renderer}, lt.Aliases...)
		for _, n := range candidates {
			if !seen[n] {
				names = append(names, n)
				seen[n] = true
			}
		}
	}
	for n := range render.ClashJSONTargets {
		if !seen[n] {
			names = append(names, n)
			seen[n] = true
		}
	}
	for n := range render.TargetAliases {
		if !seen[n] {
			names = append(names, n)
			seen[n] = true
		}
	}
	return names
}
