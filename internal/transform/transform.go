// Package transform 是节点列表的过滤器与操作器管线，语义对齐 Sub-Store 的
// filter（regex/region/type）→ op（dedupe/rename/emoji/sort/remove）两段式配置。
//
// 约定：
//   - Stage 是纯变换：接收 NodeList、返回新的 NodeList，不修改入参。
//   - 改名必须写回 Raw（原始 config map 是唯一事实源，见 docs/adr/0001），
//     统一走 withName，避免只改派生视图造成渲染结果与解析产物不一致。
//   - 正则在构造期编译并返回错误，运行期不再碰正则语法错误。
package transform

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tktogether/subforge/internal/model"
)

// Stage 是管线中的一步。
type Stage interface {
	Apply(ns model.NodeList) (model.NodeList, error)
}

// Pipeline 按声明顺序依次执行。
type Pipeline []Stage

// Apply 执行整条管线；任一步失败即中止并带上步序号与类型，便于定位配置问题。
func (p Pipeline) Apply(ns model.NodeList) (model.NodeList, error) {
	for i, s := range p {
		if s == nil {
			continue
		}
		var err error
		ns, err = s.Apply(ns)
		if err != nil {
			return nil, fmt.Errorf("管线第 %d 步（%T）: %w", i+1, s, err)
		}
	}
	return ns, nil
}

// withName 改名并重建只读派生视图。
func withName(n model.Node, name string) model.Node {
	if n.Raw == nil {
		n.Raw = map[string]any{}
	}
	n.Raw["name"] = name
	return model.NodeFromRaw(n.Raw, n.Source)
}

// compilePatterns 编译正则列表；仅对"是否为空"做 TrimSpace 判断，
// 正则本身按原文编译——正则里的空格是有语义的（如 ` - 10Gbps` 里的分隔空格），
// 配置解析层（INI）负责去掉值两侧多余空白。
func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("正则 %q 编译失败: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

func matchAny(pats []*regexp.Regexp, s string) bool {
	for _, re := range pats {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
