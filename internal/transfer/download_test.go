package transfer

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func digest(s string) string                                              { h := sha1.Sum([]byte(s)); return hex.EncodeToString(h[:]) }
func fakeClient(body string) (*Client, *atomic.Int32) {
	n := &atomic.Int32{}
	c := New(MinecraftHosts)
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		n.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	return c, n
}
func TestVerifiedDownloadAndCache(t *testing.T) {
	c, n := fakeClient("content")
	root := t.TempDir()
	f := File{Path: "libraries/g/a.jar", URL: "https://libraries.minecraft.net/g/a.jar", SHA1: digest("content"), Size: 7}
	done := 0
	if err := c.Many(context.Background(), root, []File{f, f}, func(d, total int) {
		done = d
		if total != 1 {
			t.Errorf("not deduped")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if done != 1 {
		t.Fatal("progress wrong")
	}
	if err := c.Many(context.Background(), root, []File{f}, nil); err != nil {
		t.Fatal(err)
	}
	if n.Load() != 1 {
		t.Fatal("cache not used")
	}
	b, _ := os.ReadFile(filepath.Join(root, "libraries", "g", "a.jar"))
	if string(b) != "content" {
		t.Fatal("bad file")
	}
}
func TestChecksumFailureNeverCommits(t *testing.T) {
	c, _ := fakeClient("wrong")
	root := t.TempDir()
	err := c.Many(context.Background(), root, []File{{Path: "client.jar", URL: "https://piston-data.mojang.com/client.jar", SHA1: digest("right"), Size: 5}}, nil)
	if err == nil {
		t.Fatal("bad checksum accepted")
	}
	if _, err = os.Stat(filepath.Join(root, "client.jar")); !os.IsNotExist(err) {
		t.Fatal("unverified file committed")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("partial file leaked")
	}
}
func TestCapabilitiesAndCancellation(t *testing.T) {
	c, _ := fakeClient("test")
	for _, url := range []string{"http://libraries.minecraft.net/a", "https://evil.test/a", "https://libraries.minecraft.net:8443/a", "https://user:pass@libraries.minecraft.net/a"} {
		if err := c.CheckURL(url); err == nil {
			t.Fatal("accepted", url)
		}
	}
	if err := c.Many(context.Background(), t.TempDir(), []File{{Path: "../escape", URL: "https://libraries.minecraft.net/a", SHA1: digest("test")}}, nil); err == nil {
		t.Fatal("traversal accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Many(ctx, t.TempDir(), []File{{Path: "a.jar", URL: "https://libraries.minecraft.net/a", SHA1: digest("test")}}, nil); err == nil {
		t.Fatal("cancellation ignored")
	}
}
func TestJSONIntegrity(t *testing.T) {
	c, _ := fakeClient(`{"ok":true}`)
	if _, err := c.JSON(context.Background(), "https://piston-meta.mojang.com/a", digest(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.JSON(context.Background(), "https://piston-meta.mojang.com/a", digest("wrong")); err == nil {
		t.Fatal("bad json digest accepted")
	}
}
