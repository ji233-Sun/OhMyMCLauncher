package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"ohmymc/internal/core"
	"ohmymc/internal/jobs"
	"ohmymc/internal/safefile"
	"ohmymc/internal/transfer"
	bundled "ohmymc/plugins"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type Field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Required    bool     `json:"required,omitempty"`
}
type Panel struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Scope        string  `json:"scope"`
	Kind         string  `json:"kind"`
	LoadAction   string  `json:"loadAction"`
	SubmitAction string  `json:"submitAction"`
	SubmitLabel  string  `json:"submitLabel"`
	SelectAction string  `json:"selectAction,omitempty"`
	RemoveAction string  `json:"removeAction,omitempty"`
	Fields       []Field `json:"fields"`
}
type Action struct {
	Background bool `json:"background"`
}
type Manifest struct {
	APIVersion  int               `json:"apiVersion"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Description string            `json:"description"`
	Entry       string            `json:"entry"`
	Permissions []string          `json:"permissions"`
	Actions     map[string]Action `json:"actions"`
	Panels      []Panel           `json:"panels"`
	Hooks       []string          `json:"hooks"`
}
type Info struct {
	Manifest
	Enabled bool   `json:"enabled"`
	Builtin bool   `json:"builtin"`
	Error   string `json:"error,omitempty"`
}
type loaded struct {
	manifest Manifest
	source   string
	builtin  bool
	trustKey string
}
type Manager struct {
	mu      sync.RWMutex
	ops     sync.Mutex
	items   map[string]*loaded
	broken  []Info
	Store   *core.Store
	Jobs    *jobs.Manager
	Network *transfer.Client
}

func New(store *core.Store, j *jobs.Manager) (*Manager, error) {
	m := &Manager{Store: store, Jobs: j, Network: transfer.New(transfer.MinecraftHosts)}
	return m, m.Reload()
}

var pluginID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)
var capabilities = map[string]bool{"network.minecraft": true, "files.minecraft": true, "versions.add": true, "storage.version": true, "storage.global": true, "system.memory": true, "launch.configure": true, "identity.configure": true}

func validate(m Manifest) error {
	if m.APIVersion != 1 || !pluginID.MatchString(m.ID) || strings.TrimSpace(m.Name) == "" || m.Version == "" {
		return errors.New("插件 apiVersion、id、name 或 version 无效")
	}
	if err := safefile.Component(m.Entry); err != nil || !strings.HasSuffix(m.Entry, ".js") {
		return errors.New("入口必须是插件目录中的 .js 文件")
	}
	for _, p := range m.Permissions {
		if !capabilities[p] {
			return fmt.Errorf("未知权限 %s", p)
		}
	}
	has := func(p string) bool {
		for _, v := range m.Permissions {
			if v == p {
				return true
			}
		}
		return false
	}
	for _, hook := range m.Hooks {
		if (hook != "launch.configure" && hook != "identity.configure") || !has(hook) {
			return errors.New("不支持或未授权的启动钩子")
		}
	}
	if len(m.Panels) > 32 || len(m.Actions) > 64 {
		return errors.New("插件贡献项过多")
	}
	seen := map[string]bool{}
	for _, p := range m.Panels {
		if p.ID == "" || seen[p.ID] || p.Title == "" {
			return errors.New("面板 ID 重复或标题缺失")
		}
		seen[p.ID] = true
		if p.Scope != "version" && p.Scope != "global" {
			return errors.New("未知面板作用域")
		}
		if p.Kind != "catalog" && p.Kind != "form" && p.Kind != "launch" && p.Kind != "profiles" {
			return errors.New("未知面板类型")
		}
		for _, action := range []string{p.LoadAction, p.SubmitAction} {
			if _, ok := m.Actions[action]; !ok {
				return errors.New("面板引用了未声明动作")
			}
		}
		if p.Kind == "profiles" {
			if p.Scope != "global" {
				return errors.New("玩家档案面板必须是全局面板")
			}
			for _, action := range []string{p.SelectAction, p.RemoveAction} {
				if _, ok := m.Actions[action]; !ok {
					return errors.New("玩家档案面板引用了未声明动作")
				}
			}
		}
		if len(p.Fields) > 100 {
			return errors.New("表单字段过多")
		}
		keys := map[string]bool{}
		for _, f := range p.Fields {
			if f.Key == "" || keys[f.Key] || f.Key == "__proto__" || f.Key == "constructor" || f.Key == "prototype" {
				return errors.New("非法或重复表单字段")
			}
			keys[f.Key] = true
			switch f.Type {
			case "text", "textarea", "number", "checkbox":
			default:
				return errors.New("未知字段类型")
			}
		}
	}
	return nil
}
func bounded(fsys fs.FS, path string, limit int64) ([]byte, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("插件文件超过大小限制")
	}
	return b, nil
}
func readPlugin(fsys fs.FS, dir string, builtin bool) (*loaded, error) {
	b, err := bounded(fsys, dir+"/manifest.json", 128<<10)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err = json.Unmarshal(b, &manifest); err != nil {
		return nil, err
	}
	if err = validate(manifest); err != nil {
		return nil, err
	}
	code, err := bounded(fsys, dir+"/"+manifest.Entry, 1<<20)
	if err != nil {
		return nil, err
	}
	if err = validateScript(manifest.ID, string(code)); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append(append([]byte{}, b...), code...))
	key := manifest.ID
	if !builtin {
		key += "@" + hex.EncodeToString(digest[:])
	}
	return &loaded{manifest: manifest, source: string(code), builtin: builtin, trustKey: key}, nil
}
func (m *Manager) Reload() error {
	m.ops.Lock()
	defer m.ops.Unlock()
	for _, j := range m.Jobs.List() {
		if j.Status == "running" || j.Status == "queued" {
			return errors.New("请等待插件任务结束再重新扫描")
		}
	}
	items := map[string]*loaded{}
	broken := []Info{}
	load := func(fsys fs.FS, builtin bool) error {
		dirs, err := fs.ReadDir(fsys, ".")
		if err != nil {
			return err
		}
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			p, err := readPlugin(fsys, d.Name(), builtin)
			if err == nil {
				if _, exists := items[p.manifest.ID]; exists {
					err = errors.New("插件 ID 冲突，不允许覆盖已加载的插件")
				}
			}
			if err != nil {
				broken = append(broken, Info{Manifest: Manifest{ID: "invalid-" + d.Name(), Name: d.Name(), Version: "?", Permissions: []string{}, Panels: []Panel{}}, Builtin: builtin, Error: err.Error()})
				continue
			}
			items[p.manifest.ID] = p
		}
		return nil
	}
	if err := load(bundled.FS, true); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Join(m.Store.Root(), "plugins"))
	if err != nil {
		return err
	}
	defer root.Close()
	if err = load(root.FS(), false); err != nil {
		return err
	}
	m.mu.Lock()
	m.items = items
	m.broken = broken
	m.mu.Unlock()
	return nil
}
func (m *Manager) enabled(p *loaded) bool { return m.Store.Enabled(p.trustKey, p.builtin) }
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := append([]Info{}, m.broken...)
	for _, p := range m.items {
		out = append(out, Info{Manifest: p.manifest, Enabled: m.enabled(p), Builtin: p.builtin})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (m *Manager) get(id string) (*loaded, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p := m.items[id]
	if p == nil {
		return nil, errors.New("插件未加载或清单无效")
	}
	return p, nil
}
func (m *Manager) Toggle(id string, on bool) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	p, err := m.get(id)
	if err != nil {
		return err
	}
	if m.Jobs.Busy(id) {
		return errors.New("请先结束或取消该插件的任务")
	}
	return m.Store.SetEnabled(p.trustKey, on)
}
func (m *Manager) Invoke(ctx context.Context, id, action, versionID string, data map[string]any) (any, error) {
	m.ops.Lock()
	p, err := m.get(id)
	if err != nil {
		m.ops.Unlock()
		return nil, err
	}
	if !m.enabled(p) {
		m.ops.Unlock()
		return nil, errors.New("插件已禁用")
	}
	a, ok := p.manifest.Actions[action]
	if !ok {
		m.ops.Unlock()
		return nil, errors.New("插件未声明此动作")
	}
	input := map[string]any{"versionId": versionID, "data": data}
	if versionID != "" {
		v, e := m.Store.Get(versionID)
		if e != nil {
			m.ops.Unlock()
			return nil, e
		}
		input["version"] = v
	}
	if a.Background {
		jobID, e := m.Jobs.Start(id, p.manifest.Name, func(ctx context.Context, progress jobs.Progress) error {
			_, e := m.execute(ctx, p, action, versionID, input, progress, true)
			return e
		})
		m.ops.Unlock()
		if e != nil {
			return nil, e
		}
		return map[string]string{"jobId": jobID}, nil
	}
	m.ops.Unlock()
	return m.execute(ctx, p, action, versionID, input, nil, false)
}
func (m *Manager) Configure(ctx context.Context, v core.Version) (core.LaunchOptions, error) {
	options := core.DefaultOptions()
	for _, info := range m.List() {
		if !info.Enabled {
			continue
		}
		for _, hook := range info.Hooks {
			if hook != "launch.configure" {
				continue
			}
			p, err := m.get(info.ID)
			if err != nil {
				return options, err
			}
			out, err := m.execute(ctx, p, hook, v.ID, map[string]any{"versionId": v.ID, "version": v, "options": options}, nil, false)
			if err != nil {
				return options, fmt.Errorf("插件 %s 阻止启动: %w", info.Name, err)
			}
			b, err := json.Marshal(out)
			if err != nil {
				return options, err
			}
			if err = json.Unmarshal(b, &options); err != nil {
				return options, err
			}
		}
	}
	name, err := m.PlayerName(ctx)
	if err != nil {
		return options, err
	}
	options.PlayerName = name
	return options, options.Validate()
}

// Identity providers run after launch configuration so all versions use the
// selected global profile, regardless of old per-version playerName settings.
func (m *Manager) PlayerName(ctx context.Context) (string, error) {
	name := core.DefaultOptions().PlayerName
	for _, info := range m.List() {
		if !info.Enabled {
			continue
		}
		for _, hook := range info.Hooks {
			if hook != "identity.configure" {
				continue
			}
			p, err := m.get(info.ID)
			if err != nil {
				return "", err
			}
			out, err := m.execute(ctx, p, hook, "", map[string]any{}, nil, false)
			if err != nil {
				return "", fmt.Errorf("插件 %s 无法提供玩家身份: %w", info.Name, err)
			}
			b, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			var identity struct {
				PlayerName string `json:"playerName"`
			}
			if err = json.Unmarshal(b, &identity); err != nil {
				return "", err
			}
			if err = core.ValidatePlayerName(identity.PlayerName); err != nil {
				return "", err
			}
			name = identity.PlayerName
		}
	}
	return name, nil
}
