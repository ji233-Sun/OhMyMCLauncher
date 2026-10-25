package plugins

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ohmymc/internal/core"
	"ohmymc/internal/jobs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) *Manager {
	t.Helper()
	store, err := core.NewStoreWithMinecraftRoot(filepath.Join(t.TempDir(), "launcher config"), filepath.Join(t.TempDir(), ".minecraft"))
	if err != nil {
		t.Fatal(err)
	}
	j := jobs.New()
	m, err := New(store, j)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(j.Close)
	for _, p := range m.List() {
		if p.Error != "" {
			t.Fatal("builtin failed", p.Error)
		}
	}
	return m
}
func TestBuiltinSettingsIsolationAndDisable(t *testing.T) {
	m := setup(t)
	v, _ := m.Store.Add(core.Version{Name: "A", VersionID: "a", Directory: t.TempDir(), GameDirectory: t.TempDir()})
	other, _ := m.Store.Add(core.Version{Name: "B", VersionID: "b", Directory: t.TempDir(), GameDirectory: t.TempDir()})
	id := "ohmymc.launch-settings"
	load, err := m.Invoke(context.Background(), id, "load", v.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if load.(map[string]any)["values"].(map[string]any)["playerName"] != "Player" {
		t.Fatal(load)
	}
	_, err = m.Invoke(context.Background(), id, "save", v.ID, map[string]any{"values": map[string]any{"maxMemoryMB": 4096, "playerName": "Alex"}})
	if err != nil {
		t.Fatal(err)
	}
	opts, err := m.Configure(context.Background(), v)
	if err != nil || opts.MaxMemoryMB != 4096 || opts.PlayerName != "Alex" {
		t.Fatal(opts, err)
	}
	opts, err = m.Configure(context.Background(), other)
	if err != nil || opts.MaxMemoryMB != 2048 {
		t.Fatal("config leaked", opts, err)
	}
	if err = m.Toggle(id, false); err != nil {
		t.Fatal(err)
	}
	opts, err = m.Configure(context.Background(), v)
	if err != nil || opts.MaxMemoryMB != 2048 {
		t.Fatal("disabled hook ran")
	}
	if _, err = m.Invoke(context.Background(), id, "load", v.ID, nil); err == nil {
		t.Fatal("disabled plugin called")
	}
	_ = m.Toggle(id, true)
	opts, err = m.Configure(context.Background(), v)
	if err != nil || opts.MaxMemoryMB != 4096 {
		t.Fatal("settings lost")
	}
	if _, err = m.Invoke(context.Background(), id, "save", v.ID, map[string]any{"values": map[string]any{"maxMemoryMB": 1}}); err == nil {
		t.Fatal("invalid settings accepted")
	}
}
func TestRuntimePermissionsAndInterruption(t *testing.T) {
	m := setup(t)
	p := &loaded{builtin: true, trustKey: "test", manifest: Manifest{ID: "test.plugin", Name: "test"}, source: `export function invoke(){return host.call('net.json',{url:'https://piston-meta.mojang.com/a'});}`}
	if _, err := m.execute(context.Background(), p, "test", "", nil, nil, false); err == nil || !strings.Contains(err.Error(), "权限") {
		t.Fatal("permission bypass", err)
	}
	p.source = `export function invoke(){return eval('1+1');}`
	if _, err := m.execute(context.Background(), p, "test", "", nil, nil, false); err == nil {
		t.Fatal("dynamic code allowed")
	}
	p.source = `export function invoke(){while(true){}}`
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := m.execute(ctx, p, "test", "", nil, nil, false); err == nil {
		t.Fatal("infinite loop was not interrupted")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("interrupt too slow")
	}
	if err := validateScript("test", `import x from './secret.js'; export function invoke(){return x;}`); err == nil {
		t.Fatal("module import accepted")
	}
}
func TestThirdPartyConsentAndChangedCode(t *testing.T) {
	m := setup(t)
	dir := filepath.Join(m.Store.Root(), "plugins", "example")
	_ = os.MkdirAll(dir, 0700)
	manifest := `{"apiVersion":1,"id":"example.test","name":"Test","version":"1","entry":"main.js","permissions":[],"actions":{"hello":{}},"panels":[],"hooks":[]}`
	_ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0600)
	code := filepath.Join(dir, "main.js")
	_ = os.WriteFile(code, []byte(`export function invoke(){return {ok:true};}`), 0600)
	if err := m.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Invoke(context.Background(), "example.test", "hello", "", nil); err == nil {
		t.Fatal("third party enabled implicitly")
	}
	if err := m.Toggle("example.test", true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Invoke(context.Background(), "example.test", "hello", "", nil); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(code, []byte(`export function invoke(){return {ok:false};}`), 0600)
	_ = m.Reload()
	if _, err := m.Invoke(context.Background(), "example.test", "hello", "", nil); err == nil {
		t.Fatal("changed code kept consent")
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func sha(b []byte) string                                          { h := sha1.Sum(b); return hex.EncodeToString(h[:]) }
func encoded(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func fixture(t *testing.T, m *Manager) map[string][]byte {
	t.Helper()
	base := "https://piston-data.mojang.com/"
	files := map[string][]byte{}
	artifact := func(name, body string) map[string]any {
		b := []byte(body)
		files[base+name] = b
		return map[string]any{"url": base + name, "sha1": sha(b), "size": len(b)}
	}
	asset := artifact("asset", "sound")
	hash := asset["sha1"].(string)
	files["https://resources.download.minecraft.net/"+hash[:2]+"/"+hash] = []byte("sound")
	index := encoded(map[string]any{"virtual": true, "map_to_resources": true, "objects": map[string]any{"sounds/test.ogg": map[string]any{"hash": hash, "size": 5}}})
	idx := artifact("index.json", string(index))
	idx["id"] = "fixture-assets"
	var zipBody bytes.Buffer
	z := zip.NewWriter(&zipBody)
	f, _ := z.Create("fixture.dll")
	_, _ = f.Write([]byte("native"))
	_ = z.Close()
	native := artifact("native.jar", zipBody.String())
	native["path"] = "fixture/native.jar"
	lib := artifact("lib.jar", "library")
	lib["path"] = "fixture/lib.jar"
	client := artifact("client.jar", "client")
	logfile := artifact("log.xml", "<Configuration/>")
	logfile["id"] = "fixture.xml"
	meta := encoded(map[string]any{"id": "fixture", "type": "release", "mainClass": "FixtureGame", "javaVersion": map[string]any{"majorVersion": 21}, "minecraftArguments": "--username ${auth_player_name} --assetsDir ${game_assets}", "assetIndex": idx, "downloads": map[string]any{"client": client}, "logging": map[string]any{"client": map[string]any{"argument": "-Dlog4j.configurationFile=${path}", "file": logfile}}, "libraries": []any{map[string]any{"name": "g:lib:1", "downloads": map[string]any{"artifact": lib, "classifiers": map[string]any{"natives": native}}, "natives": map[string]any{"windows": "natives", "linux": "natives", "osx": "natives"}}}})
	metaArtifact := artifact("version.json", string(meta))
	metaArtifact["id"] = "fixture"
	metaArtifact["type"] = "release"
	metaArtifact["releaseTime"] = "2026-01-01T00:00:00Z"
	files["https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"] = encoded(map[string]any{"latest": map[string]string{"release": "fixture"}, "versions": []any{metaArtifact}})
	m.Network.HTTP.Transport = rtFunc(func(r *http.Request) (*http.Response, error) {
		b, ok := files[r.URL.String()]
		status := 200
		if !ok {
			status = 404
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(b)), Request: r}, nil
	})
	return files
}
func awaitJob(t *testing.T, m *Manager, id string) jobs.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range m.Jobs.List() {
			if j.ID == id && j.Status != "running" && j.Status != "queued" {
				return j
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job timeout")
	return jobs.Job{}
}
func TestVanillaPluginDownloadsRegistersAndPrepares(t *testing.T) {
	m := setup(t)
	fixture(t, m)
	ctx := context.Background()
	catalog, err := m.Invoke(ctx, "ohmymc.vanilla", "catalog", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.(map[string]any)["items"].([]any)) != 1 {
		t.Fatal(catalog)
	}
	out, err := m.Invoke(ctx, "ohmymc.vanilla", "install", "", map[string]any{"version": "fixture", "name": "测试世界", "isolate": true})
	if err != nil {
		t.Fatal(err)
	}
	job := awaitJob(t, m, out.(map[string]string)["jobId"])
	if job.Status != "completed" {
		t.Fatalf("job failed: %+v", job)
	}
	list := m.Store.List()
	if len(list) != 1 || list[0].Name != "测试世界" || list[0].JavaMajor != 21 {
		t.Fatal(list)
	}
	v := list[0]
	if v.Directory != m.Store.MinecraftRoot() || v.GameDirectory != filepath.Join(m.Store.MinecraftRoot(), "instances", v.ID) {
		t.Fatalf("download ignored the configured game root: %+v", v)
	}
	if _, err := os.Stat(filepath.Join(m.Store.Root(), "minecraft")); !os.IsNotExist(err) {
		t.Fatalf("download wrote to the legacy config-relative root: %v", err)
	}
	for _, path := range []string{filepath.Join(v.GameDirectory, "resources", "sounds", "test.ogg"), filepath.Join(v.Directory, "assets", "virtual", "fixture-assets", "sounds", "test.ogg")} {
		if b, err := os.ReadFile(path); err != nil || string(b) != "sound" {
			t.Fatal("legacy resource missing", path, err)
		}
	}
	meta, err := core.Resolve(v.Directory, v.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = core.Plan(v, meta, core.DefaultOptions(), core.Java{Path: "java", Major: 21}, core.CurrentPlatform(), t.TempDir())
	if err != nil {
		t.Fatal("downloaded version not launchable", err)
	}
	nativeDir := t.TempDir()
	if err = core.ExtractNatives(filepath.Join(v.Directory, "libraries", "fixture", "native.jar"), nativeDir, nil); err != nil {
		t.Fatal(err)
	}
}
func TestFailedDownloadNeverRegisters(t *testing.T) {
	m := setup(t)
	files := fixture(t, m)
	files["https://piston-data.mojang.com/client.jar"] = []byte("corrupt")
	out, err := m.Invoke(context.Background(), "ohmymc.vanilla", "install", "", map[string]any{"version": "fixture", "name": "bad"})
	if err != nil {
		t.Fatal(err)
	}
	j := awaitJob(t, m, out.(map[string]string)["jobId"])
	if j.Status != "failed" || len(m.Store.List()) != 0 {
		t.Fatal(fmt.Sprintf("job=%+v versions=%v", j, m.Store.List()))
	}
}
