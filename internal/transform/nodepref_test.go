package transform

import "testing"

func TestNodePrefSpec_Empty(t *testing.T) {
	if !(NodePrefSpec{}.Empty()) {
		t.Fatal("零值应 Empty")
	}
	if !(NodePrefSpec{Prefixes: map[string]string{"vmess": "x"}}.Empty() == false) {
		t.Fatal("Prefixes 非空应非 Empty")
	}
	if !(NodePrefSpec{Include: []string{"a"}}.Empty() == false) {
		t.Fatal("Include 非空应非 Empty")
	}
	if !(NodePrefSpec{Exclude: []string{"b"}}.Empty() == false) {
		t.Fatal("Exclude 非空应非 Empty")
	}
	if !(NodePrefSpec{Prefixes: map[string]string{"vmess": "x"}, Include: []string{"a"}, Exclude: []string{"b"}}.Empty() == false) {
		t.Fatal("全非空应非 Empty")
	}
}
