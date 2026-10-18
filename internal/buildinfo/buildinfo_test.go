package buildinfo

import (
	"strings"
	"testing"
)

func TestFull_Default(t *testing.T) {
	got := Full()
	if got == "" {
		t.Fatal("Full() 不应为空")
	}
	if !strings.HasPrefix(got, "subforge ") {
		t.Fatalf("Full() 应以 'subforge ' 开头，实际 %q", got)
	}
}

func TestFull_Format(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	defer func() { Version, Commit, Date = oldV, oldC, oldD }()
	Version = "v9"
	Commit = "abc"
	Date = "2020-01-02"
	got := Full()
	want := "subforge v9 (commit abc, built 2020-01-02)"
	if got != want {
		t.Fatalf("Full() = %q, want %q", got, want)
	}
}
