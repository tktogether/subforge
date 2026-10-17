// Package api — 转换历史环形缓冲区，供 Dashboard /api/history 使用。
package api

import (
	"sync"
	"time"
)

// HistoryEntry 是一次转换的摘要记录。
type HistoryEntry struct {
	ID       int64    `json:"id"`
	At       string   `json:"at"`
	Target   string   `json:"target"`
	Format   string   `json:"format"`
	Nodes    int      `json:"nodes"`
	RawNodes int      `json:"rawNodes"`
	Failures []string `json:"failures"`
	Warnings []string `json:"warnings"`
	Cached   int      `json:"cached"`
	URLCount int      `json:"urlCount"`
}

// HistoryBuffer 是一个并发安全的历史 ring buffer，只保留最近 max 条。
type HistoryBuffer struct {
	mu      sync.RWMutex
	entries []HistoryEntry
	nextID  int64
	max     int
}

// NewHistoryBuffer 构造一个容量为 max 的缓冲区；max<=0 时取默认 20。
func NewHistoryBuffer(max int) *HistoryBuffer {
	if max <= 0 {
		max = 20
	}
	return &HistoryBuffer{max: max}
}

// Record 追加一条历史记录。
func (b *HistoryBuffer) Record(e HistoryEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	e.ID = b.nextID
	e.At = time.Now().UTC().Format(time.RFC3339)
	b.entries = append(b.entries, e)
	if len(b.entries) > b.max {
		b.entries = b.entries[len(b.entries)-b.max:]
	}
}

// Recent 返回最近 n 条，最新的在前。
func (b *HistoryBuffer) Recent(n int) []HistoryEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if n <= 0 || n > len(b.entries) {
		n = len(b.entries)
	}
	out := make([]HistoryEntry, n)
	for i := 0; i < n; i++ {
		out[i] = b.entries[len(b.entries)-1-i]
	}
	return out
}

// Stats 返回累计统计：转换总数、缓存命中数、总 URL 数。
func (b *HistoryBuffer) Stats() (total, cachedHits, urlTotal int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	total = len(b.entries)
	for _, e := range b.entries {
		cachedHits += e.Cached
		urlTotal += e.URLCount
	}
	return
}
