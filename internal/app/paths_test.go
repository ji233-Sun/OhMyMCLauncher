package app

import (
	"context"
	"encoding/json"
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
