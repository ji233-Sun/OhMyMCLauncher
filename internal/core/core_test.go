package core

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func put(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func metadataFile(t *testing.T, root, id, body string) {
	t.Helper()
	put(t, filepath.Join(root, "versions", id, id+".json"), []byte(body))
}
func TestStoreImportPersistenceAndNonDestructiveRemove(t *testing.T) {
	root := t.TempDir()
	game := t.TempDir()
	metadataFile(t, game, "test", `{"id":"test","mainClass":"net.minecraft.client.main.Main","type":"release","javaVersion":{"majorVersion":21}}`)
	save := filepath.Join(game, "saves", "world", "level.dat")
	put(t, save, []byte("user-save"))
	s, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	vs, err := s.Import(game)
	if err != nil || len(vs) != 1 {
		t.Fatalf("%v %v", vs, err)
	}
	if _, err = s.Update(vs[0].ID, "我的世界", "说明"); err != nil {
		t.Fatal(err)
	}
	s2, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if s2.List()[0].Name != "我的世界" {
		t.Fatal("name not persisted")
	}
	again, err := s2.Import(filepath.Join(game, "versions", "test"))
	if err != nil || len(again) != 1 || len(s2.List()) != 1 {
		t.Fatal("duplicate import", err)
	}
	if err = s2.Remove(vs[0].ID); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(save)
	if err != nil || string(b) != "user-save" {
		t.Fatal("save altered")
	}
}
func TestCorruptDatabaseIsNotReset(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	put(t, path, []byte("bad JSON"))
	if _, err := NewStore(root); err == nil {
		t.Fatal("accepted corruption")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "bad JSON" {
		t.Fatal("database overwritten")
	}
}
func TestInheritanceAndCycle(t *testing.T) {
	root := t.TempDir()
	metadataFile(t, root, "base", `{"id":"base","mainClass":"Main","libraries":[{"name":"g:a:1"},{"name":"g:b:1"}],"arguments":{"game":["--base"]}}`)
	metadataFile(t, root, "child", `{"id":"child","inheritsFrom":"base","libraries":[{"name":"g:a:2"}],"arguments":{"game":["--child"]}}`)
	m, err := Resolve(root, "child")
	if err != nil {
		t.Fatal(err)
	}
	if m.Jar != "base" || len(m.Libraries) != 2 || m.Libraries[1].Name != "g:a:2" || len(m.Arguments.Game) != 2 {
		t.Fatalf("incorrect merge %+v", m)
	}
	metadataFile(t, root, "base", `{"id":"base","inheritsFrom":"child"}`)
	if _, err = Resolve(root, "child"); err == nil {
		t.Fatal("cycle accepted")
	}
	if _, err = Resolve(root, "../escape"); err == nil {
		t.Fatal("traversal accepted")
	}
}
func TestRulesAndArgumentExpansion(t *testing.T) {
	p := Platform{OS: "windows", Arch: "amd64", Version: "10.0", Bits: "64"}
	var rules []Rule
	if err := json.Unmarshal([]byte(`[{"action":"allow"},{"action":"disallow","os":{"name":"osx"}}]`), &rules); err != nil {
		t.Fatal(err)
	}
	if !Allowed(rules, p, nil) {
		t.Fatal("windows disallowed")
	}
	p.OS = "osx"
	if Allowed(rules, p, nil) {
		t.Fatal("osx allowed")
	}
	var args []json.RawMessage
	_ = json.Unmarshal([]byte(`["--username","${auth_player_name}",{"rules":[{"action":"allow","features":{"has_custom_resolution":true}}],"value":["--width","${resolution_width}"]},{"rules":[{"action":"allow","features":{"is_demo_user":true}}],"value":"--demo"}]`), &args)
	out, err := ExpandArguments(args, p, map[string]bool{"has_custom_resolution": true}, map[string]string{"auth_player_name": "Player", "resolution_width": "1280"})
	if err != nil || strings.Join(out, " ") != "--username Player --width 1280" {
		t.Fatalf("%v %v", out, err)
	}
	if _, err = substitute("${unknown}", nil); err == nil {
		t.Fatal("unknown token ignored")
	}
}
func TestPlanOfflineIdentityAndSpaces(t *testing.T) {
	root := filepath.Join(t.TempDir(), "space directory")
	put(t, filepath.Join(root, "versions", "v", "v.jar"), []byte("jar"))
	m := Metadata{ID: "v", Jar: "v", MainClass: "Main", MinecraftArguments: `--username ${auth_player_name} --gameDir "${game_directory}" --uuid ${auth_uuid}`}
	v := Version{ID: "a", VersionID: "v", Directory: root, GameDirectory: filepath.Join(root, "instance space")}
	o := DefaultOptions()
	o.JVMArgs = "-Dtest=space value\n-XX:+UseG1GC"
	plan, err := Plan(v, m, o, Java{Path: "java", Major: 21}, Platform{OS: "windows"}, filepath.Join(root, "natives"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, arg := range plan.Args {
		if arg == v.GameDirectory {
			found = true
		}
	}
	if !found {
		t.Fatal("path was split")
	}
	if OfflineUUID("Notch") != "b50ad385829d3141a2167e7d7539ba7f" {
		t.Fatal(OfflineUUID("Notch"))
	}
	if !strings.Contains(strings.Join(plan.Args, "\n"), "-Xmx2048M") {
		t.Fatal("memory missing")
	}
	if _, err = SplitLegacy(`"oops`); err == nil {
		t.Fatal("bad legacy quote accepted")
	}
}
func TestNativeExtractionRejectsZipSlip(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("../escape.dll")
	_, _ = f.Write([]byte("bad"))
	_ = z.Close()
	root := t.TempDir()
	archive := filepath.Join(root, "natives.jar")
	put(t, archive, b.Bytes())
	if err := ExtractNatives(archive, filepath.Join(root, "out"), nil); err == nil {
		t.Fatal("zip slip allowed")
	}
	if _, err := os.Stat(filepath.Join(root, "escape.dll")); !os.IsNotExist(err) {
		t.Fatal("escaped root")
	}
}
func TestGameHelper(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "ohmymc-test-game" {
		return
	}
	fmt.Println("fixture game started")
	fmt.Fprintln(os.Stderr, "fixture error stream")
	os.Exit(7)
}
func TestActualProcessAndLogs(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	p := NewProcesses(t.TempDir())
	info, err := p.Start("fixture", LaunchPlan{Executable: exe, Args: []string{"-test.run=^TestGameHelper$", "--", "ohmymc-test-game"}, WorkingDirectory: t.TempDir(), Warnings: []string{"fixture launch warning"}})
	if err != nil {
		t.Fatal(err)
	}
	if info.PID == 0 {
		t.Fatal("no pid")
	}
	deadline := time.Now().Add(10 * time.Second)
	for p.Running("fixture") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	log := p.Logs("fixture")
	if log.Running || log.ExitCode == nil || *log.ExitCode != 7 {
		t.Fatalf("%+v", log)
	}
	if !strings.Contains(strings.Join(log.Lines, "\n"), "fixture game started") {
		t.Fatal("stdout absent")
	}
	if !strings.Contains(strings.Join(log.Lines, "\n"), "[OhMyMC] fixture launch warning") {
		t.Fatal("launch warning absent from process logs")
	}
	if _, err := os.Stat(log.Path); err != nil {
		t.Fatal(err)
	}
}
func TestJavaProbeIfInstalled(t *testing.T) {
	all := DetectJava(context.Background())
	if len(all) == 0 {
		t.Skip("Java not installed")
	}
	for _, j := range all {
		if j.Major < 1 || j.Path == "" {
			t.Fatal(j)
		}
	}
}
