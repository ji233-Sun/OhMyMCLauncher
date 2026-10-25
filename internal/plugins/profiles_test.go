package plugins

import (
	"context"
	"encoding/json"
	"ohmymc/internal/core"
	"testing"
)

func TestGlobalPlayerProfilesAndPersistence(t *testing.T) {
	m := setup(t)
	ctx := context.Background()
	const id = "ohmymc.player-profiles"
	action := func(action string, data map[string]any) (any, error) { return m.Invoke(ctx, id, action, "", data) }
	for _, name := range []string{"Alex", "constructor"} {
		if _, err := action("create", map[string]any{"name": name}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"", "bad name", "bad/name", "12345678901234567", "aLeX"} {
		if _, err := action("create", map[string]any{"name": name}); err == nil {
			t.Fatal("invalid or duplicate name accepted", name)
		}
	}
	if _, err := action("select", map[string]any{"id": "Alex"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A", "B"} {
		v, err := m.Store.Add(core.Version{Name: name, VersionID: name})
		if err != nil {
			t.Fatal(err)
		}
		if err = m.Store.SetSetting(v.ID, "ohmymc.launch-settings", json.RawMessage(`{"playerName":"Legacy","useRecommendedMemory":false,"maxMemoryMB":2048}`)); err != nil {
			t.Fatal(err)
		}
		opts, err := m.Configure(ctx, v)
		if err != nil || opts.PlayerName != "Alex" {
			t.Fatal("selected identity was not global", opts, err)
		}
	}
	reopened, err := core.NewStoreWithMinecraftRoot(m.Store.Root(), m.Store.MinecraftRoot())
	if err != nil {
		t.Fatal(err)
	}
	m.Store = reopened
	loaded, err := action("load", nil)
	if err != nil {
		t.Fatal(err)
	}
	result := loaded.(map[string]any)
	if result["selectedId"] != "Alex" {
		t.Fatal("selection was not persisted", result)
	}
	profiles := result["profiles"].([]any)
	if profiles[1].(map[string]any)["uuid"] != core.OfflineUUID("Alex") {
		t.Fatal("offline UUID differs from launch UUID")
	}
	if _, err = action("remove", map[string]any{"id": "Alex"}); err != nil {
		t.Fatal(err)
	}
	if name, err := m.PlayerName(ctx); err != nil || name != "Player" {
		t.Fatal("removing current profile did not select fallback", name, err)
	}
	if _, err = action("remove", map[string]any{"id": "constructor"}); err != nil {
		t.Fatal(err)
	}
	if _, err = action("remove", map[string]any{"id": "Player"}); err == nil {
		t.Fatal("last profile removed")
	}
	if err = m.Toggle(id, false); err != nil {
		t.Fatal(err)
	}
	if name, err := m.PlayerName(ctx); err != nil || name != "Player" {
		t.Fatal(name, err)
	}
}

func TestMemoryModesAndLegacySettings(t *testing.T) {
	m := setup(t)
	ctx := context.Background()
	v, err := m.Store.Add(core.Version{Name: "test", VersionID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	const id = "ohmymc.launch-settings"
	if err = m.Store.SetSetting(v.ID, id, json.RawMessage(`{"playerName":"OldName","minMemoryMB":512,"maxMemoryMB":3072}`)); err != nil {
		t.Fatal(err)
	}
	opts, err := m.Configure(ctx, v)
	if err != nil || opts.MaxMemoryMB != 3072 || opts.PlayerName != "Player" {
		t.Fatal("legacy custom memory was lost", opts, err)
	}
	if _, err := core.SystemMemory(); err != nil {
		t.Skip("recommendation requires a system memory probe", err)
	}
	result, err := m.Invoke(ctx, id, "save", v.ID, map[string]any{"values": map[string]any{"useRecommendedMemory": true, "minMemoryMB": 1, "maxMemoryMB": 1}})
	if err != nil {
		t.Fatal(err)
	}
	values := result.(map[string]any)["values"].(map[string]any)
	if values["maxMemoryMB"].(float64) < 128 || values["useRecommendedMemory"] != true {
		t.Fatal("recommended mode trusted stale numeric inputs", result)
	}
	if _, ok := values["playerName"]; ok {
		t.Fatal("launch settings still own the player name")
	}
	opts, err = m.Configure(ctx, v)
	if err != nil || opts.MaxMemoryMB < 128 || opts.MaxMemoryMB > 8192 {
		t.Fatal(opts, err)
	}
}

func TestNewHostCapabilitiesRequirePermission(t *testing.T) {
	m := setup(t)
	for _, method := range []string{"storage.global.get", "storage.global.set", "system.memory", "identity.offline"} {
		p := &loaded{builtin: true, trustKey: "test", manifest: Manifest{ID: "test.plugin", Name: "test"}, source: `export function invoke(){return host.call('` + method + `', {name:'Alex'});}`}
		if _, err := m.execute(context.Background(), p, "test", "", nil, nil, false); err == nil {
			t.Fatal("new capability bypassed permission check", method)
		}
	}
}
