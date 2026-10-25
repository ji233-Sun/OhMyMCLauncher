package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func layoutStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStoreWithMinecraftRoot(filepath.Join(t.TempDir(), "config"), filepath.Join(t.TempDir(), ".minecraft"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func layoutSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			out[path] = "directory"
			return nil
		}
		b, err := os.ReadFile(path)
		out[path] = fmt.Sprintf("%x", sha256.Sum256(b))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestImportLauncherLayout(t *testing.T) {
	launcher := filepath.Join(t.TempDir(), "MC 启动器")
	root := filepath.Join(launcher, ".minecraft")
	isolated, shared := "整合包 with spaces", "原版"
	for _, id := range []string{isolated, shared} {
		metadataFile(t, root, id, fmt.Sprintf(`{"id":%q,"mainClass":"Main","javaVersion":{"majorVersion":21}}`, id))
	}
	put(t, filepath.Join(root, "versions", isolated, "mods", "fixture.jar"), []byte("fixture only"))
	put(t, filepath.Join(root, "saves", "world", "level.dat"), []byte("leave shared saves alone"))
	// Some installations contain another .minecraft inside the real root.
	metadataFile(t, filepath.Join(root, ".minecraft"), "nested", `{"id":"nested","mainClass":"Main"}`)
	before := layoutSnapshot(t, launcher)
	s := layoutStore(t)
	for _, tc := range []struct {
		name, path string
		count      int
	}{
		{"launcher", launcher, 2},
		{"minecraft", root, 2},
		{"versions", filepath.Join(root, "versions"), 2},
		{"isolated version", filepath.Join(root, "versions", isolated), 1},
		{"shared version", filepath.Join(root, "versions", shared), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			versions, err := s.Import(tc.path)
			if err != nil || len(versions) != tc.count {
				t.Fatalf("import: %v, %v", versions, err)
			}
			for _, v := range versions {
				game := root
				if v.VersionID == isolated {
					game = filepath.Join(root, "versions", isolated)
				}
				if v.Directory != root || v.GameDirectory != game || v.JavaMajor != 21 {
					t.Fatalf("incorrect imported paths: %+v", v)
				}
			}
		})
	}
	if len(s.List()) != 2 {
		t.Fatal("path aliases caused duplicate registrations", s.List())
	}
	if after := layoutSnapshot(t, launcher); !reflect.DeepEqual(before, after) {
		t.Fatal("import modified the source installation")
	}
}

func TestImportedGameDirectoryMarkers(t *testing.T) {
	for _, marker := range []string{"saves", "mods", "config", "defaultconfigs", "resourcepacks", "shaderpacks", "screenshots", "resources", "options.txt", "launcher caches only", "file named saves"} {
		t.Run(marker, func(t *testing.T) {
			root := t.TempDir()
			id := "尚无存档的整合包"
			dir := filepath.Join(root, "versions", id)
			metadataFile(t, root, id, fmt.Sprintf(`{"id":%q,"mainClass":"Main"}`, id))
			want := dir
			switch marker {
			case "launcher caches only":
				put(t, filepath.Join(dir, "PCL", "Setup.ini"), []byte("fixture"))
				put(t, filepath.Join(dir, id+"-natives", "fixture.dll"), []byte("fixture"))
				want = root
			case "file named saves":
				put(t, filepath.Join(dir, "saves"), []byte("not a directory"))
				want = root
			case "options.txt":
				put(t, filepath.Join(dir, marker), []byte("fixture"))
			default:
				if err := os.MkdirAll(filepath.Join(dir, marker), 0700); err != nil {
					t.Fatal(err)
				}
			}
			versions, err := layoutStore(t).Import(root)
			if err != nil || len(versions) != 1 || versions[0].GameDirectory != want {
				t.Fatalf("game directory: %v, %v; want %s", versions, err, want)
			}
		})
	}
}

func TestChangingManagedRootPreservesExistingVersions(t *testing.T) {
	data := t.TempDir()
	old, err := NewStore(data) // Simulate the old configuration-relative layout.
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), ".minecraft")
	for _, root := range []string{old.MinecraftRoot(), external} {
		metadataFile(t, root, "v", `{"id":"v","mainClass":"Main"}`)
		vs, err := old.Import(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = old.Update(vs[0].ID, "已有版本", "保留说明"); err != nil {
			t.Fatal(err)
		}
		if err = old.SetSetting(vs[0].ID, "plugin.test", json.RawMessage(`{"memory":4096}`)); err != nil {
			t.Fatal(err)
		}
	}
	before := old.List()
	games := filepath.Join(t.TempDir(), "launcher", ".minecraft")
	current, err := NewStoreWithMinecraftRoot(data, games)
	if err != nil {
		t.Fatal(err)
	}
	if current.Root() != data || current.MinecraftRoot() != games || !reflect.DeepEqual(before, current.List()) {
		t.Fatal("changed roots rewrote version paths or metadata")
	}
	for _, v := range before {
		// A later directory marker must not rewrite an existing registration.
		put(t, filepath.Join(v.Directory, "versions", v.VersionID, "config", "fixture.txt"), []byte("fixture"))
		again, err := current.Import(v.Directory)
		if err != nil || len(again) != 1 || again[0] != v {
			t.Fatalf("existing registration changed on reimport: %v, %v", again, err)
		}
		var setting struct {
			Memory int `json:"memory"`
		}
		if err := json.Unmarshal(current.Setting(v.ID, "plugin.test"), &setting); err != nil || setting.Memory != 4096 {
			t.Fatal("plugin settings lost", setting, err)
		}
	}
	entries, err := os.ReadDir(games)
	if err != nil || len(entries) != 0 {
		t.Fatal("existing games were moved or copied", err)
	}
	if _, err := os.Stat(filepath.Join(data, "state.json")); err != nil {
		t.Fatal("launcher state moved out of the configuration root", err)
	}
	if _, err := NewStoreWithMinecraftRoot(t.TempDir(), " "); err == nil {
		t.Fatal("blank game root accepted")
	}
}

