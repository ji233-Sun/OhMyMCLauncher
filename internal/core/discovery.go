package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ohmymc/internal/safefile"
)

type ScanIssue struct {
	VersionID string `json:"versionId"`
	Message   string `json:"message"`
}

type ScanResult struct {
	Added  []Version   `json:"added"`
	Issues []ScanIssue `json:"issues"`
}

func discoveryKey(root, id string) string {
	key := filepath.Join(filepath.Clean(root), "versions", id)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

// Discover reads the configured game tree only. A broken sibling never hides
// healthy versions, and removed registrations stay hidden across restarts.
func (s *Store) Discover(restoreRemoved bool) (ScanResult, error) {
	result := ScanResult{Added: []Version{}, Issues: []ScanIssue{}}
	entries, err := os.ReadDir(filepath.Join(s.minecraftRoot, "versions"))
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	candidates := []Version{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if _, err := os.Stat(filepath.Join(s.minecraftRoot, "versions", id, id+".json")); errors.Is(err, os.ErrNotExist) {
			continue
		}
		meta, err := Resolve(s.minecraftRoot, id)
		if err == nil {
			var info os.FileInfo
			info, err = os.Stat(filepath.Join(s.minecraftRoot, "versions", meta.Jar, meta.Jar+".jar"))
			if err == nil && !info.Mode().IsRegular() {
				err = errors.New("客户端 JAR 不是普通文件")
			}
		}
		if err != nil {
			result.Issues = append(result.Issues, ScanIssue{VersionID: id, Message: err.Error()})
			continue
		}
		candidates = append(candidates, Version{Name: id, VersionID: id, MinecraftVersion: meta.ID, Type: meta.Type, Directory: s.minecraftRoot, GameDirectory: importedGameDirectory(s.minecraftRoot, id), JavaMajor: meta.JavaVersion.MajorVersion})
	}
	// Avoid rewriting launcher state on every startup when nothing has changed.
	s.mu.RLock()
	needsSave := false
	for _, v := range candidates {
		key := discoveryKey(v.Directory, v.VersionID)
		registered := false
		for _, old := range s.db.Versions {
			if discoveryKey(old.Directory, old.VersionID) == key {
				registered = true
				break
			}
		}
		if !registered && (!s.db.Ignored[key] || restoreRemoved) {
			needsSave = true
			break
		}
	}
	s.mu.RUnlock()
	if !needsSave {
		return result, nil
	}
	err = s.mutate(func() error {
		for _, v := range candidates {
			key := discoveryKey(v.Directory, v.VersionID)
			if s.db.Ignored[key] && !restoreRemoved {
				continue
			}
			exists := false
			for _, old := range s.db.Versions {
				if discoveryKey(old.Directory, old.VersionID) == key {
					exists = true
					break
				}
			}
			if !exists {
				v.ID = safefile.ID()
				v.CreatedAt = time.Now().UTC().Format(time.RFC3339)
				s.db.Versions = append(s.db.Versions, v)
				result.Added = append(result.Added, v)
				delete(s.db.Ignored, key)
			}
		}
		return nil
	})
	if err != nil {
		result.Added = []Version{}
	}
	return result, err
}
