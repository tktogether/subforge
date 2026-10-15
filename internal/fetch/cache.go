// Package fetch 负责订阅与规则源的拉取：并发控制、失败重试、条件请求（ETag/Last-Modified）
// 与磁盘缓存。对齐设计书 §4 Fetcher 一节：并行拉取、超时重试、ETag 缓存。
package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache 是按 URL 分片的磁盘缓存（元数据 JSON + 原文 body）。
type Cache struct {
	Dir string
	mu  sync.Mutex
}

type cacheMeta struct {
	ETag     string    `json:"etag,omitempty"`
	Modified time.Time `json:"modified,omitempty"`
}

// Open 打开（必要时创建）缓存目录，返回 Cache。
func Open(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建缓存目录 %s: %w", dir, err)
	}
	return &Cache{Dir: dir}, nil
}

func (c *Cache) key(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])
}

func (c *Cache) metaPath(url string) string {
	return filepath.Join(c.Dir, c.key(url)+".meta.json")
}

func (c *Cache) contentPath(url string) string {
	return filepath.Join(c.Dir, c.key(url)+".body")
}

// Meta 返回缓存的条件请求标识；缺失或损坏时返回空值。
func (c *Cache) Meta(url string) (string, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := os.ReadFile(c.metaPath(url))
	if err != nil {
		return "", time.Time{}
	}
	var m cacheMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return "", time.Time{}
	}
	return m.ETag, m.Modified
}

// Content 返回缓存原文；缺失时返回 error。
func (c *Cache) Content(url string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return os.ReadFile(c.contentPath(url))
}

// Save 写入缓存原文与元数据（内容先行、元数据后写，避免只读到条件标识）。
func (c *Cache) Save(url string, body []byte, meta cacheMeta) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.WriteFile(c.contentPath(url), body, 0o644); err != nil {
		return fmt.Errorf("写入缓存原文: %w", err)
	}
	m, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.metaPath(url), m, 0o644); err != nil {
		return fmt.Errorf("写入缓存元数据: %w", err)
	}
	return nil
}

// ClearMeta 删除条件请求标识（保留原文）。用于 304 命中但缓存原文缺失的自愈场景。
func (c *Cache) ClearMeta(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = os.Remove(c.metaPath(url))
}
