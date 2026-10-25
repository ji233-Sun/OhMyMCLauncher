package core

import (
	"errors"
	"fmt"
	"ohmymc/internal/safefile"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Process struct {
	VersionID string `json:"versionId"`
	PID       int    `json:"pid"`
	Running   bool   `json:"running"`
	StartedAt string `json:"startedAt"`
	ExitCode  *int   `json:"exitCode,omitempty"`
	LogPath   string `json:"logPath"`
}
type LogSnapshot struct {
	Lines    []string `json:"lines"`
	Running  bool     `json:"running"`
	ExitCode *int     `json:"exitCode,omitempty"`
	Path     string   `json:"path"`
}
type runningProcess struct {
	info   Process
	cmd    *exec.Cmd
	writer *logWriter
}
type Processes struct {
	mu    sync.Mutex
	items map[string]*runningProcess
	root  string
}

func NewProcesses(root string) *Processes {
	return &Processes{items: map[string]*runningProcess{}, root: root}
}
func (p *Processes) List() []Process {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []Process{}
	for _, r := range p.items {
		out = append(out, r.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}
func (p *Processes) Running(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.items[id]
	return r != nil && r.info.Running
}
func (p *Processes) Start(id string, plan LaunchPlan) (Process, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r := p.items[id]; r != nil && r.info.Running {
		return Process{}, errors.New("此版本已经运行")
	}
	if err := safefile.Component(id); err != nil {
		return Process{}, err
	}
	if err := os.MkdirAll(plan.WorkingDirectory, 0700); err != nil {
		return Process{}, err
	}
	path := filepath.Join(p.root, "logs", id, time.Now().Format("20060102-150405")+"-"+safefile.ID()[:6]+".log")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Process{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Process{}, err
	}
	w := &logWriter{file: f, lines: []string{}}
	for _, warning := range plan.Warnings {
		_, _ = w.Write([]byte("[OhMyMC] " + warning + "\n"))
	}
	cmd := exec.Command(plan.Executable, plan.Args...)
	configureProcess(cmd)
	cmd.Dir = plan.WorkingDirectory
	cmd.Stdout = w
	cmd.Stderr = w
	if err = cmd.Start(); err != nil {
		f.Close()
		return Process{}, fmt.Errorf("启动 Java 失败: %w", err)
	}
	info := Process{VersionID: id, PID: cmd.Process.Pid, Running: true, StartedAt: time.Now().UTC().Format(time.RFC3339), LogPath: path}
	r := &runningProcess{info: info, cmd: cmd, writer: w}
	p.items[id] = r
	go func() {
		err := cmd.Wait()
		code := 0
		if err != nil {
			code = -1
			if e, ok := err.(*exec.ExitError); ok {
				code = e.ExitCode()
			}
		}
		_, _ = w.Write([]byte(fmt.Sprintf("\n[OhMyMC] 游戏进程已退出，退出码 %d\n", code)))
		w.mu.Lock()
		_ = w.file.Close()
		w.mu.Unlock()
		p.mu.Lock()
		r.info.Running = false
		r.info.ExitCode = &code
		p.mu.Unlock()
	}()
	return info, nil
}
func (p *Processes) Stop(id string) error {
	p.mu.Lock()
	r := p.items[id]
	if r == nil || !r.info.Running {
		p.mu.Unlock()
		return errors.New("此版本没有运行中的进程")
	}
	p.mu.Unlock()
	err := r.cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
func (p *Processes) Logs(id string) LogSnapshot {
	p.mu.Lock()
	r := p.items[id]
	if r == nil {
		p.mu.Unlock()
		return LogSnapshot{Lines: []string{}}
	}
	info := r.info
	p.mu.Unlock()
	r.writer.mu.Lock()
	defer r.writer.mu.Unlock()
	lines := append([]string{}, r.writer.lines...)
	if r.writer.partial != "" {
		lines = append(lines, r.writer.partial)
	}
	return LogSnapshot{Lines: lines, Running: info.Running, ExitCode: info.ExitCode, Path: info.LogPath}
}

type logWriter struct {
	mu      sync.Mutex
	file    *os.File
	lines   []string
	partial string
	bytes   int64
}

func (w *logWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.bytes < 64<<20 {
		_, _ = w.file.Write(b)
		w.bytes += int64(len(b))
	}
	text := w.partial + string(b)
	parts := strings.Split(text, "\n")
	w.partial = parts[len(parts)-1]
	if len(w.partial) > 32768 {
		w.partial = w.partial[len(w.partial)-32768:]
	}
	for _, line := range parts[:len(parts)-1] {
		if len(line) > 32768 {
			line = line[:32768] + " …"
		}
		w.lines = append(w.lines, strings.TrimSuffix(line, "\r"))
	}
	if len(w.lines) > 500 {
		w.lines = append([]string{}, w.lines[len(w.lines)-500:]...)
	}
	return len(b), nil
}
