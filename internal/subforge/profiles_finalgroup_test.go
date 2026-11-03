package subforge

import (
	"path/filepath"
	"testing"

	"github.com/tktogether/subforge/internal/subcfg"
)

func TestManagerSetProfileFinalGroup(t *testing.T) {
	m, err := NewManager(filepath.Join(t.TempDir(), "subforge.db"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	ctx := t.Context()
	p, err := m.AddProfile(ctx, "统一组", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	cfg := subcfg.DefaultFinalGroupConfig()
	cfg.Enabled = true
	cfg.Mode = subcfg.FinalGroupModeCustom
	cfg.Name = "🌐 统一出口"
	if err := m.SetProfileFinalGroup(ctx, p.ID, cfg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if !got.FinalGroup.Enabled || got.FinalGroup.Name != "🌐 统一出口" {
		t.Fatalf("FinalGroup = %+v", got.FinalGroup)
	}
	if err := m.SetProfileFinalGroup(ctx, 999999, cfg); err == nil {
		t.Fatal("不存在的方案应返回错误")
	}
}
