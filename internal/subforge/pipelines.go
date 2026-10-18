package subforge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/store"
)

// --- pipelines（有序步骤管线，plan §4.1）---
//
// store 层已有 pipelines 表的完整 CRUD，这里只做 Manager 包装：把 sql.ErrNoRows 翻成
// 既有口径的中文错误（与 subforge.go 的「订阅 %d 不存在」一致），并在 Manager 层挡一次空名。
// HTTP 层据此区分 404（不存在）与 500（真故障）。

// Pipeline 是 pipelines 表的一行；别名与 subforge.go 里的 Rule 同构。
type Pipeline = store.Pipeline

// ListPipelines 返回全部 DB 管线（不含模板来源，模板由 internal/api 合并）。
func (m *Manager) ListPipelines(ctx context.Context) ([]Pipeline, error) {
	return m.db.ListPipelines(ctx)
}

// GetPipeline 按 id 取管线；不存在返回「流水线 %d 不存在」。
func (m *Manager) GetPipeline(ctx context.Context, id int64) (Pipeline, error) {
	p, err := m.db.GetPipeline(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Pipeline{}, fmt.Errorf("流水线 %d 不存在", id)
		}
		return Pipeline{}, err
	}
	return p, nil
}

// GetPipelineByName 按名取管线；不存在返回「流水线 %s 不存在」。
func (m *Manager) GetPipelineByName(ctx context.Context, name string) (Pipeline, error) {
	p, err := m.db.GetPipelineByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Pipeline{}, fmt.Errorf("流水线 %s 不存在", name)
		}
		return Pipeline{}, err
	}
	return p, nil
}

// CreatePipeline 新建管线。空名在 Manager 层挡下（HTTP 层据此回 400），
// Source 固定为 ui —— template 来源只由模板解析产出，不落库。
func (m *Manager) CreatePipeline(ctx context.Context, name, description, stepsJSON string, enabled bool) (*Pipeline, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("流水线名称不能为空")
	}
	if strings.TrimSpace(stepsJSON) == "" {
		stepsJSON = "[]"
	}
	return m.db.CreatePipeline(ctx, Pipeline{
		Name:        name,
		Description: description,
		StepsJSON:   stepsJSON,
		Enabled:     enabled,
		Source:      "ui",
	})
}

// UpdatePipeline 整体更新一条管线；id 不存在返回「流水线 %d 不存在」。
func (m *Manager) UpdatePipeline(ctx context.Context, p Pipeline) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("流水线名称不能为空")
	}
	if err := m.db.UpdatePipeline(ctx, p); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("流水线 %d 不存在", p.ID)
		}
		return err
	}
	return nil
}

// DeletePipeline 按 id 删除；不存在返回「流水线 %d 不存在」。
func (m *Manager) DeletePipeline(ctx context.Context, id int64) error {
	if err := m.db.DeletePipeline(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("流水线 %d 不存在", id)
		}
		return err
	}
	return nil
}
