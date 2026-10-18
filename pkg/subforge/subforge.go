// Package subforge 提供 SubForge 的编程接口，允许其他 Go 程序直接使用
// SubForge 的转换引擎，无需启动 HTTP 服务。
//
// 用法：
//
//	engine, err := subforge.NewEngine(subforge.DefaultEngineConfig())
//	if err != nil {
//		log.Fatal(err)
//	}
//	result, err := engine.Convert(ctx, subforge.Params{
//		URL:    "https://example.com/sub",
//		Target: "clash.meta",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(string(result.Body))
package subforge

import (
	"context"
	"fmt"
	"time"

	"github.com/tktogether/subforge/internal/buildinfo"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
)

// Version 返回当前 SubForge 版本。
func Version() string {
	return buildinfo.Full()
}

// Params 是转换请求的参数。
type Params = convert.Params

// Result 是转换结果。
type Result = convert.Result

// Engine 是转换引擎。
type Engine struct {
	engine *convert.Engine
}

// DefaultEngineConfig 返回默认引擎配置。
type EngineConfig struct {
	// FetcherConfig 拉取器配置。
	FetcherConfig fetch.Config
	// Profile 产物注释头信息。
	Profile render.Profile
	// Now 生成时间戳来源。
	Now func() time.Time
}

// DefaultEngineConfig 返回默认引擎配置。
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		FetcherConfig: fetch.DefaultConfig(),
		Profile: render.Profile{
			Name:    "SubForge",
			Version: buildinfo.Version,
		},
		Now: time.Now,
	}
}

// NewEngine 创建一个转换引擎。
func NewEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.FetcherConfig.Parallel <= 0 {
		cfg.FetcherConfig = fetch.DefaultConfig()
	}
	if cfg.Profile.Name == "" {
		cfg.Profile.Name = "SubForge"
	}
	if cfg.Profile.Version == "" {
		cfg.Profile.Version = buildinfo.Version
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	fetcher := fetch.New(fetch.WithConfig(cfg.FetcherConfig))

	engine := &convert.Engine{
		Fetcher: fetcher,
		RendererFor: func(target string) (render.Renderer, error) {
			return registry.Lookup(target, clashmeta.Options{})
		},
		Profile: cfg.Profile,
		Now:     cfg.Now,
	}

	return &Engine{engine: engine}, nil
}

// Convert 执行一次转换。
func (e *Engine) Convert(ctx context.Context, p Params) (Result, error) {
	if e == nil || e.engine == nil {
		return Result{}, fmt.Errorf("Engine 未初始化")
	}
	return e.engine.Run(ctx, p)
}

// Fetcher 返回底层拉取器。
func (e *Engine) Fetcher() *fetch.Fetcher {
	if e == nil || e.engine == nil {
		return nil
	}
	return e.engine.Fetcher
}
