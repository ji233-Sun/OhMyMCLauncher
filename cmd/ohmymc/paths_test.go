package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultGamesBesideExecutable(t *testing.T) {
	// A shortcut or shell can start the launcher from an unrelated directory.
	t.Chdir(t.TempDir())
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := resolvePaths("", "")
	if err != nil {
		t.Fatal(err)
	}
	wantGames := filepath.Join(filepath.Dir(executable), ".minecraft")
	if paths.Games != wantGames || paths.Data != filepath.Join(config, "OhMyMC") {
		t.Fatalf("unexpected defaults: %+v; games must be %s", paths, wantGames)
	}
	paths, err = resolvePaths(filepath.Join(t.TempDir(), "separate config"), "")
	if err != nil || paths.Games != wantGames {
		t.Fatalf("--data changed the game root: %+v, %v", paths, err)
	}
	if _, err := os.Stat(".minecraft"); !os.IsNotExist(err) {
		t.Fatalf("path resolution wrote into the working directory: %v", err)
	}
}

func TestExplicitLauncherPaths(t *testing.T) {
	working := t.TempDir()
	t.Chdir(working)
	paths, err := resolvePaths("配置 目录", filepath.Join("游戏 目录", ".minecraft"))
	if err != nil {
		t.Fatal(err)
	}
	want := launcherPaths{
		Data:  filepath.Join(working, "配置 目录"),
		Games: filepath.Join(working, "游戏 目录", ".minecraft"),
	}
	if paths != want {
		t.Fatalf("relative overrides: %+v, want %+v", paths, want)
	}
	absolute, err := resolvePaths(want.Data, want.Games)
	if err != nil || absolute != want {
		t.Fatalf("absolute overrides: %+v, %v", absolute, err)
	}
}
