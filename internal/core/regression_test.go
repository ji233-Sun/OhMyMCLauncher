package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFailedPersistenceRollsBackNewMapEntries(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Add(Version{Name: "test", VersionID: "test", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(s.Root(), "state.json")
	if err = os.Remove(state); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	if err = s.SetEnabled("new.plugin", true); err == nil {
		t.Fatal("expected persistence failure")
	}
	if s.Enabled("new.plugin", false) {
		t.Fatal("failed enablement remained in memory")
	}
	if err = s.SetSetting(v.ID, "new.plugin", json.RawMessage(`{"value":1}`)); err == nil {
		t.Fatal("expected persistence failure")
	}
	if got := string(s.Setting(v.ID, "new.plugin")); got != "{}" {
		t.Fatal("failed setting remained in memory", got)
	}
	if len(s.List()) != 1 {
		t.Fatal("rollback damaged existing versions")
	}
}

func TestMixedLegacyAndModernInheritancePreservesBaseArguments(t *testing.T) {
	root := t.TempDir()
	metadataFile(t, root, "base", `{"id":"base","mainClass":"Main","minecraftArguments":"--username ${auth_player_name} --gameDir \"${game_directory}\" --base"}`)
	put(t, filepath.Join(root, "versions", "base", "base.jar"), []byte("jar"))
	metadataFile(t, root, "child", `{"id":"child","inheritsFrom":"base","arguments":{"jvm":["-Dtest=true"],"game":["--child"]}}`)
	meta, err := Resolve(root, "child")
	if err != nil {
		t.Fatal(err)
	}
	v := Version{ID: "instance", VersionID: "child", Directory: root, GameDirectory: filepath.Join(root, "game with spaces")}
	plan, err := Plan(v, meta, DefaultOptions(), Java{Path: "java", Major: 8}, Platform{OS: "windows"}, "native path")
	if err != nil {
		t.Fatal(err)
	}
	joined := "\n" + strings.Join(plan.Args, "\n") + "\n"
	for _, want := range []string{"-cp", filepath.Join(root, "versions", "base", "base.jar"), "-Djava.library.path=native path", "-Dtest=true", "--username", "Player", v.GameDirectory, "--base", "--child", "--width", "1280"} {
		if !strings.Contains(joined, "\n"+want+"\n") {
			t.Fatalf("missing %q in %v", want, plan.Args)
		}
	}
	// A legacy child's complete string replaces the parent's, rather than
	// appending duplicate identity/game flags after normalization.
	metadataFile(t, root, "override", `{"id":"override","inheritsFrom":"base","minecraftArguments":"--replacement ${auth_player_name}"}`)
	override, err := Resolve(root, "override")
	if err != nil {
		t.Fatal(err)
	}
	game, err := ExpandArguments(override.Arguments.Game, Platform{}, nil, map[string]string{"auth_player_name": "Player", "resolution_width": "1280", "resolution_height": "720"})
	if err != nil || strings.Contains(strings.Join(game, " "), "--base") || game[0] != "--replacement" {
		t.Fatal("legacy child did not replace parent's full string", game, err)
	}
}

func TestLegacyNativeOnlyLibraryUsesClassifierPath(t *testing.T) {
	l := Library{Name: "org.lwjgl.lwjgl:lwjgl-platform:2.9.1", Natives: map[string]string{"windows": "natives-windows-${arch}"}}
	artifact, err := LibraryArtifact(l)
	if err != nil || artifact != nil {
		t.Fatal("native-only library incorrectly requires a regular artifact", artifact, err)
	}
	native, err := NativeArtifact(l, Platform{OS: "windows", Bits: "64"})
	want := "org/lwjgl/lwjgl/lwjgl-platform/2.9.1/lwjgl-platform-2.9.1-natives-windows-64.jar"
	if err != nil || native == nil || native.Path != want {
		t.Fatal(native, err)
	}
	root := t.TempDir()
	put(t, filepath.Join(root, "libraries", filepath.FromSlash(want)), []byte("installed native"))
	put(t, filepath.Join(root, "versions", "v", "v.jar"), []byte("installed client"))
	_, err = Plan(Version{Directory: root, GameDirectory: root}, Metadata{ID: "v", Jar: "v", MainClass: "Main", Libraries: []Library{l}}, DefaultOptions(), Java{Path: "java"}, Platform{OS: "windows", Bits: "64"}, t.TempDir())
	if err != nil {
		t.Fatal("complete legacy install rejected", err)
	}
	l.Downloads.Classifiers = map[string]Artifact{"natives-linux": {Path: "linux.jar"}}
	if _, err = NativeArtifact(l, Platform{OS: "windows", Bits: "64"}); err == nil {
		t.Fatal("incomplete modern metadata silently fell back")
	}
}

func TestJavaArchitectureNormalizationAndCompatibility(t *testing.T) {
	for input, want := range map[string]string{"x86": "386", "i386": "386", "x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64"} {
		if got := normalizeJavaArch(input); got != want {
			t.Fatal(input, got, want)
		}
	}
	if err := validateJavaArchitecture(Java{Arch: runtime.GOARCH}); err != nil {
		t.Fatal(err)
	}
	for _, mismatch := range []string{"", "unsupported"} {
		if err := validateJavaArchitecture(Java{Arch: mismatch}); err == nil {
			t.Fatal("incompatible JVM accepted", mismatch)
		}
	}
	match := javaArchPattern.FindStringSubmatch("Property settings:\n    os.arch = amd64\n    os.name = Windows 11\n")
	if len(match) != 2 || match[1] != "amd64" {
		t.Fatal(match)
	}
}
