package lineform

import "testing"

func TestNetUpgrade(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http", "h2"},
		{"ws", "ws"},
		{"", ""},
		{"grpc", "grpc"},
	}
	for _, c := range cases {
		if got := netUpgrade(c.in); got != c.want {
			t.Errorf("netUpgrade(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoinKV(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{nil, ""},
		{[]string{""}, ""},
		{[]string{"a=1"}, "a=1"},
		{[]string{"", "a=1", "", "b=2"}, "a=1,b=2"},
		{[]string{"", ""}, ""},
	}
	for _, c := range cases {
		got := joinKV(c.parts...)
		if got != c.want {
			t.Errorf("joinKV(%v) = %q, want %q", c.parts, got, c.want)
		}
	}
}

func TestKV(t *testing.T) {
	cases := []struct {
		k, v, want string
	}{
		{"k", "", ""},
		{"k", "v", "k=v"},
		{"", "", ""},
		{"", "v", "=v"},
	}
	for _, c := range cases {
		if got := kv(c.k, c.v); got != c.want {
			t.Errorf("kv(%q, %q) = %q, want %q", c.k, c.v, got, c.want)
		}
	}
}

func TestRendererName_Lineform(t *testing.T) {
	f1 := Format{Name: "surge"}
	f2 := Format{Name: "loon"}
	r1 := New(f1, Options{})
	r2 := New(f2, Options{})
	if got := r1.Name(); got != "surge" {
		t.Errorf("Name() = %q, want %q", got, "surge")
	}
	if got := r2.Name(); got != "loon" {
		t.Errorf("Name() = %q, want %q", got, "loon")
	}
	if got := (&Renderer{}).Name(); got != "" {
		t.Errorf("zero Renderer Name() = %q, want %q", got, "")
	}
}
