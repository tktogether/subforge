// Package api — 日志环形缓冲区，供 /api/logs 读取。
package api

import (
	"strings"
	"sync"
	"time"
)

// LogEntry 是一条捕获的日志行。
type LogEntry struct {
	ID   int64  `json:"id"`
	Ts   string `json:"ts"`
	Line string `json:"line"`
}

// LogBuffer 是一个并发安全的日志 ring buffer，只保留最近 max 条。
// 它实现了 io.Writer，可挂到 log.Logger 的 multi-writer 上。
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	nextID  int64
	max     int
}

// NewLogBuffer 构造一个容量为 max 的缓冲区；max<=0 时取默认 500。
func NewLogBuffer(max int) *LogBuffer {
	if max <= 0 {
		max = 500
	}
	return &LogBuffer{max: max}
}

// Write 实现 io.Writer：把一段日志追加进 buffer。log.Logger 每次只写一行
// （带尾部换行），所以这里 TrimRight 一下整行存下来。
func (b *LogBuffer) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\n")
	if strings.TrimSpace(line) == "" {
		return len(p), nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	b.entries = append(b.entries, LogEntry{
		ID:   b.nextID,
		Ts:   time.Now().UTC().Format(time.RFC3339),
		Line: line,
	})
	if len(b.entries) > b.max {
		b.entries = b.entries[len(b.entries)-b.max:]
	}
	return len(p), nil
}

// Since 返回 ID 大于 since 的条目，最多 limit 条，按时间升序。
func (b *LogBuffer) Since(since int64, limit int) []LogEntry {
	if limit <= 0 || limit > b.max {
		limit = 200
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]LogEntry, 0, limit)
	// 从尾部倒查，拿够 limit 条就停，再反转回去。
	buf := make([]LogEntry, 0, limit)
	for i := len(b.entries) - 1; i >= 0; i-- {
		if len(buf) >= limit {
			break
		}
		if b.entries[i].ID > since {
			buf = append(buf, b.entries[i])
		}
	}
	for i := len(buf) - 1; i >= 0; i-- {
		out = append(out, buf[i])
	}
	return out
}

// All 返回全部条目，按时间升序。
func (b *LogBuffer) All() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]LogEntry, len(b.entries))
	copy(out, b.entries)
	return out
}
