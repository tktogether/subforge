package transform

import (
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/tktogether/subforge/internal/model"
)

// defaultScriptTimeout 是脚本执行的默认上限，防止死循环拖垮请求。
// 对齐设计书 M2 §1.1 的"5s 默认，可配置"。
const defaultScriptTimeout = 5 * time.Second

// 脚本沙箱约定（对齐设计书 §1.1 "脚本隔离"）：
//   - 只注入一个纯数据对象 node，不提供 require / fetch / 文件系统 / 进程能力；
//   - 引擎为 goja（ES5.1+，无 cgo），语法错误在构造期暴露；
//   - 单次执行超时后由 vm.Interrupt 打断，脚本 panic 被 recover 成错误，
//     两者都不会 crash 主进程。
//
// node 暴露的字段：name / type / server / port / source，以及完整原始配置 raw。
// 脚本通过 node.raw 能读到内核写入的任意字段（uuid / sni / ws-opts …）。

// scriptRunner 承载一段已编译脚本，可对多个节点重复求值。
// 预编译一次而不是每个节点都 Compile：1000 节点规模下这是数量级的差别。
type scriptRunner struct {
	name    string
	prog    *goja.Program
	timeout time.Duration
}

func newScriptRunner(name, src string, timeout time.Duration) (*scriptRunner, error) {
	if strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("脚本 %q 内容为空", name)
	}
	if timeout <= 0 {
		timeout = defaultScriptTimeout
	}
	prog, err := goja.Compile(name, src, false)
	if err != nil {
		return nil, fmt.Errorf("脚本 %q 编译失败: %w", name, err)
	}
	return &scriptRunner{name: name, prog: prog, timeout: timeout}, nil
}

// run 在带超时与 panic 兜底的前提下执行脚本，返回完成值。
func (r *scriptRunner) run(vm *goja.Runtime) (goja.Value, error) {
	start := time.Now()
	timer := time.AfterFunc(r.timeout, func() {
		vm.Interrupt(fmt.Sprintf("执行超过 %s 上限", r.timeout))
	})
	defer timer.Stop()

	var (
		v   goja.Value
		err error
	)
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("脚本 %q panic: %v", r.name, rec)
			}
		}()
		v, err = vm.RunProgram(r.prog)
	}()
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		var ie *goja.InterruptedError
		if ok := asInterrupted(err, &ie); ok {
			// 保留「脚本 %q 被中断（超时或死循环」前缀（internal/web/src/content/help.ts 索引该短语），
			// 额外带上中断原因与已运行耗时，便于排查。显示名来自调用方（试运行脚本），不暴露内部标识。
			return nil, fmt.Errorf("脚本 %q 被中断（超时或死循环）：%v（已运行 %s）", r.name, ie, elapsed)
		}
		return nil, fmt.Errorf("脚本 %q 执行失败: %w", r.name, err)
	}
	return v, nil
}

// asInterrupted 判定错误是否为中断错误。
func asInterrupted(err error, target **goja.InterruptedError) bool {
	if ie, ok := err.(*goja.InterruptedError); ok {
		*target = ie
		return true
	}
	return false
}

// nodeObject 把节点投影成脚本可见的纯数据对象。
func nodeObject(vm *goja.Runtime, n model.Node) *goja.Object {
	obj := vm.NewObject()
	_ = obj.Set("name", n.Name)
	_ = obj.Set("type", n.Type)
	_ = obj.Set("server", n.Server)
	_ = obj.Set("port", n.Port)
	_ = obj.Set("source", n.Source)
	raw := vm.NewObject()
	for k, v := range n.Raw {
		_ = raw.Set(k, v)
	}
	_ = obj.Set("raw", raw)
	return obj
}

// nodeFromObject 把脚本可能改过的对象读回节点。
// raw 是事实源：脚本改 node.raw.* 与改 node.name 都写回 raw，再统一重新派生。
func nodeFromObject(obj *goja.Object, orig model.Node) model.Node {
	raw := map[string]any{}
	if m, ok := obj.Get("raw").Export().(map[string]any); ok {
		for k, v := range m {
			raw[k] = v
		}
	}
	if len(raw) == 0 {
		for k, v := range orig.Raw {
			raw[k] = v
		}
	}
	// 顶层字段覆盖 raw：脚本里 node.name = "x" 比 node.raw.name = "x" 更自然。
	for _, k := range []string{"name", "type", "server", "port"} {
		v := obj.Get(k)
		if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
			continue
		}
		raw[k] = v.String()
	}
	if _, ok := raw["name"]; !ok {
		raw["name"] = orig.Name
	}
	return model.NodeFromRaw(raw, orig.Source)
}

// ScriptFilter 用脚本对每个节点做布尔判定，返回真值即保留（对应 filter.type=script）。
type ScriptFilter struct {
	runner *scriptRunner
}

// NewScriptFilter 编译过滤脚本；语法错误在构造期暴露。
func NewScriptFilter(name, src string, timeout time.Duration) (ScriptFilter, error) {
	r, err := newScriptRunner(name, src, timeout)
	if err != nil {
		return ScriptFilter{}, err
	}
	return ScriptFilter{runner: r}, nil
}

func (f ScriptFilter) Apply(ns model.NodeList) (model.NodeList, error) {
	out := make(model.NodeList, 0, len(ns))
	for _, n := range ns {
		vm := goja.New()
		if err := vm.Set("node", nodeObject(vm, n)); err != nil {
			return nil, fmt.Errorf("脚本过滤 %q: %w", n.Name, err)
		}
		v, err := f.runner.run(vm)
		if err != nil {
			return nil, fmt.Errorf("节点 %q: %w", n.Name, err)
		}
		if v.ToBoolean() {
			out = append(out, n)
		}
	}
	return out, nil
}

// ScriptOperate 用脚本改写每个节点（对应 op.type=script）。
// 脚本通过修改 node 的字段（name/type/server/port/raw）来生效；返回值被忽略。
type ScriptOperate struct {
	runner *scriptRunner
}

// NewScriptOperate 编译操作脚本；语法错误在构造期暴露。
func NewScriptOperate(name, src string, timeout time.Duration) (ScriptOperate, error) {
	r, err := newScriptRunner(name, src, timeout)
	if err != nil {
		return ScriptOperate{}, err
	}
	return ScriptOperate{runner: r}, nil
}

func (o ScriptOperate) Apply(ns model.NodeList) (model.NodeList, error) {
	out := make(model.NodeList, len(ns))
	for i, n := range ns {
		vm := goja.New()
		obj := nodeObject(vm, n)
		if err := vm.Set("node", obj); err != nil {
			return nil, fmt.Errorf("脚本操作 %q: %w", n.Name, err)
		}
		if _, err := o.runner.run(vm); err != nil {
			return nil, fmt.Errorf("节点 %q: %w", n.Name, err)
		}
		out[i] = nodeFromObject(obj, n)
	}
	return out, nil
}
