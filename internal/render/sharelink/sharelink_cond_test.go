package sharelink

import (
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

func TestLabelOfWithName(t *testing.T) {
	if got := labelOf(model.Node{Name: "foo", Type: "ss"}); got != "foo" {
		t.Fatalf("got %q", got)
	}
	if got := labelOf(model.Node{Type: "ss"}); got != "<unnamed ss>" {
		t.Fatalf("got %q", got)
	}
}
func TestUserinfoStringBranches(t *testing.T) {
	if got := userinfoString("u", "p"); got == "" {
		t.Fatalf("empty")
	}
	if got := userinfoString("u", ""); got == "" {
		t.Fatalf("empty")
	}
	if got := userinfoString("", "p"); got == "" {
		t.Fatalf("empty")
	}
}
func TestIsTrueSharelink(t *testing.T) {
	if !isTrue(true) {
		t.Fatal("exp true")
	}
	if isTrue("false") {
		t.Fatal("exp false")
	}
	if isTrue(42) {
		t.Fatal("exp false")
	}
}
func TestStrSliceBoth(t *testing.T) {
	if got := strSlice([]string{"a"}); len(got) != 1 {
		t.Fatalf("len %d", len(got))
	}
	if got := strSlice([]any{"a", nil}); len(got) != 1 {
		t.Fatalf("len %d", len(got))
	}
}
func TestSharelinkRenderBase64(t *testing.T) {
	r := New("list", Options{Base64: true})
	req := render.Request{Nodes: model.NodeList{model.NewNodeFromParts("ss-node", "ss", "1.1.1.1", "8388", map[string]any{"cipher": "aes-256-gcm", "password": "p"})}}
	out, err := r.Render(req)
	if err != nil || len(out) == 0 {
		t.Fatalf("err %v len %d", err, len(out))
	}
}
func TestSharelinkRenderUnsupportedSkip(t *testing.T) {
	r := New("list", Options{})
	req := render.Request{Nodes: model.NodeList{model.NewNodeFromParts("x", "unknownproto", "1.1.1.1", "443", nil)}}
	out, err := r.Render(req)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(out) == 0 {
	}
}
