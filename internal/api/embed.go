package api

import (
	"embed"
	"io/fs"
)

// webEmbed 内嵌 Vite 构建产物。
//
// all: 前缀不可省：Vite 会产出 _plugin-vue_export-helper-*.js 这类以 "_" 开头的文件，
// 默认规则会静默跳过，表现为运行时 404 而编译期毫无提示。
// 目录不存在时 go:embed 是编译错误，所以 internal/api/web/dist/.gitkeep 必须入库。
//
//go:embed all:web/dist
var webEmbed embed.FS

// webFS 是 web/dist 目录的子文件系统（嵌入二进制），供静态资源与 SPA fallback 使用。
var webFS fs.FS

func init() {
	sub, err := fs.Sub(webEmbed, "web/dist")
	if err != nil {
		panic("api: 嵌入 web/dist 资源失败: " + err.Error())
	}
	webFS = sub
}
