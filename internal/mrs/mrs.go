// Package mrs 实现 M3.2 MRS 离线生成：把文本/Clash YAML 规则集转成
// mihomo rule-set 二进制（.mrs），复用内核 rules/provider.ConvertToMrs，
// 保证与 mihomo convert-ruleset 字节级一致。
package mrs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	P "github.com/metacubex/mihomo/constant/provider"
	"github.com/metacubex/mihomo/rules/provider"
)

// DefaultTTL 是磁盘缓存的默认有效期（7 天）。
const DefaultTTL = 7 * 24 * time.Hour

// Generate 从规则集源数据生成 MRS 二进制。
// behavior: "domain" / "ipcidr" / "classical"（classical 无二进制形态，会报错）
// format: "yaml" / "text" / "mrs"（源数据格式）
func Generate(buf []byte, behavior, format string) ([]byte, error) {
	b, err := P.ParseBehavior(behavior)
	if err != nil {
		return nil, fmt.Errorf("解析 behavior %q: %w", behavior, err)
	}
	f, err := P.ParseRuleFormat(format)
	if err != nil {
		return nil, fmt.Errorf("解析 format %q: %w", format, err)
	}
	var out bytes.Buffer
	if err := provider.ConvertToMrs(buf, b, f, &out); err != nil {
		return nil, fmt.Errorf("ConvertToMrs: %w", err)
	}
	return out.Bytes(), nil
}

// GenerateFile 生成 MRS 并写入目标文件。
func GenerateFile(srcPath, dstPath, behavior, format string) error {
	buf, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("读取源文件 %s: %w", srcPath, err)
	}
	out, err := Generate(buf, behavior, format)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("创建目标目录: %w", err)
	}
	if err := os.WriteFile(dstPath, out, 0o644); err != nil {
		return fmt.Errorf("写入目标文件 %s: %w", dstPath, err)
	}
	return nil
}

// CachePath 根据源文件路径与 behavior/format 计算缓存文件路径。
func CachePath(cacheDir, srcPath, behavior, format string) string {
	base := filepath.Base(srcPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s.mrs", name, behavior, format))
}

// CacheValid 判断缓存文件是否在 TTL 内。
func CacheValid(path string, ttl time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return time.Since(info.ModTime()) < ttl
}

// GenerateCached 生成 MRS，命中缓存时直接返回缓存内容。
func GenerateCached(srcPath, cacheDir, behavior, format string, ttl time.Duration) ([]byte, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	cp := CachePath(cacheDir, srcPath, behavior, format)
	if CacheValid(cp, ttl) {
		return os.ReadFile(cp)
	}
	buf, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("读取源文件 %s: %w", srcPath, err)
	}
	out, err := Generate(buf, behavior, format)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err == nil {
		_ = os.WriteFile(cp, out, 0o644)
	}
	return out, nil
}
