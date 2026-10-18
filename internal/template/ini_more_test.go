package template

import (
	"testing"
)

func TestSectionAddGroupSeparator_More(t *testing.T) {
	s := &Section{Name: "g"}
	if len(s.Entries()) != 0 {
		t.Fatalf("entries = %d", len(s.Entries()))
	}
	s.AddGroupSeparator()
	if len(s.Entries()) != 1 {
		t.Fatalf("entries = %d, want 1", len(s.Entries()))
	}
	if s.Entries()[0].Key != groupSeparator {
		t.Errorf("key = %q", s.Entries()[0].Key)
	}
	s.Add("a", Value{Raw: "1"})
	blocks := s.GroupBlocks()
	if len(blocks) < 1 {
		t.Errorf("blocks = %d", len(blocks))
	}
}

func TestConfigClone_More(t *testing.T) {
	cfg, err := ParseString("[s]\na=1\nb=2\n", "t")
	if err != nil {
		t.Fatal(err)
	}
	clone := cfg.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	if len(clone.SectionNames()) != len(cfg.SectionNames()) {
		t.Errorf("names len mismatch")
	}
	for i, n := range cfg.SectionNames() {
		if clone.SectionNames()[i] != n {
			t.Errorf("name %d = %q vs %q", i, clone.SectionNames()[i], n)
		}
	}
	v, ok := clone.Get("s", "a")
	if !ok {
		t.Fatal("missing a")
	}
	if v.Raw != "1" {
		t.Errorf("value = %q", v.Raw)
	}
	clone.Section("s").Set("a", Value{Raw: "9"})
	origV, _ := cfg.Get("s", "a")
	if origV.Raw != "1" {
		t.Errorf("original changed: %q", origV.Raw)
	}
}

func TestConfigCloneNil_More(t *testing.T) {
	var c *Config
	got := c.Clone()
	if got == nil {
		t.Fatal("nil result")
	}
	if len(got.SectionNames()) != 0 {
		t.Errorf("names = %d", len(got.SectionNames()))
	}
}
