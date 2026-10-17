package api

import (
	"embed"
	"io/fs"
)

//go:embed web
var webEmbed embed.FS

// webFS 是 web/ 目录的子文件系统（嵌入二进制），供页面与静态资源服务使用。
var webFS fs.FS

func init() {
	sub, err := fs.Sub(webEmbed, "web")
	if err != nil {
		panic("api: 嵌入 web 资源失败: " + err.Error())
	}
	webFS = sub
}