func planValue(t *testing.T, args []string, key string) string {
	t.Helper()
	index := slices.Index(args, key)
	if index < 0 || index+1 == len(args) {
		t.Fatalf("missing launch argument %s", key)
	}
	return args[index+1]
}

func TestIsolatedModpackUsesSharedLibrariesAndAssets(t *testing.T) {
	for _, id := range []string{"整合包 中文", "Modpack with spaces"} {
		t.Run(id, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "MC with spaces", ".minecraft")
			m := Metadata{ID: id, MainClass: "cpw.mods.bootstraplauncher.BootstrapLauncher", Type: "release"}
			m.JavaVersion.MajorVersion = 21
			m.AssetIndex.ID = "17"
			m.Libraries = []Library{{Name: "cpw.mods:bootstraplauncher:2.0.2"}, {Name: "cpw.mods:securejarhandler:3.0.8"}}
			modules := []string{}
			for _, lib := range m.Libraries {
				path, err := MavenPath(lib.Name)
				if err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(root, "libraries", filepath.FromSlash(path)), []byte("fixture jar"))
				modules = append(modules, "${library_directory}/"+path)
			}
			m.Arguments.JVM = rawArguments("-Djava.library.path=${natives_directory}", "-cp", "${classpath}", "-DlibraryDirectory=${library_directory}", "-DignoreList=client-extra,${version_name}.jar", "-p", strings.Join(modules, "${classpath_separator}"), "--add-modules", "ALL-MODULE-PATH")
			m.Arguments.Game = rawArguments("--gameDir", "${game_directory}", "--assetsDir", "${assets_root}", "--assetIndex", "${assets_index_name}", "--fml.mcVersion", "1.21.1", "--launchTarget", "forgeclient")
			b, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			metadataFile(t, root, id, string(b))
			put(t, filepath.Join(root, "versions", id, id+".jar"), []byte("fixture client"))
			put(t, filepath.Join(root, "versions", id, "mods", "fixture.jar"), []byte("fixture mod"))
			put(t, filepath.Join(root, "assets", "indexes", "17.json"), []byte(`{"objects":{}}`))
			before := layoutSnapshot(t, root)
			versions, err := layoutStore(t).Import(root)
			if err != nil {
				t.Fatal(err)
			}
			v := versions[0]
			m, err = Resolve(root, id)
			if err != nil {
				t.Fatal(err)
			}
			natives := filepath.Join(t.TempDir(), "not created")
			plan, err := Plan(v, m, DefaultOptions(), Java{Path: "java", Major: 21}, CurrentPlatform(), natives)
			if err != nil {
				t.Fatal(err)
			}
			if plan.WorkingDirectory != v.GameDirectory || planValue(t, plan.Args, "--gameDir") != filepath.Join(root, "versions", id) || planValue(t, plan.Args, "--assetsDir") != filepath.Join(root, "assets") {
				t.Fatal("shared resources and isolated game directory were mixed up", plan)
			}
			classpath := filepath.SplitList(planValue(t, plan.Args, "-cp"))
			if len(classpath) != 3 || classpath[2] != filepath.Join(root, "versions", id, id+".jar") {
				t.Fatal("classpath split or wrong client jar", classpath)
			}
			for index, path := range filepath.SplitList(planValue(t, plan.Args, "-p")) {
				if filepath.Clean(path) != classpath[index] {
					t.Fatal("incorrect module path", path)
				}
			}
			if strings.Contains(strings.Join(plan.Args, "\n"), "${") || !slices.Contains(plan.Args, "-DignoreList=client-extra,"+id+".jar") {
				t.Fatal("NeoForge placeholders were not preserved/expanded")
			}
			if after := layoutSnapshot(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("launch preview modified the imported files")
			}
			if _, err = os.Stat(natives); !os.IsNotExist(err) {
				t.Fatal("preview extracted natives", err)
			}
		})
	}
}

// Opt-in inspection only: registration and all generated files stay in TempDir.
// It does not run Java, prepare natives, download files or write to the source.
func TestExistingInstallationReadOnly(t *testing.T) {
	path := os.Getenv("OHMYMC_COMPAT_ROOT")
	if path == "" {
		t.Skip("set OHMYMC_COMPAT_ROOT to inspect an existing installation read-only")
	}
	versions, err := layoutStore(t).Import(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range versions {
		t.Run(v.VersionID, func(t *testing.T) {
			m, err := Resolve(v.Directory, v.VersionID)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := Plan(v, m, DefaultOptions(), Java{Path: "not-executed", Major: m.JavaVersion.MajorVersion}, CurrentPlatform(), filepath.Join(t.TempDir(), "natives-not-created"))
			if err != nil {
				t.Fatal(err)
			}
			for index, arg := range plan.Args {
				if (arg == "-p" || arg == "--module-path") && index+1 < len(plan.Args) {
					for _, path := range filepath.SplitList(plan.Args[index+1]) {
						if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
							t.Fatalf("missing module: %s (%v)", path, err)
						}
					}
				}
			}
			if strings.Contains(strings.Join(plan.Args, "\n"), "${") {
				t.Fatal("unresolved launch placeholder")
			}
			t.Logf("root=%s; game=%s; Java=%d; libraries=%d; main=%s", v.Directory, plan.WorkingDirectory, plan.JavaMajor, len(m.Libraries), m.MainClass)
		})
	}
}
