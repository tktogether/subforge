// Package config 从 INI 模板里提取服务运行期设置。
package config

import (
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/template"
)

// 默认值：与命令行参数的默认值保持一致。
const (
	DefaultMixedPort = 7890
)

// Settings 是从 INI 配置提取出的运行期设置。
type Settings struct {
	// Token 访问令牌，空串表示不鉴权。
	Token string
	// MixedPort clash.meta 输出的 mixed-port。
	MixedPort int
	// Name 写入产物注释头的配置名。
	Name string
}

// Default 返回全部取默认值的设置。
func Default() Settings { return Settings{MixedPort: DefaultMixedPort} }

// sectionSubForge 是本服务自己的设置段落。
//
// 故意不叫 [common]：那是 subconverter 模板语义的段落，里面的 key 各有各的含义。
// [common] 只作为 token 的兼容回退读取。
const sectionSubForge = "subforge"

// FromINI 从解析好的 INI 提取设置。
// defaults 是"配置里没有写时用哪个值"的基准——通常是命令行参数。
func FromINI(cfg *template.Config, defaults Settings) Settings {
	out := defaults
	if cfg == nil {
		return out
	}

	if section := cfg.Lookup(sectionSubForge); section != nil {
		if v, ok := section.Get("token"); ok {
			out.Token = strings.TrimSpace(v.Raw)
		}
		if v, ok := section.Get("mixed_port"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v.Raw)); err == nil && n > 0 && n < 65536 {
				out.MixedPort = n
			}
		}
		if v, ok := section.Get("name"); ok {
			out.Name = strings.TrimSpace(v.Raw)
		}
	}
	// 兼容写法：subconverter 习惯把 token 放在 [common]。
	if out.Token == "" {
		if v, ok := cfg.Get("common", "token"); ok {
			out.Token = strings.TrimSpace(v.Raw)
		}
	}
	return out
}
