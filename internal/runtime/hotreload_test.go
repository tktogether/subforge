package runtime

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSnapshot_Basic(t *testing.T) {
	s := NewSnapshot(1)
	if s.Load() != 1 {
		t.Fatalf("初始值 = %d", s.Load())
	}
	if old := s.Store(2); old != 1 {
		t.Errorf("Store 应返回旧值 %d", old)
	}
	if s.Load() != 2 {
		t.Errorf("新值 = %d", s.Load())
	}
}

func TestSnapshot_StructValue(t *testing.T) {
	type opts struct{ Port int }
	s := NewSnapshot(opts{Port: 7890})
	s.Store(opts{Port: 1080})
	if got := s.Load(); got.Port != 1080 {
		t.Errorf("结构值快照 = %+v", got)
	}
}

// TestSnapshot_Concurrent 读端永不加锁，这里用两个写者压着读，确认没有数据竞争。
func TestSnapshot_Concurrent(t *testing.T) {
	s := NewSnapshot(0)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(seed int) {
			defer wg.Done()
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				s.Store(seed*10000 + n)
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = s.Load()
		}
	}()
	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// waitCalls 等到计数器达到至少 want 次。
func waitCalls(t *testing.T, n *int32, want int32, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for atomic.LoadInt32(n) < want {
		if time.Now().After(deadline) {
			t.Fatalf("等待回调达到 %d 次，实际 %d", want, atomic.LoadInt32(n))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func write(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWatch_InitialAndChange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.ini")
	write(t, p, "[common]\ntoken = one\n")

	got := make(chan string, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, p, 10*time.Millisecond, func(b []byte) error {
			got <- string(b)
			return nil
		})
	}()

	first := <-got
	if first != "[common]\ntoken = one\n" {
		t.Fatalf("首次回调内容 = %q", first)
	}

	time.Sleep(30 * time.Millisecond)
	if len(got) != 0 {
		t.Fatalf("内容未变却触发了回调")
	}

	write(t, p, "[common]\ntoken = two\n")
	select {
	case second := <-got:
		if second != "[common]\ntoken = two\n" {
			t.Errorf("变更回调内容 = %q", second)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("修改文件后未触发回调")
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("Watch 退出: %v", err)
	}
}

// TestWatch_KeepsWatchingAfterMissingFile 文件被临时移走再回来时不应崩、也不该漏掉变更。
func TestWatch_KeepsWatchingAfterMissingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.ini")
	write(t, p, "a=1\n")

	var calls int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, p, 10*time.Millisecond, func([]byte) error {
			atomic.AddInt32(&calls, 1)
			return nil
		})
	}()

	waitCalls(t, &calls, 1, 2*time.Second)
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("文件缺失不应触发回调，实际 %d 次", n)
	}
	write(t, p, "a=2\n")
	waitCalls(t, &calls, 2, 2*time.Second)
	cancel()
	<-done
}

func TestWatch_DefaultInterval(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.ini")
	write(t, p, "x=1\n")
	var n int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, p, 0, func([]byte) error {
			atomic.AddInt32(&n, 1)
			return nil
		})
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Watch 退出: %v", err)
	}
	if n != 1 {
		t.Errorf("interval<=0 时应使用默认值并回调一次，实际 %d 次", n)
	}
}

func TestDigestOf(t *testing.T) {
	a := DigestOf([]byte("abc"))
	b := DigestOf([]byte("abc"))
	c := DigestOf([]byte("abd"))
	if a != b {
		t.Errorf("相同内容摘要不同: %s %s", a, b)
	}
	if a == c {
		t.Errorf("不同内容摘要相同: %s", a)
	}
}
