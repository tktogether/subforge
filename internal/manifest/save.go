package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadManifestRaw 只做字节级反序列化：不校验、不补默认值、不改写 output.path。
// 专供「读原始结构 → 修改 → 写回」使用，避免 LoadManifest 的绝对路径固化。
func LoadManifestRaw(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("解析 manifest %s: %w", path, err)
	}
	return &m, nil
}

// SaveManifest 把 manifest 写回 path；已存在旧文件时先备份 <path>.bak。
func SaveManifest(path string, m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest 为空")
	}
	if old, rerr := os.ReadFile(path); rerr == nil {
		_ = os.WriteFile(path+".bak", old, 0o644)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// DeriveSourceName 从 URL 推导缺省名称，与 applyDefaults 的规则一致。
func DeriveSourceName(u string) string {
	return urlBasename(u)
}

// ValidateForWrite 复用读取侧校验（≥1 source / URL 非空 / 名称唯一）。
func (m *Manifest) ValidateForWrite() error {
	return m.validate()
}
