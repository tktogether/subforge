package mrs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateFile_Success(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "fixtures", "mrs", "cidr-sample.src.txt")
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Generate(srcBytes, "ipcidr", "text")
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "a", "b", "out.mrs")
	if err := GenerateFile(src, dst, "ipcidr", "text"); err != nil {
		t.Fatalf("GenerateFile: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("len %d vs %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
}

func TestGenerateFile_MissingSource(t *testing.T) {
	err := GenerateFile(filepath.Join(t.TempDir(), "nope.txt"), filepath.Join(t.TempDir(), "out.mrs"), "ipcidr", "text")
	if err == nil {
		t.Fatal("expected err")
	}
	if !strings.Contains(err.Error(), "读取源文件") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestGenerateFile_InvalidBehavior(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "fixtures", "mrs", "cidr-sample.src.txt")
	err := GenerateFile(src, filepath.Join(t.TempDir(), "out.mrs"), "bogus", "text")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestCacheValid_MRS(t *testing.T) {
	if CacheValid(filepath.Join(t.TempDir(), "nope"), time.Hour) {
		t.Error("expected false for missing")
	}
	p := filepath.Join(t.TempDir(), "c.mrs")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !CacheValid(p, time.Hour) {
		t.Error("expected true within ttl")
	}
	if !CacheValid(p, 0) {
		t.Error("expected true with ttl=0 (default)")
	}
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(p, past, past); err != nil {
		t.Fatal(err)
	}
	if CacheValid(p, time.Minute) {
		t.Error("expected false after 48h with minute ttl")
	}
}

func TestGenerateCached_MRS(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "fixtures", "mrs", "cidr-sample.src.txt")
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Generate(srcBytes, "ipcidr", "text")
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(t.TempDir(), "cache")
	got, err := GenerateCached(src, cacheDir, "ipcidr", "text", 0)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len %d vs %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
	cp := CachePath(cacheDir, src, "ipcidr", "text")
	cached, err := os.ReadFile(cp)
	if err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	if len(cached) != len(got) {
		t.Fatalf("cache len %d vs %d", len(cached), len(got))
	}
	for i := range cached {
		if cached[i] != got[i] {
			t.Fatalf("cache byte mismatch at %d", i)
		}
	}
	got2, err := GenerateCached(src, cacheDir, "ipcidr", "text", time.Hour)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(got2) != len(got) {
		t.Fatalf("second len %d vs %d", len(got2), len(got))
	}
	for i := range got2 {
		if got2[i] != got[i] {
			t.Fatalf("second byte mismatch at %d", i)
		}
	}
	_, err = GenerateCached(filepath.Join(t.TempDir(), "nope.txt"), cacheDir, "ipcidr", "text", time.Hour)
	if err == nil {
		t.Fatal("expected err for missing source")
	}
}
