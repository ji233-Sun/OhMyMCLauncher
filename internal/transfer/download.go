package transfer

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"ohmymc/internal/safefile"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var MinecraftHosts = []string{"piston-meta.mojang.com", "piston-data.mojang.com", "launchermeta.mojang.com", "launcher.mojang.com", "libraries.minecraft.net", "resources.download.minecraft.net"}

type Client struct {
	HTTP  *http.Client
	hosts map[string]bool
}

func New(hosts []string) *Client {
	c := &Client{hosts: map[string]bool{}}
	for _, h := range hosts {
		c.hosts[strings.ToLower(h)] = true
	}
	c.HTTP = &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("重定向次数过多")
		}
		return c.CheckURL(req.URL.String())
	}}
	return c
}
func (c *Client) CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.User != nil || !c.hosts[strings.ToLower(u.Hostname())] || (u.Port() != "" && u.Port() != "443") {
		return fmt.Errorf("网络权限拒绝: %s", raw)
	}
	return nil
}
func (c *Client) get(ctx context.Context, raw string) (*http.Response, error) {
	if err := c.CheckURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OhMyMC/0.1.0")
	r, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != http.StatusOK {
		r.Body.Close()
		return nil, fmt.Errorf("下载失败 HTTP %d: %s", r.StatusCode, raw)
	}
	return r, nil
}
func (c *Client) JSON(ctx context.Context, raw, hash string) (json.RawMessage, error) {
	r, err := c.get(ctx, raw)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 16<<20+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 16<<20 {
		return nil, errors.New("远端 JSON 大小超限")
	}
	if hash != "" {
		sum := sha1.Sum(b)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), hash) {
			return nil, errors.New("JSON SHA-1 校验失败")
		}
	}
	if !json.Valid(b) {
		return nil, errors.New("服务器未返回有效 JSON")
	}
	return json.RawMessage(b), nil
}

type File struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

func validFile(f File) error {
	if err := safefile.Relative(f.Path); err != nil {
		return err
	}
	b, err := hex.DecodeString(f.SHA1)
	if err != nil || len(b) != 20 {
		return errors.New("下载必须提供有效 SHA-1: " + f.Path)
	}
	if f.Size < 0 || f.Size > 2<<30 {
		return errors.New("下载文件大小超限")
	}
	return nil
}
func downloadPathKey(path, platform string) string {
	// Windows and the usual macOS volumes are case-insensitive. Do not merge
	// distinct Linux paths merely because their spelling differs by case.
	if platform == "windows" || platform == "darwin" {
		return strings.ToLower(path)
	}
	return path
}

func (c *Client) Many(ctx context.Context, rootPath string, files []File, progress func(int, int)) error {
	if len(files) > 50000 {
		return errors.New("下载文件数量超限")
	}
	unique := map[string]string{}
	list := []File{}
	var size int64
	for _, f := range files {
		if err := validFile(f); err != nil {
			return err
		}
		if err := c.CheckURL(f.URL); err != nil {
			return err
		}
		key := downloadPathKey(f.Path, runtime.GOOS)
		if hash, ok := unique[key]; ok {
			if !strings.EqualFold(hash, f.SHA1) {
				return errors.New("下载计划包含冲突路径")
			}
			continue
		}
		unique[key] = f.SHA1
		list = append(list, f)
		size += f.Size
	}
	if size > 32<<30 {
		return errors.New("下载计划超过 32 GiB")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	queue := make(chan File)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	done := 0
	if progress != nil {
		progress(0, len(list))
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range queue {
				err := c.file(ctx, root, f)
				mu.Lock()
				if err != nil && first == nil {
					first = fmt.Errorf("%s: %w", f.Path, err)
					cancel()
				}
				if err == nil {
					done++
					if progress != nil {
						progress(done, len(list))
					}
				}
				mu.Unlock()
			}
		}()
	}
send:
	for _, f := range list {
		select {
		case <-ctx.Done():
			break send
		case queue <- f:
		}
	}
	close(queue)
	wg.Wait()
	if first != nil {
		return first
	}
	return ctx.Err()
}
func (c *Client) file(ctx context.Context, root *os.Root, f File) error {
	if r, err := root.Open(filepath.FromSlash(f.Path)); err == nil {
		h := sha1.New()
		n, e := io.Copy(h, &contextReader{ctx: ctx, r: r})
		r.Close()
		if e == nil && (f.Size == 0 || n == f.Size) && strings.EqualFold(hex.EncodeToString(h.Sum(nil)), f.SHA1) {
			return nil
		}
	}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = c.attempt(ctx, root, f)
		if err == nil {
			return nil
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 400 * time.Millisecond):
			}
		}
	}
	return err
}
func (c *Client) attempt(ctx context.Context, root *os.Root, f File) error {
	r, err := c.get(ctx, f.URL)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	path := filepath.FromSlash(f.Path)
	if err = root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".part-" + safefile.ID()
	out, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	h := sha1.New()
	limit := int64(2 << 30)
	if f.Size > 0 {
		limit = f.Size
	}
	n, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(r.Body, limit+1))
	if copyErr == nil {
		copyErr = out.Sync()
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > limit || (f.Size > 0 && n != f.Size) {
		return errors.New("文件大小校验失败")
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), f.SHA1) {
		return errors.New("SHA-1 校验失败，文件未写入正式位置")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return root.Rename(tmp, path)
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}

type Copy struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func Copies(ctx context.Context, rootPath string, items []Copy) error {
	if len(items) > 50000 {
		return errors.New("复制任务过大")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, item := range items {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = safefile.Relative(item.From); err != nil {
			return err
		}
		if err = safefile.Relative(item.To); err != nil {
			return err
		}
		f, err := root.Open(filepath.FromSlash(item.From))
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, r: f}, 128<<20+1))
		f.Close()
		if err != nil {
			return err
		}
		if len(b) > 128<<20 {
			return errors.New("资源文件超过复制限制")
		}
		if err = safefile.AtomicRoot(root, item.To, b); err != nil {
			return err
		}
	}
	return nil
}
