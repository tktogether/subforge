package subforge

import (
	"context"
	"strings"
	"testing"
)

func TestPipelineCRUD(t *testing.T) {
	m := newTestManager(t)
	p, err := m.CreatePipeline(context.Background(), "clean", "desc", `[{"op":"filter.regex"}]`, true)
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if p.ID == 0 || p.Name != "clean" || p.StepsJSON != `[{"op":"filter.regex"}]` || !p.Enabled || p.Source != "ui" {
		t.Fatalf("create result = %+v", *p)
	}
	got, err := m.GetPipeline(context.Background(), p.ID)
	if err != nil || got.Name != "clean" {
		t.Fatalf("GetPipeline: %v %+v", err, got)
	}
	byName, err := m.GetPipelineByName(context.Background(), "clean")
	if err != nil || byName.ID != p.ID {
		t.Fatalf("GetPipelineByName: %v %+v", err, byName)
	}
	rows, err := m.ListPipelines(context.Background())
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListPipelines: %v len=%d", err, len(rows))
	}
	upd := got
	upd.Description = "new desc"
	upd.StepsJSON = `[]`
	if err := m.UpdatePipeline(context.Background(), upd); err != nil {
		t.Fatalf("UpdatePipeline: %v", err)
	}
	got2, _ := m.GetPipeline(context.Background(), p.ID)
	if got2.Description != "new desc" || got2.StepsJSON != `[]` {
		t.Fatalf("update did not persist: %+v", got2)
	}
	if err := m.DeletePipeline(context.Background(), p.ID); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}
	if _, err := m.GetPipeline(context.Background(), p.ID); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("after delete GetPipeline err = %v", err)
	}
	if rows, _ := m.ListPipelines(context.Background()); len(rows) != 0 {
		t.Fatalf("after delete ListPipelines len = %d", len(rows))
	}
}

func TestPipelineEmptyName(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.CreatePipeline(context.Background(), "   ", "d", "[]", true); err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("CreatePipeline empty name err = %v", err)
	}
	if err := m.UpdatePipeline(context.Background(), Pipeline{ID: 1, Name: ""}); err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("UpdatePipeline empty name err = %v", err)
	}
}

func TestPipelineNotFound(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.GetPipeline(context.Background(), 9999); err == nil || !strings.Contains(err.Error(), "流水线 9999 不存在") {
		t.Fatalf("GetPipeline err = %v", err)
	}
	if _, err := m.GetPipelineByName(context.Background(), "nope"); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("GetPipelineByName err = %v", err)
	}
	if err := m.UpdatePipeline(context.Background(), Pipeline{ID: 9999, Name: "x"}); err == nil || !strings.Contains(err.Error(), "流水线 9999 不存在") {
		t.Fatalf("UpdatePipeline err = %v", err)
	}
	if err := m.DeletePipeline(context.Background(), 9999); err == nil || !strings.Contains(err.Error(), "流水线 9999 不存在") {
		t.Fatalf("DeletePipeline err = %v", err)
	}
}

func TestPipelineEmptyStepsDefault(t *testing.T) {
	m := newTestManager(t)
	p, err := m.CreatePipeline(context.Background(), "x", "", "", true)
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if p.StepsJSON != "[]" {
		t.Fatalf("empty steps not defaulted: %q", p.StepsJSON)
	}
}
