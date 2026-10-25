package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStateExposesSeparateGameRoot(t *testing.T) {
	data := filepath.Join(t.TempDir(), "config")
	games := filepath.Join(t.TempDir(), ".minecraft")
	s, err := NewWithMinecraftRoot(data, games)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	result, err := s.Invoke(context.Background(), "state.get", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err = json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	if state["dataRoot"] != data || state["minecraftRoot"] != games {
		t.Fatal("frontend cannot distinguish config from managed games", state)
	}
}

func TestServiceAutomaticallyRecognizesLocalVersions(t *testing.T) {
	data, games := t.TempDir(), filepath.Join(t.TempDir(), "launcher", ".minecraft")
	version := filepath.Join(games, "versions", "local")
	if err := os.MkdirAll(version, 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"local.json": `{"id":"local","mainClass":"Main","javaVersion":{"majorVersion":21}}`,
		"local.jar":  "test fixture, never executed",
	} {
		if err := os.WriteFile(filepath.Join(version, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewWithMinecraftRoot(data, games)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state := s.State()
	if len(state.Versions) != 1 || state.Versions[0].VersionID != "local" || len(state.ScanIssues) != 0 {
		t.Fatal(state)
	}
	if _, err = s.Invoke(context.Background(), "versions.remove", map[string]any{"id": state.Versions[0].ID}); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewWithMinecraftRoot(data, games)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.State().Versions) != 0 {
		t.Fatal("removed record was automatically restored")
	}
	if _, err = reopened.Invoke(context.Background(), "versions.scan", nil); err != nil {
		t.Fatal(err)
	}
	if len(reopened.State().Versions) != 1 {
		t.Fatal("rescan did not restore local version")
	}
}
