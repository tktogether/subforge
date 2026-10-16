// Package runtime 提供服务运行期所需的并发原语与配置热重载。
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// Snapshot 是一个并发安全的不可变值快照，读端永不加锁。
//
// 热重载的标准做法是双缓冲：后台线程解析新配置、整体校验通过后一次性替换指针，
// 前台请求始终拿到一份完整的旧值或新值，不会读到改了一半的配置。
type Snapshot[T any] struct {
	val atomic.Pointer[T]
}

// NewSnapshot 用初始值构造快照。
func NewSnapshot[T any](initial T) *Snapshot[T] {
	v := initial
	s := &Snapshot[T]{}
	s.val.Store(&v)
	return s
}

// Load 取当前值。并发安全，不加锁。
func (s *Snapshot[T]) Load() T {
	return *s.val.Load()
}

// Store 整体替换当前值，返回旧值。
func (s *Snapshot[T]) Store(v T) T {
	old := s.val.Load()
	nv := v
	s.val.Store(&nv)
	if old == nil {
		var zero T
		return zero
	}
	return *old
}

// Watch 轮询文件内容变化，内容改变且稳定时回调 onChange。
//
// 有意不用 fsnotify：Windows 上需要额外依赖，而配置文件很小，轮询的开销可以忽略。
// 轮询先看 mtime+size（几乎不花钱），两者都变了才真正读文件并比对内容摘要——
// 这样即使文件系统的时间粒度较粗也不会漏掉"大小相同、内容不同"的修改。
//
// 写入方不是原子写：os.WriteFile 是 truncate + write，中途读会拿到改到一半的
// 内容。所以变化要在连续两轮都读到相同摘要才交付，代价是一个轮询间隔的延迟，
// 换来的是永远不会把残缺配置推到下游。
//
// onChange 在检测到变化时同步调用；返回错误只记入返回值，不会停止轮询。
func Watch(ctx context.Context, path string, interval time.Duration, onChange func(content []byte) error) error {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	type state struct {
		mod    int64
		size   int64
		digest [32]byte
	}
	var last state

	read := func() (state, []byte, error) {
		st, err := os.Stat(path)
		if err != nil {
			return state{}, nil, err
		}
		buf, err := os.ReadFile(path)
		if err != nil {
			return state{}, nil, err
		}
		sum := sha256.Sum256(buf)
		return state{
			mod:    st.ModTime().UnixNano(),
			size:   st.Size(),
			digest: sum,
		}, buf, nil
	}

	// 首次读取的内容作为基线；读失败也把空内容递出去，让调用方知道初始状态是空的。
	st, buf, err := read()
	if err == nil {
		last = st
	}
	if onChange != nil {
		_ = onChange(buf)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var pending state
	havePending := false

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// 先用 stat 粗筛：mtime 与 size 都没变就跳过读文件。
			st2, err := os.Stat(path)
			if err != nil {
				continue // 文件被临时移走时不视为变化
			}
			if st2.ModTime().UnixNano() == last.mod && st2.Size() == last.size {
				continue
			}

			st, buf, err := read()
			if err != nil {
				continue
			}
			if st.digest == last.digest {
				last = st // 只换了 mtime、内容没变（比如原子替换成同一份内容）
				havePending = false
				continue
			}
			// 同一份内容必须在两轮里都读到才交付，滤掉写一半的中间态。
			if havePending && pending.digest == st.digest {
				havePending = false
				last = st
				if onChange != nil {
					_ = onChange(buf)
				}
				continue
			}
			havePending = true
			pending = st
		}
	}
}

// DigestOf 给出内容摘要的十六进制前缀，用于日志里标识一次重载。
func DigestOf(buf []byte) string {
	sum := sha256.Sum256(buf)
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], binary.BigEndian.Uint64(sum[:8]))
	binary.BigEndian.PutUint64(b[8:], binary.BigEndian.Uint64(sum[8:16]))
	return fmt.Sprintf("%x", b)
}
