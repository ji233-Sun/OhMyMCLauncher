package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Calcium-Ion/moejs"
	"ohmymc/internal/jobs"
	"sync/atomic"
	"time"
)

func compile(ctx context.Context, id, source string) (*moejs.Module, error) {
	mod, err := moejs.CompileWith(id+"/main.js", source, moejs.CompileOptions{Context: ctx})
	if err != nil {
		return nil, err
	}
	if len(mod.Requests()) != 0 {
		return nil, errors.New("插件须打包为单文件，禁止外部模块导入")
	}
	if _, err = mod.Hook("invoke"); err != nil {
		return nil, errors.New("插件须 export function invoke(action, input)")
	}
	return mod, nil
}
func validateScript(id, source string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := compile(ctx, id, source)
	return err
}

// Each call gets a fresh realm, so failed calls and concurrent plugins never share
// JS state. The only concurrent Runtime method used is its supported Interrupt.
func (m *Manager) execute(parent context.Context, p *loaded, action, versionID string, input map[string]any, progress jobs.Progress, background bool) (result any, err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("插件运行异常: %v", v)
		}
	}()
	timeout := 45 * time.Second
	if background {
		timeout = 45 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	compileCtx, compileCancel := context.WithTimeout(ctx, 2*time.Second)
	mod, err := compile(compileCtx, p.manifest.ID, p.source)
	compileCancel()
	if err != nil {
		return nil, err
	}
	hook, err := mod.Hook("invoke")
	if err != nil {
		return nil, err
	}
	rt := moejs.NewRuntime(moejs.Options{DisableDynamicCode: true, MemoryLimit: 128 << 20})
	defer rt.ReleaseCallData()
	i := &invocation{manager: m, plugin: p, ctx: ctx, versionID: versionID, progress: progress}
	var inHost atomic.Bool
	done, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		var spent time.Duration
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				rt.Interrupt(ctx.Err())
				return
			case <-ticker.C:
				if !inHost.Load() {
					spent += 20 * time.Millisecond
				}
				if spent > 8*time.Second {
					rt.Interrupt(errors.New("插件 JavaScript 执行预算超限"))
					cancel()
					return
				}
			}
		}
	}()
	defer func() { close(done); <-joined }()
	err = rt.SetGlobal("host", map[string]any{"call": moejs.NativeFunc(func(realm *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if len(args) < 1 || len(args) > 2 || !args[0].IsString() {
			return moejs.Undefined(), realm.TypeError("host.call expects a method and optional payload")
		}
		payload := json.RawMessage(`{}`)
		if len(args) == 2 {
			var value any
			if err := rt.UnmarshalWith(args[1], &value, moejs.DecodeOptions{MaxBytes: 16 << 20, MaxNodes: 1000000}); err != nil {
				return moejs.Undefined(), err
			}
			b, err := json.Marshal(value)
			if err != nil {
				return moejs.Undefined(), err
			}
			payload = b
		}
		value, err := func() (any, error) {
			inHost.Store(true)
			defer inHost.Store(false)
			return i.host(args[0].String(), payload)
		}()
		if err != nil {
			return moejs.Undefined(), err
		}
		b, err := json.Marshal(value)
		if err != nil {
			return moejs.Undefined(), err
		}
		return rt.ParseJSONString(string(b))
	})})
	if err != nil {
		return nil, err
	}
	if err = rt.Load(mod); err != nil {
		return nil, err
	}
	has, err := rt.Has(hook)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, errors.New("invoke 导出不是函数")
	}
	b, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	arg, err := rt.ParseJSONString(string(b))
	if err != nil {
		return nil, err
	}
	actionValue, err := rt.FromGo(action)
	if err != nil {
		return nil, err
	}
	value, err := rt.Call(hook, actionValue, arg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.manifest.Name, err)
	}
	if _, _, promise := moejs.PromiseResult(value); promise {
		return nil, errors.New("v1 插件须返回同步 JSON；长任务请声明 background")
	}
	if err = rt.UnmarshalWith(value, &result, moejs.DecodeOptions{MaxBytes: 16 << 20, MaxNodes: 1000000}); err != nil {
		return nil, err
	}
	return result, ctx.Err()
}
