package subforge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ScriptEnabledKey 是脚本子系统的安全开关（默认关闭）。
const ScriptEnabledKey = "script.enabled"

// ScriptResolver 实现 config.ScriptResolver：从 scripts 表按名取源码。
// 本包只依赖 store 的鸭子类型，不直接 import internal/config。
type ScriptResolver struct {
	m *Manager
}

// ScriptResolver 返回绑定到当前 Manager 的脚本解析器。
func (m *Manager) ScriptResolver() *ScriptResolver {
	return &ScriptResolver{m: m}
}

// ScriptsEnabled 报告 script.enabled 是否开启。
// 值等于 "1"/"true"/"yes"/"on"（大小写不敏感、去空白）才算开启；读取失败按关闭处理。
func (r *ScriptResolver) ScriptsEnabled() bool {
	v, err := r.m.GetConfig(context.Background(), ScriptEnabledKey)
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// ResolveScript 按名返回脚本源码；未命中或已停用分别报错。
func (r *ScriptResolver) ResolveScript(name string) (string, error) {
	s, err := r.m.DB().GetScriptByName(context.Background(), name)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("脚本 %q 不存在", name)
		}
		return "", err
	}
	if !s.Enabled {
		return "", fmt.Errorf("脚本 %q 已停用", name)
	}
	return s.Code, nil
}
