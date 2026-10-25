package app

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"ohmymc/internal/core"
)

// Exercises the public service through a real JVM. No network or Minecraft files
// are needed. A machine without a JDK skips this integration test explicitly.
func TestJavaLaunchThroughService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var java core.Java
	var javac string
	for _, candidate := range core.DetectJava(ctx) {
		name := "javac"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path := filepath.Join(filepath.Dir(candidate.Path), name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && candidate.Major >= 8 {
			java, javac = candidate, path
			break
		}
	}
	if javac == "" {
		t.Skip("a JDK 8+ is required for the real Java launch fixture")
	}
	classes := t.TempDir()
	args := []string{"-d", classes}
	if java.Major >= 9 {
		args = append(args, "--release", "8")
	}
	args = append(args, filepath.Join("testdata", "LaunchFixture.java"))
	if out, err := exec.CommandContext(ctx, javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile Java fixture: %v\n%s", err, out)
	}
	game := filepath.Join(t.TempDir(), "game directory with spaces")
	versionDir := filepath.Join(game, "versions", "fixture")
	if err := os.MkdirAll(versionDir, 0700); err != nil {
		t.Fatal(err)
	}
	meta := map[string]any{
		"id": "fixture", "type": "release", "mainClass": "LaunchFixture",
		"javaVersion": map[string]int{"majorVersion": 8},
		"arguments": map[string]any{
			"jvm":  []string{"-Djava.library.path=${natives_directory}", "-cp", "${classpath}"},
			"game": []string{"${auth_player_name}", "${auth_uuid}", "${game_directory}"},
		},
	}
	body, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(versionDir, "fixture.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	class, err := os.ReadFile(filepath.Join(classes, "LaunchFixture.class"))
	if err != nil {
		t.Fatal(err)
	}
	jar, err := os.Create(filepath.Join(versionDir, "fixture.jar"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(jar)
	entry, err := z.Create("LaunchFixture.class")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write(class); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = jar.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	imported, err := s.Invoke(ctx, "versions.import", map[string]any{"path": game})
	if err != nil {
		t.Fatal(err)
	}
	v := imported.([]core.Version)[0]
	if _, err = s.Invoke(ctx, "plugins.invoke", map[string]any{
		"id": "ohmymc.player-profiles", "action": "create", "data": map[string]any{"name": "FixturePlayer"},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Invoke(ctx, "plugins.invoke", map[string]any{
		"id": "ohmymc.launch-settings", "action": "save", "versionId": v.ID,
		"data": map[string]any{"values": map[string]any{
			"playerName": "IgnoredOldName", "javaPath": java.Path, "useRecommendedMemory": false,
			"minMemoryMB": 128, "maxMemoryMB": 256,
			"jvmArgs": "-Dohmymc.fixture=with spaces", "gameArgs": "one argument with spaces",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.Invoke(ctx, "versions.preview", map[string]any{"id": v.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(preview.(core.LaunchPlan).Args, "\n"), "-Xmx256M") {
		t.Fatal("launch.configure was not applied")
	}
	if !strings.Contains(strings.Join(preview.(core.LaunchPlan).Args, "\n"), "FixturePlayer") {
		t.Fatal("preview did not use the selected global player profile")
	}
	if _, err = s.Invoke(ctx, "versions.launch", map[string]any{"id": v.ID}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if s.Processes.Running(v.ID) {
			_ = s.Processes.Stop(v.ID)
		}
	}()
	for s.Processes.Running(v.ID) && ctx.Err() == nil {
		time.Sleep(20 * time.Millisecond)
	}
	logs := s.Processes.Logs(v.ID)
	if logs.Running || logs.ExitCode == nil || *logs.ExitCode != 0 {
		t.Fatalf("JVM did not exit successfully: %+v", logs)
	}
	text := strings.Join(logs.Lines, "\n")
	for _, want := range []string{
		"OHMYMC_JAVA_OK", "OHMYMC_STDERR_OK", "property=with spaces", "cwd=" + game,
		"arg0=FixturePlayer", "arg1=" + core.OfflineUUID("FixturePlayer"),
		"arg2=" + game, "arg3=one argument with spaces",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in JVM output:\n%s", want, text)
		}
	}
	played, err := s.Store.Get(v.ID)
	if err != nil || played.LastPlayed == "" {
		t.Fatalf("launch record was not persisted: %+v, %v", played, err)
	}
}
