package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"ohmymc/internal/core"
	"ohmymc/internal/jobs"
	"ohmymc/internal/plugins"
	"path/filepath"
	"runtime"
	"sync"
)

const Version = "0.1.0"

type Service struct {
	Store     *core.Store
	Processes *core.Processes
	Plugins   *plugins.Manager
	Jobs      *jobs.Manager
	ops       sync.Mutex
}

func New(root string) (*Service, error) {
	return NewWithMinecraftRoot(root, filepath.Join(root, "minecraft"))
}

func NewWithMinecraftRoot(root, minecraftRoot string) (*Service, error) {
	store, err := core.NewStoreWithMinecraftRoot(root, minecraftRoot)
	if err != nil {
		return nil, err
	}
	j := jobs.New()
	p, err := plugins.New(store, j)
	if err != nil {
		return nil, err
	}
	return &Service{Store: store, Processes: core.NewProcesses(store.Root()), Plugins: p, Jobs: j}, nil
}
func (s *Service) Close() { s.Jobs.Close() }

type State struct {
	Versions      []core.Version `json:"versions"`
	Plugins       []plugins.Info `json:"plugins"`
	Jobs          []jobs.Job     `json:"jobs"`
	Processes     []core.Process `json:"processes"`
	DataRoot      string         `json:"dataRoot"`
	MinecraftRoot string         `json:"minecraftRoot"`
	Platform      string         `json:"platform"`
	AppVersion    string         `json:"appVersion"`
}

func (s *Service) State() State {
	return State{
		Versions: s.Store.List(), Plugins: s.Plugins.List(), Jobs: s.Jobs.List(),
		Processes: s.Processes.List(), DataRoot: s.Store.Root(),
		MinecraftRoot: s.Store.MinecraftRoot(), Platform: runtime.GOOS, AppVersion: Version,
	}
}

type request struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Notes     string         `json:"notes"`
	Path      string         `json:"path"`
	Enabled   bool           `json:"enabled"`
	Action    string         `json:"action"`
	VersionID string         `json:"versionId"`
	Data      map[string]any `json:"data"`
}

func (s *Service) Invoke(ctx context.Context, method string, payload map[string]any) (any, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, errors.New("请求过大")
	}
	var p request
	if err = json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	switch method {
	case "state.get":
		return s.State(), nil
	case "versions.import":
		if p.Path == "" {
			return nil, errors.New("请选择要导入的目录")
		}
		s.ops.Lock()
		defer s.ops.Unlock()
		return s.Store.Import(p.Path)
	case "versions.update":
		s.ops.Lock()
		defer s.ops.Unlock()
		return s.Store.Update(p.ID, p.Name, p.Notes)
	case "versions.remove":
		s.ops.Lock()
		defer s.ops.Unlock()
		if s.Processes.Running(p.ID) {
			return nil, errors.New("请先停止此版本的游戏进程")
		}
		return nil, s.Store.Remove(p.ID)
	case "versions.launch", "versions.preview":
		s.ops.Lock()
		defer s.ops.Unlock()
		v, err := s.Store.Get(p.ID)
		if err != nil {
			return nil, err
		}
		if method == "versions.launch" && s.Processes.Running(v.ID) {
			return nil, errors.New("此版本已经运行")
		}
		options, err := s.Plugins.Configure(ctx, v)
		if err != nil {
			return nil, err
		}
		plan, err := core.BuildLaunch(ctx, v, options, s.Store.Root(), method == "versions.launch")
		if err != nil {
			return nil, err
		}
		if method == "versions.preview" {
			return plan, nil
		}
		proc, err := s.Processes.Start(v.ID, plan)
		if err != nil {
			return nil, err
		}
		if err = s.Store.Played(v.ID); err != nil {
			_ = s.Processes.Stop(v.ID)
			return nil, fmt.Errorf("保存启动记录失败，已停止进程: %w", err)
		}
		return proc, nil
	case "versions.stop":
		return nil, s.Processes.Stop(p.ID)
	case "logs.get":
		if _, err := s.Store.Get(p.ID); err != nil {
			return nil, err
		}
		return s.Processes.Logs(p.ID), nil
	case "java.detect":
		return core.DetectJava(ctx), nil
	case "plugins.toggle":
		s.ops.Lock()
		defer s.ops.Unlock()
		return nil, s.Plugins.Toggle(p.ID, p.Enabled)
	case "plugins.reload":
		s.ops.Lock()
		defer s.ops.Unlock()
		return nil, s.Plugins.Reload()
	case "plugins.invoke":
		return s.Plugins.Invoke(ctx, p.ID, p.Action, p.VersionID, p.Data)
	case "jobs.cancel":
		return nil, s.Jobs.Cancel(p.ID)
	case "desktop.pickFolder", "desktop.openFolder":
		return nil, errors.New("此操作需要桌面窗口；浏览器模式请手动输入或打开路径")
	default:
		return nil, fmt.Errorf("未知操作: %s", method)
	}
}
