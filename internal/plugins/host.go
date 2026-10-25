package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"ohmymc/internal/core"
	"ohmymc/internal/jobs"
	"ohmymc/internal/safefile"
	"ohmymc/internal/transfer"
	"os"
	"path/filepath"
	"strings"
)

type invocation struct {
	manager   *Manager
	plugin    *loaded
	ctx       context.Context
	versionID string
	progress  jobs.Progress
	allocated *core.Version
	gamePath  string
	committed bool
}

func (i *invocation) require(cap string) error {
	if !i.manager.enabled(i.plugin) {
		return errors.New("插件已禁用")
	}
	for _, p := range i.plugin.manifest.Permissions {
		if p == cap {
			return nil
		}
	}
	return fmt.Errorf("插件权限不足: %s", cap)
}
func decode(b json.RawMessage, v any) error {
	if len(b) == 0 {
		b = []byte(`{}`)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("无效的宿主参数: %w", err)
	}
	return nil
}
func (i *invocation) host(method string, b json.RawMessage) (any, error) {
	if err := i.ctx.Err(); err != nil {
		return nil, err
	}
	if len(b) > 16<<20 {
		return nil, errors.New("宿主调用参数过大")
	}
	rootPath := i.manager.Store.MinecraftRoot()
	switch method {
	case "system.platform":
		return core.CurrentPlatform(), nil
	case "system.memory":
		if err := i.require("system.memory"); err != nil {
			return nil, err
		}
		return core.SystemMemory()
	case "identity.offline":
		if err := i.require("identity.configure"); err != nil {
			return nil, err
		}
		var p struct {
			Name string `json:"name"`
		}
		if err := decode(b, &p); err != nil {
			return nil, err
		}
		if err := core.ValidatePlayerName(p.Name); err != nil {
			return nil, err
		}
		return map[string]string{"name": p.Name, "uuid": core.OfflineUUID(p.Name)}, nil
	case "progress":
		var p struct {
			Phase string `json:"phase"`
			Done  int    `json:"done"`
			Total int    `json:"total"`
		}
		if err := decode(b, &p); err != nil {
			return nil, err
		}
		if len(p.Phase) > 200 {
			return nil, errors.New("进度文本过长")
		}
		if i.progress != nil {
			i.progress(p.Phase, p.Done, p.Total)
		}
		return nil, nil
	case "net.json":
		if err := i.require("network.minecraft"); err != nil {
			return nil, err
		}
		var p struct {
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
		}
		if err := decode(b, &p); err != nil {
			return nil, err
		}
		return i.manager.Network.JSON(i.ctx, p.URL, p.SHA1)
	case "files.download":
		if err := i.require("network.minecraft"); err != nil {
			return nil, err
		}
		if err := i.require("files.minecraft"); err != nil {
			return nil, err
		}
		var p struct {
			Files []transfer.File `json:"files"`
			Phase string          `json:"phase"`
		}
		if err := decode(b, &p); err != nil {
			return nil, err
		}
		err := i.manager.Network.Many(i.ctx, rootPath, p.Files, func(done, total int) {
			if i.progress != nil {
				i.progress(p.Phase, done, total)
			}
		})
		return nil, err
	case "files.copy":
		if err := i.require("files.minecraft"); err != nil {
			return nil, err
		}
		var p struct {
			Items []transfer.Copy `json:"items"`
		}
		if err := decode(b, &p); err != nil {
			return nil, err
		}
		return nil, transfer.Copies(i.ctx, rootPath, p.Items)
	case "files.writeJSON":
		if err := i.require("files.minecraft"); err != nil {
			return nil, err
		}
		var p struct {
			Path  string          `json:"path"`
			Value json.RawMessage `json:"value"`
		}
		if err := decode(b, &p); err != nil {
			return nil, err
		}
		if !strings.HasSuffix(p.Path, ".json") || !json.Valid(p.Value) {
			return nil, errors.New("只允许写入 JSON 文件")
		}
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		return nil, safefile.AtomicRoot(root, p.Path, p.Value)
	case "storage.global.get", "storage.global.set":
		if err := i.require("storage.global"); err != nil {
			return nil, err
		}
		if method == "storage.global.get" {
			return i.manager.Store.GlobalSetting(i.plugin.manifest.ID), nil
		}
		return nil, i.manager.Store.SetGlobalSetting(i.plugin.manifest.ID, b)
	case "storage.get", "storage.set":
		if err := i.require("storage.version"); err != nil {
			return nil, err
		}
		if _, err := i.manager.Store.Get(i.versionID); err != nil {
			return nil, err
		}
		if method == "storage.get" {
			return i.manager.Store.Setting(i.versionID, i.plugin.manifest.ID), nil
		}
		return nil, i.manager.Store.SetSetting(i.versionID, i.plugin.manifest.ID, b)
	case "launch.defaults":
		if err := i.require("launch.configure"); err != nil {
			return nil, err
		}
		return core.DefaultOptions(), nil
	case "launch.validate":
		if err := i.require("launch.configure"); err != nil {
			return nil, err
		}
		var options core.LaunchOptions
		if err := decode(b, &options); err != nil {
			return nil, err
		}
		return nil, options.Validate()
	case "versions.allocate":
		if err := i.require("versions.add"); err != nil {
			return nil, err
		}
		if i.allocated != nil {
			return nil, errors.New("每个调用只能分配一个版本")
		}
		var p struct {
			Version string `json:"version"`
			Name    string `json:"name"`
			Isolate bool   `json:"isolate"`
		}
		if err := decode(b, &p); err != nil {
			return nil, err
		}
		if err := safefile.Component(p.Version); err != nil {
			return nil, err
		}
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" || len([]rune(p.Name)) > 80 {
			return nil, errors.New("版本名称须为 1–80 个字符")
		}
		id := safefile.ID()
		game := rootPath
		if p.Isolate {
			i.gamePath = "instances/" + id
			game = filepath.Join(rootPath, "instances", id)
		}
		i.allocated = &core.Version{ID: id, Name: p.Name, MinecraftVersion: p.Version, VersionID: p.Version, Directory: rootPath, GameDirectory: game}
		return map[string]string{"id": id, "gamePath": i.gamePath}, nil
	case "versions.commit":
		if err := i.require("versions.add"); err != nil {
			return nil, err
		}
		if i.allocated == nil || i.committed {
			return nil, errors.New("没有待登记的版本")
		}
		v := *i.allocated
		meta, err := core.Resolve(v.Directory, v.VersionID)
		if err != nil {
			return nil, err
		}
		if _, err = os.Stat(filepath.Join(rootPath, "versions", meta.Jar, meta.Jar+".jar")); err != nil {
			return nil, err
		}
		v.JavaMajor = meta.JavaVersion.MajorVersion
		v.Type = meta.Type
		if err = os.MkdirAll(v.GameDirectory, 0700); err != nil {
			return nil, err
		}
		if err = i.ctx.Err(); err != nil {
			return nil, err
		}
		v, err = i.manager.Store.Add(v)
		if err == nil {
			i.committed = true
		}
		return v, err
	default:
		return nil, fmt.Errorf("未知宿主能力 %s", method)
	}
}
