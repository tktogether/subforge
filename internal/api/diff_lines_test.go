package api

import (
	"reflect"
	"testing"
)

// TestDiffLines_EmptyBaselineIsClean 空串表示"没有任何行"：
// 空基线对非空文本应当只有新增行，不该多出一行内容为空的删除行（幽灵行）。
func TestDiffLines_EmptyBaselineIsClean(t *testing.T) {
	got := DiffLines("", "only-right")
	want := []DiffLine{{Type: "+", Text: "only-right"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiffLines(\"\", \"only-right\") = %+v，want %+v", got, want)
	}
}

// TestDiffLines_BothEmpty 两侧都空 = 无差异，结果应为空列表而不是一行空"相等"行。
func TestDiffLines_BothEmpty(t *testing.T) {
	if got := DiffLines("", ""); len(got) != 0 {
		t.Errorf("DiffLines(\"\", \"\") = %+v，want 空", got)
	}
}

// TestDiffLines_TrailingNewlineIsNotALine 末尾换行只是行结束符，不产生额外空行。
func TestDiffLines_TrailingNewlineIsNotALine(t *testing.T) {
	got := DiffLines("a\nb\n", "a\nb")
	for _, l := range got {
		if l.Type != " " {
			t.Errorf("仅末尾换行差异不应产生增删行: %+v", got)
			break
		}
	}
	if len(got) != 2 {
		t.Errorf("行数 = %d，want 2: %+v", len(got), got)
	}
}

// TestDiffLines_InsertAndDelete 增删行语义：左独有为 "-"，右独有为 "+"，相等为 " "。
func TestDiffLines_InsertAndDelete(t *testing.T) {
	got := DiffLines("keep\ndrop", "keep\nadd")
	var plus, minus int
	for _, l := range got {
		switch l.Type {
		case "+":
			plus++
			if l.Text != "add" {
				t.Errorf("新增行文本 = %q，want add", l.Text)
			}
		case "-":
			minus++
			if l.Text != "drop" {
				t.Errorf("删除行文本 = %q，want drop", l.Text)
			}
		}
	}
	if plus != 1 || minus != 1 {
		t.Errorf("增删计数 = +%d/-%d，want +1/-1: %+v", plus, minus, got)
	}
}
