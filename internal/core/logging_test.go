package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedLoggingConfigurationFallback(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		file          string
		wantError     bool
		wantWarning   bool
	}{
		{"modern missing XML", "2.22.1", "missing", false, true},
		{"lowest accepted release", "2.17.1", "missing", false, true},
		{"existing XML", "2.22.1", "regular", false, false},
		{"not a regular file", "2.22.1", "directory", true, false},
		{"legacy requires XML", "2.14.1", "missing", true, false},
		{"old patched branch remains strict", "2.12.4", "missing", true, false},
		{"prerelease remains strict", "2.22.1-beta", "missing", true, false},
		{"legacy with XML", "2.14.1", "regular", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			v := Version{VersionID: "fixture", Directory: root, GameDirectory: root}
			m := Metadata{ID: "fixture", Jar: "fixture", MainClass: "Main", Libraries: []Library{{Name: "org.apache.logging.log4j:log4j-core:" + tc.version}}}
			m.Logging.Client.Argument = "-Dlog4j.configurationFile=${path}"
			m.Logging.Client.File.ID = "client-1.12.xml"
			lib, err := MavenPath(m.Libraries[0].Name)
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(root, "libraries", filepath.FromSlash(lib)), []byte("fixture library"))
			put(t, filepath.Join(root, "versions", "fixture", "fixture.jar"), []byte("fixture client"))
			config := filepath.Join(root, "assets", "log_configs", "client-1.12.xml")
			switch tc.file {
			case "regular":
				put(t, config, []byte("<Configuration/>"))
			case "directory":
				if err := os.MkdirAll(config, 0700); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := Plan(v, m, DefaultOptions(), Java{Path: "java", Major: 21}, CurrentPlatform(), t.TempDir())
			if (err != nil) != tc.wantError || (len(plan.Warnings) > 0) != tc.wantWarning {
				t.Fatalf("unexpected logging behavior: warnings=%v, err=%v", plan.Warnings, err)
			}
			if tc.wantWarning && strings.Contains(strings.Join(plan.Args, "\n"), "-Dlog4j.configurationFile=") {
				t.Fatal("passed a nonexistent logging file to Java")
			}
			if tc.file == "regular" && !strings.Contains(strings.Join(plan.Args, "\n"), "-Dlog4j.configurationFile="+config) {
				t.Fatal("existing logging configuration was ignored")
			}
			if tc.file == "missing" {
				if _, err := os.Stat(config); !os.IsNotExist(err) {
					t.Fatal("preview created a logging configuration", err)
				}
			}
		})
	}
}

func TestLoggingFallbackRejectsUnknownOrMixedVersions(t *testing.T) {
	modern := Library{Name: "org.apache.logging.log4j:log4j-core:2.22.1"}
	old := Library{Name: "org.apache.logging.log4j:log4j-core:2.14.1"}
	for _, libraries := range [][]Library{nil, {{Name: "other:library:2.22.1"}}, {{Name: "org.apache.logging.log4j:log4j-core:unknown"}}, {modern, old}, {old, modern}} {
		if hasModernLog4j(libraries, CurrentPlatform()) {
			t.Fatal("unsafe or unknown Log4j allowed a fallback", libraries)
		}
	}
}
