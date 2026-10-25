package safefile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Relative rejects traversal and Windows alternate streams on every platform.
func Relative(p string) error {
	if p == "" || strings.ContainsAny(p, "\\:\x00") || !filepath.IsLocal(filepath.FromSlash(p)) {
		return errors.New("不安全的相对路径: " + p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return errors.New("不安全的路径分段")
		}
	}
	return nil
}
func Component(s string) error {
	if strings.Contains(s, "/") {
		return errors.New("名称不能含路径分隔符")
	}
	return Relative(s)
}

func Atomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ohmymc-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}

// AtomicRoot uses os.Root so symbolic links cannot escape a capability's directory.
func AtomicRoot(root *os.Root, path string, b []byte) error {
	if err := Relative(path); err != nil {
		return err
	}
	p := filepath.FromSlash(path)
	if err := root.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	tmp := p + ".tmp-" + ID()
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	return root.Rename(tmp, p)
}
