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
//
// 先写同目录唯一临时文件再改名，避免写一半崩溃或断电留下半截清单。
// 权限沿用旧文件 mode；新文件保持临时文件的 0600（不再无条件放宽到 0644）。
// 符号链接与只读目录无法原子替换时退回原地写，保留原有部署兼容性。
func SaveManifest(path string, m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest 为空")
	}
	mode := os.FileMode(0o600)
	isSymlink := false
	if linfo, lerr := os.Lstat(path); lerr == nil {
		isSymlink = linfo.Mode()&os.ModeSymlink != 0
		if info, serr := os.Stat(path); serr == nil {
			mode = info.Mode().Perm()
		}
	}
	if old, rerr := os.ReadFile(path); rerr == nil {
		_ = os.WriteFile(path+".bak", old, mode)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	if isSymlink {
		// 符号链接原地写才能保留链接本身；替换 inode 会把它变成普通文件。
		return os.WriteFile(path, b, mode)
	}
	directWrite := func() error {
		if werr := os.WriteFile(path, b, mode); werr != nil {
			return fmt.Errorf("写清单 %s: %w", path, werr)
		}
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		// 目录不可写（单文件 bind mount、只读配置目录等）：退回原地写。
		return directWrite()
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	_ = os.Chmod(tmpName, mode)
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return directWrite()
	}
	return nil
}

// DeriveSourceName 从 URL 推导缺省名称，与 applyDefaults 的规则一致。
func DeriveSourceName(u string) string {
	return urlBasename(u)
}

// ValidateForWrite 复用读取侧校验（≥1 source / URL 非空 / 名称唯一）。
func (m *Manifest) ValidateForWrite() error {
	return m.validate()
}
