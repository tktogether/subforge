package manifest

import "testing"

func TestWithRetention_Positive(t *testing.T) {
	o := updateOptions{retention: 5}
	WithRetention(3)(&o)
	if o.retention != 3 {
		t.Fatalf("retention = %d, want 3", o.retention)
	}
}

func TestWithRetention_ZeroAndNegative(t *testing.T) {
	o := updateOptions{retention: 9}
	WithRetention(0)(&o)
	if o.retention != 9 {
		t.Fatalf("retention(0) 改变了值: %d", o.retention)
	}
	WithRetention(-1)(&o)
	if o.retention != 9 {
		t.Fatalf("retention(-1) 改变了值: %d", o.retention)
	}
}
