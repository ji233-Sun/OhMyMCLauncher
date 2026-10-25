package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoveryNonDestructiveAndRemovalPersists(t *testing.T) {
	data, games := t.TempDir(), filepath.Join(t.TempDir(), ".minecraft")
	metadataFile(t, games, "healthy", `{"id":"healthy","mainClass":"Main","javaVersion":{"majorVersion":21}}`)
	put(t, filepath.Join(games, "versions", "healthy", "healthy.jar"), []byte("test fixture"))
	put(t, filepath.Join(games, "versions", "healthy", "mods", "fixture.jar"), []byte("preserve"))
	metadataFile(t, games, "broken", `{`)
	metadataFile(t, games, "incomplete", `{"id":"incomplete","mainClass":"Main"}`)
	before := layoutSnapshot(t, games)
	s, err := NewStoreWithMinecraftRoot(data, games)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := s.Discover(false)
	if err != nil || len(scan.Added) != 1 || len(scan.Issues) != 2 {
		t.Fatal(scan, err)
	}
	v := scan.Added[0]
	if v.GameDirectory != filepath.Join(games, "versions", "healthy") || v.JavaMajor != 21 {
		t.Fatal(v)
	}
	if _, err = s.Update(v.ID, "custom name", "notes"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetSetting(v.ID, "test.plugin", json.RawMessage(`{"keep":true}`)); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(data, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if scan, err = s.Discover(false); err != nil || len(scan.Added) != 0 {
		t.Fatal(scan, err)
	}
	after, _ := os.ReadFile(filepath.Join(data, "state.json"))
	if string(saved) != string(after) || s.List()[0].Name != "custom name" || string(s.Setting(v.ID, "test.plugin")) != `{"keep":true}` {
		t.Fatal("discovery overwrote existing registration")
	}
	if err = s.Remove(v.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStoreWithMinecraftRoot(data, games)
	if err != nil {
		t.Fatal(err)
	}
	if scan, err = reopened.Discover(false); err != nil || len(scan.Added) != 0 || len(reopened.List()) != 0 {
		t.Fatal("removed version returned at startup", scan, err)
	}
	if scan, err = reopened.Discover(true); err != nil || len(scan.Added) != 1 {
		t.Fatal("explicit rescan did not restore record", scan, err)
	}
	if !reflect.DeepEqual(before, layoutSnapshot(t, games)) {
		t.Fatal("discovery or removal modified game files")
	}
}

func TestDiscoveryDoesNotScanWorkingDirectoryOrOldRoot(t *testing.T) {
	working := t.TempDir()
	t.Chdir(working)
	metadataFile(t, filepath.Join(working, ".minecraft"), "unrelated", `{"id":"unrelated","mainClass":"Main"}`)
	s := layoutStore(t)
	result, err := s.Discover(false)
	if err != nil || len(result.Added) != 0 || len(result.Issues) != 0 {
		t.Fatal(result, err)
	}
}
