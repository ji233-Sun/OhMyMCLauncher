package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ohmymc/internal/safefile"
)

type Version struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	MinecraftVersion string `json:"minecraftVersion"`
	VersionID        string `json:"versionId"`
	Type             string `json:"type"`
	Notes            string `json:"notes"`
	Directory        string `json:"directory"`
	GameDirectory    string `json:"gameDirectory"`
	CreatedAt        string `json:"createdAt"`
	LastPlayed       string `json:"lastPlayed"`
	PlayCount        int    `json:"playCount"`
	JavaMajor        int    `json:"javaMajor"`
}
type database struct {
	Schema   int                                   `json:"schema"`
	Versions []Version                             `json:"versions"`
	Enabled  map[string]bool                       `json:"enabled"`
	Settings map[string]map[string]json.RawMessage `json:"settings"`
}
type Store struct {
	mu            sync.RWMutex
	root          string
	minecraftRoot string
	db            database
}

func NewStore(root string) (*Store, error) {
	return NewStoreWithMinecraftRoot(root, filepath.Join(root, "minecraft"))
}

// NewStoreWithMinecraftRoot keeps launcher state separate from managed game files.
// Existing version records retain their original paths; no files are migrated.
func NewStoreWithMinecraftRoot(root, minecraftRoot string) (*Store, error) {
	if strings.TrimSpace(minecraftRoot) == "" {
		return nil, errors.New("游戏安装目录不能为空")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	games, err := filepath.Abs(minecraftRoot)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	s := &Store{root: abs, minecraftRoot: games, db: database{Schema: 1, Versions: []Version{}, Enabled: map[string]bool{}, Settings: map[string]map[string]json.RawMessage{}}}
	b, err := os.ReadFile(filepath.Join(abs, "state.json"))
	if err == nil {
		if err = json.Unmarshal(b, &s.db); err != nil {
			return nil, fmt.Errorf("版本数据库损坏，已保留原文件: %w", err)
		}
		if s.db.Schema != 1 {
			return nil, errors.New("不支持的数据库版本")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.db.Enabled == nil {
		s.db.Enabled = map[string]bool{}
	}
	if s.db.Settings == nil {
		s.db.Settings = map[string]map[string]json.RawMessage{}
	}
	if s.db.Versions == nil {
		s.db.Versions = []Version{}
	}
	for _, d := range []string{"plugins", "logs", "instances"} {
		if err = os.MkdirAll(filepath.Join(abs, d), 0700); err != nil {
			return nil, err
		}
	}
	if err = os.MkdirAll(games, 0700); err != nil {
		return nil, fmt.Errorf("创建游戏安装目录失败: %w", err)
	}
	return s, nil
}
func (s *Store) Root() string          { return s.root }
func (s *Store) MinecraftRoot() string { return s.minecraftRoot }
func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}
	return safefile.Atomic(filepath.Join(s.root, "state.json"), b)
}

// mutate rolls back memory too if durable storage fails.
func (s *Store) mutate(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	backup, _ := json.Marshal(s.db)
	if err := fn(); err != nil {
		return err
	}
	if err := s.saveLocked(); err != nil {
		var restored database
		_ = json.Unmarshal(backup, &restored)
		s.db = restored
		return err
	}
	return nil
}
func (s *Store) List() []Version {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Version{}, s.db.Versions...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}
func (s *Store) Get(id string) (Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.db.Versions {
		if v.ID == id {
			return v, nil
		}
	}
	return Version{}, errors.New("版本不存在")
}
func (s *Store) Add(v Version) (Version, error) {
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 80 {
		return v, errors.New("版本名称须为 1–80 个字符")
	}
	err := s.mutate(func() error {
		for _, old := range s.db.Versions {
			if old.Directory == v.Directory && old.VersionID == v.VersionID && old.GameDirectory == v.GameDirectory {
				v = old
				return nil
			}
		}
		if v.ID == "" {
			v.ID = safefile.ID()
		}
		v.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		s.db.Versions = append(s.db.Versions, v)
		return nil
	})
	return v, err
}
func (s *Store) Update(id, name, notes string) (Version, error) {
	var result Version
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 || len(notes) > 4000 {
		return result, errors.New("名称须为 1–80 个字符，说明不能超过 4000 字节")
	}
	err := s.mutate(func() error {
		for i := range s.db.Versions {
			if s.db.Versions[i].ID == id {
				s.db.Versions[i].Name = name
				s.db.Versions[i].Notes = notes
				result = s.db.Versions[i]
				return nil
			}
		}
		return errors.New("版本不存在")
	})
	return result, err
}
func (s *Store) Remove(id string) error {
	return s.mutate(func() error {
		for i, v := range s.db.Versions {
			if v.ID == id {
				s.db.Versions = append(s.db.Versions[:i], s.db.Versions[i+1:]...)
				delete(s.db.Settings, id)
				return nil
			}
		}
		return errors.New("版本不存在")
	})
}
func (s *Store) Played(id string) error {
	return s.mutate(func() error {
		for i := range s.db.Versions {
			if s.db.Versions[i].ID == id {
				s.db.Versions[i].LastPlayed = time.Now().UTC().Format(time.RFC3339)
				s.db.Versions[i].PlayCount++
				return nil
			}
		}
		return errors.New("版本不存在")
	})
}
func (s *Store) Enabled(id string, def bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.db.Enabled[id]
	if !ok {
		return def
	}
	return b
}
func (s *Store) SetEnabled(id string, on bool) error {
	return s.mutate(func() error { s.db.Enabled[id] = on; return nil })
}
func (s *Store) Setting(version, plugin string) json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.db.Settings[version][plugin]
	if len(b) == 0 {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage{}, b...)
}
func (s *Store) SetSetting(version, plugin string, b json.RawMessage) error {
	if len(b) > 65536 || !json.Valid(b) {
		return errors.New("插件配置过大或不是有效 JSON")
	}
	if _, err := s.Get(version); err != nil {
		return err
	}
	return s.mutate(func() error {
		if s.db.Settings[version] == nil {
			s.db.Settings[version] = map[string]json.RawMessage{}
		}
		s.db.Settings[version][plugin] = append(json.RawMessage{}, b...)
		return nil
	})
}

