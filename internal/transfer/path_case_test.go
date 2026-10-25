package transfer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDownloadPathCaseRules(t *testing.T) {
	if downloadPathKey("libraries/A.jar", "linux") == downloadPathKey("libraries/a.jar", "linux") {
		t.Fatal("distinct case-sensitive paths merged")
	}
	if downloadPathKey("libraries/A.jar", "windows") != downloadPathKey("libraries/a.jar", "windows") {
		t.Fatal("Windows aliases not deduplicated")
	}
}

func TestCaseSensitiveDownloadsKeepBothFiles(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("filesystem integration case requires a case-sensitive platform")
	}
	c, count := fakeClient("content")
	root := t.TempDir()
	files := []File{
		{Path: "libraries/A.jar", URL: "https://libraries.minecraft.net/A.jar", SHA1: digest("content"), Size: 7},
		{Path: "libraries/a.jar", URL: "https://libraries.minecraft.net/a.jar", SHA1: digest("content"), Size: 7},
	}
	if err := c.Many(context.Background(), root, files, nil); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 2 {
		t.Fatal("files incorrectly merged")
	}
	for _, file := range files {
		if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path))); err != nil || string(b) != "content" {
			t.Fatal(file.Path, string(b), err)
		}
	}
}
