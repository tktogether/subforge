package subforge

import (
	"context"
	"testing"
)

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("Version() 返回空字符串")
	}
	t.Logf("Version: %s", v)
}

func TestNewEngine(t *testing.T) {
	engine, err := NewEngine(DefaultEngineConfig())
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}
	if engine == nil {
		t.Fatal("NewEngine 返回 nil")
	}
	if engine.Fetcher() == nil {
		t.Fatal("Fetcher 未初始化")
	}
}

func TestConvert_NoSource(t *testing.T) {
	engine, err := NewEngine(DefaultEngineConfig())
	if err != nil {
		t.Fatalf("NewEngine 失败: %v", err)
	}
	_, err = engine.Convert(context.Background(), Params{})
	if err == nil {
		t.Fatal("空来源应该报错")
	}
	t.Logf("预期错误: %v", err)
}
