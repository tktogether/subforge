// Package buildinfo 保存构建期注入的版本信息。
package buildinfo

import "fmt"

// 通过 -ldflags -X 注入，默认值为源码构建。
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Full 返回单行版本描述。
func Full() string {
	return fmt.Sprintf("subforge %s (commit %s, built %s)", Version, Commit, Date)
}
