// diff 命令：预览 override 覆盖层对模板默认值的改动（M2 §1.2 分层覆盖 DSL）。
//
// 不拉订阅、不跑管线，只把"模板默认 + 覆盖层"合并结果打印出来，
// 让用户在真正转换前先看清每一处改动。回滚即删 override 条目。
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render"
)

const diffUsage = `subforge diff - 预览覆盖层对模板默认值的改动

用法:
  subforge diff -override <覆盖层文件> [-config <INI 模板>] [选项]

参数:
  -override  覆盖层文件（YAML / INI），必填
  -config    subconverter 风格 INI 模板；不给时只显示覆盖层自身声明

输出:
  代理组  改 / 新增 / 未变（默认隐藏未变组，-v 显示）
  规则    prepend（+P 前缀）/ append（+A 前缀）
  节点    server / port 标量替换
`

func cmdDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	var configFile, overrideFile string
	verbose := fs.Bool("v", false, "显示未改动的代理组")
	fs.StringVar(&configFile, "config", "", "subconverter 风格 INI 模板（覆盖的基底）")
	fs.StringVar(&overrideFile, "override", "", "覆盖层文件（YAML / INI）")
	fs.Usage = func() { fmt.Print(diffUsage) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if overrideFile == "" {
		fs.Usage()
		return fmt.Errorf("需要 -override")
	}

	var (
		baseGroups []render.Group
		baseRules  []string
	)
	if configFile != "" {
		tpl, err := loadTemplateForDiff(configFile)
		if err != nil {
			return err
		}
		sem, err := tpl.Semantics()
		if err != nil {
			return err
		}
		base, err := convert.ApplyTemplate(sem, convert.Params{})
		if err != nil {
			return err
		}
		baseGroups, baseRules = base.Groups, base.Rules
	}

	ov, err := config.LoadOverrideFile(overrideFile)
	if err != nil {
		return err
	}

	mergedGroups := config.MergeGroups(baseGroups, ov.ToGroups())
	mergedRules := config.MergeRules(baseRules, ov.Rules.Prepend, ov.Rules.Append)

	printDiff(baseGroups, mergedGroups, ov, *verbose)
	printRuleDiff(baseRules, mergedRules, ov)
	printProxyDiff(ov)
	return nil
}

func loadTemplateForDiff(path string) (*config.Template, error) {
	// 带 :// 的当远程模板拉回来，否则按本地路径读；与 convert 命令同一套语义。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tpl, err := config.LoadSource(ctx, path,
		config.HTTPLoader(&http.Client{Timeout: 30 * time.Second}, nil))
	if err != nil {
		return nil, fmt.Errorf("加载配置模板 %s: %w", path, err)
	}
	return tpl, nil
}

func printDiff(base, merged []render.Group, ov config.Override, verbose bool) {
	fmt.Println("== 代理组 ==")
	baseIdx := map[string]render.Group{}
	for _, g := range base {
		baseIdx[g.Name] = g
	}
	mergedIdx := map[string]render.Group{}
	for _, g := range merged {
		mergedIdx[g.Name] = g
	}
	changed := 0
	for _, g := range ov.Groups {
		bg, ok := baseIdx[g.Name]
		mg := mergedIdx[g.Name]
		if !ok {
			fmt.Printf("  [新] %-14s type=%q strategy=%q proxies=%v\n",
				g.Name, firstNonEmpty(g.Type, mg.Type), g.Strategy, mg.Proxies)
			changed++
			continue
		}
		parts := groupChanges(bg, g, mg)
		if len(parts) == 0 {
			if verbose {
				fmt.Printf("  [同] %s\n", g.Name)
			}
			continue
		}
		fmt.Printf("  [改] %-14s %s\n", g.Name, strings.Join(parts, "; "))
		changed++
	}
	for name, g := range mergedIdx {
		if _, inBase := baseIdx[name]; inBase {
			continue
		}
		if isGroupInOverrides(name, ov) {
			continue // 已作为 [新] 打印
		}
		// 模板有、覆盖层未动的组不算改动；只有 verbose 才列出。
		if verbose {
			fmt.Printf("  [同] %s\n", name)
		}
		_ = g
	}
	if changed == 0 {
		fmt.Println("  （覆盖层未改动任何代理组）")
	}
}

// groupChanges 对比 base 与 override 声明的差异（只看 override 写了的字段）。
func groupChanges(base render.Group, over config.GroupOverride, merged render.Group) []string {
	var parts []string
	if over.Type != "" && over.Type != base.Type {
		parts = append(parts, fmt.Sprintf("type %q → %q", base.Type, over.Type))
	}
	if over.Strategy != "" && over.Strategy != base.Strategy {
		parts = append(parts, fmt.Sprintf("strategy %q → %q", base.Strategy, over.Strategy))
	}
	if over.Test != "" && over.Test != base.Test {
		parts = append(parts, fmt.Sprintf("test %q → %q", base.Test, over.Test))
	}
	if over.Lazy && !base.Lazy {
		parts = append(parts, "lazy → true")
	}
	// proxies 是追加语义：列出 merged 里相对 base 新增的节点。
	seen := map[string]bool{}
	for _, p := range base.Proxies {
		seen[p] = true
	}
	var added []string
	for _, p := range merged.Proxies {
		if !seen[p] {
			added = append(added, p)
		}
	}
	if len(added) > 0 {
		sort.Strings(added)
		parts = append(parts, fmt.Sprintf("proxies +%s", strings.Join(added, ", ")))
	}
	return parts
}

func isGroupInOverrides(name string, ov config.Override) bool {
	for _, g := range ov.Groups {
		if g.Name == name {
			return true
		}
	}
	return false
}

func printRuleDiff(base, merged []string, ov config.Override) {
	fmt.Println("== 规则 ==")
	changed := false
	for _, r := range ov.Rules.Prepend {
		fmt.Printf("  +P %s\n", r)
		changed = true
	}
	for _, r := range ov.Rules.Append {
		fmt.Printf("  +A %s\n", r)
		changed = true
	}
	if !changed {
		fmt.Println("  （覆盖层未增删规则）")
		return
	}
	fmt.Printf("  合并后共 %d 条（基础 %d + prepend %d + append %d）\n",
		len(merged), len(base), len(ov.Rules.Prepend), len(ov.Rules.Append))
}

func printProxyDiff(ov config.Override) {
	fmt.Println("== 节点 ==")
	changed := false
	for _, p := range ov.Proxies {
		if p.Server == "" && p.Port == "" {
			continue
		}
		var parts []string
		if p.Server != "" {
			parts = append(parts, fmt.Sprintf("server → %s", p.Server))
		}
		if p.Port != "" {
			parts = append(parts, fmt.Sprintf("port → %s", p.Port))
		}
		fmt.Printf("  [改] /%s/ %s\n", p.Name, strings.Join(parts, "; "))
		changed = true
	}
	if !changed {
		fmt.Println("  （覆盖层未改节点）")
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
