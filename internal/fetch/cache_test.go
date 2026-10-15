package fetch

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_SaveAndRead(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/sub?token=abc"
	body := []byte("vmess://abc\nvless://xyz\n")
	modified := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)

	if err := c.Save(url, body, cacheMeta{ETag: `"r2"`, Modified: modified}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	etag, mod := c.Meta(url)
	if etag != `"r2"` {
		t.Fatalf("ETag=%q", etag)
	}
	if !mod.Equal(modified) {
		t.Fatalf("Modified=%v, want %v", mod, modified)
	}
	got, err := c.Content(url)
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("Content 不一致: %q", got)
	}
}

func TestCache_Missing(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if etag, mod := c.Meta("https://example.com/never"); etag != "" || !mod.IsZero() {
		t.Fatalf("未写入的缓存不应有元数据: %q %v", etag, mod)
	}
	if _, err := c.Content("https://example.com/never"); err == nil {
		t.Fatal("未写入的缓存不应有原文")
	}
}

func TestCache_DifferentURLs(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, b := "https://example.com/sub?a=1", "https://example.com/sub?a=2"
	if err := c.Save(a, []byte("AAA"), cacheMeta{ETag: `"a"`}); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(b, []byte("BBB"), cacheMeta{ETag: `"b"`}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		url  string
		etag string
		body string
	}{
		{a, `"a"`, "AAA"}, {b, `"b"`, "BBB"},
	} {
		if etag, _ := c.Meta(tc.url); etag != tc.etag {
			t.Errorf("%s: ETag=%q, want %q", tc.url, etag, tc.etag)
		}
		got, err := c.Content(tc.url)
		if err != nil || string(got) != tc.body {
			t.Errorf("%s: Content=%q err=%v", tc.url, got, err)
		}
	}
}

// TestCache_ClearMeta 覆盖 304 自愈场景：清掉条件标识但保留原文。
func TestCache_ClearMeta(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/sub"
	if err := c.Save(url, []byte("BODY"), cacheMeta{ETag: `"x"`, Modified: time.Now()}); err != nil {
		t.Fatal(err)
	}
	c.ClearMeta(url)
	if etag, _ := c.Meta(url); etag != "" {
		t.Fatalf("清元数据后 ETag 应为空，实际 %q", etag)
	}
	if got, err := c.Content(url); err != nil || string(got) != "BODY" {
		t.Fatalf("原文应保留: %q err=%v", got, err)
	}
	// 重复清除不应报错
	c.ClearMeta(url)
}

func TestCache_CorruptedMeta(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/sub"
	if err := os.WriteFile(c.metaPath(url), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if etag, mod := c.Meta(url); etag != "" || !mod.IsZero() {
		t.Fatalf("损坏的元数据应返回空值: %q %v", etag, mod)
	}
}

func TestCache_PathsAreURLDerived(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/sub"
	if err := c.Save(url, []byte("BODY"), cacheMeta{ETag: `"x"`}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("应只有原文与元数据两个文件，实际 %d", len(entries))
	}
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(c.Dir, e.Name())); err != nil {
			t.Fatalf("缓存文件不可读: %s: %v", e.Name(), err)
		}
	}
}