// Import accepts a launcher folder containing .minecraft, a Minecraft root,
// its versions directory, or one version directory. Imported files stay in place.
func (s *Store) Import(path string) ([]Version, error) {
	path, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("请选择 Minecraft 目录或版本文件夹")
	}
	root := path
	ids := []string{}
	base := filepath.Base(path)
	if _, err := os.Stat(filepath.Join(path, base+".json")); err == nil && filepath.Base(filepath.Dir(path)) == "versions" {
		root = filepath.Dir(filepath.Dir(path))
		ids = append(ids, base)
	} else {
		if base == "versions" {
			root = filepath.Dir(path)
		} else if !directoryExists(filepath.Join(root, "versions")) && directoryExists(filepath.Join(root, ".minecraft", "versions")) {
			root = filepath.Join(root, ".minecraft")
		}
		entries, err := os.ReadDir(filepath.Join(root, "versions"))
		if err != nil {
			return nil, fmt.Errorf("找不到 versions 目录: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				if _, err := os.Stat(filepath.Join(root, "versions", e.Name(), e.Name()+".json")); err == nil {
					ids = append(ids, e.Name())
				}
			}
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("未找到包含版本 JSON 的有效版本")
	}
	// Validate the whole import before altering the registry.
	candidates := []Version{}
	for _, id := range ids {
		m, err := Resolve(root, id)
		if err != nil {
			return nil, fmt.Errorf("版本 %s: %w", id, err)
		}
		game := importedGameDirectory(root, id)
		candidates = append(candidates, Version{Name: id, VersionID: id, MinecraftVersion: m.ID, Type: m.Type, Directory: root, GameDirectory: game, JavaMajor: m.JavaVersion.MajorVersion})
	}
	added := []Version{}
	err = s.mutate(func() error {
		for _, v := range candidates {
			exists := false
			for _, old := range s.db.Versions {
				if old.Directory == v.Directory && old.VersionID == v.VersionID {
					added = append(added, old)
					exists = true
					break
				}
			}
			if !exists {
				v.ID = safefile.ID()
				v.CreatedAt = time.Now().UTC().Format(time.RFC3339)
				s.db.Versions = append(s.db.Versions, v)
				added = append(added, v)
			}
		}
		return nil
	})
	return added, err
}

func directoryExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// Recognize version-local game data even before the first world is created.
// Launcher metadata and native caches alone do not imply game isolation.
// No private launcher settings, accounts or game data are read or rewritten.
func importedGameDirectory(root, id string) string {
	dir := filepath.Join(root, "versions", id)
	for _, marker := range []string{"saves", "mods", "config", "defaultconfigs", "resourcepacks", "shaderpacks", "screenshots", "resources"} {
		if directoryExists(filepath.Join(dir, marker)) {
			return dir
		}
	}
	if fi, err := os.Stat(filepath.Join(dir, "options.txt")); err == nil && fi.Mode().IsRegular() {
		return dir
	}
	return root
}
