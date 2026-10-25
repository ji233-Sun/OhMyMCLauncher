package jobs

import (
	"context"
	"errors"
	"fmt"
	"ohmymc/internal/safefile"
	"sort"
	"sync"
	"time"
)

type Job struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Phase     string `json:"phase"`
	Done      int    `json:"done"`
	Total     int    `json:"total"`
	Error     string `json:"error"`
	CreatedAt string `json:"createdAt"`
}
type entry struct {
	Job
	key    string
	cancel context.CancelFunc
}
type Manager struct {
	mu    sync.Mutex
	items map[string]*entry
	wg    sync.WaitGroup
}

func New() *Manager { return &Manager{items: map[string]*entry{}} }

type Progress func(phase string, done, total int)

func (m *Manager) Start(key, title string, fn func(context.Context, Progress) error) (string, error) {
	m.mu.Lock()
	for _, e := range m.items {
		if e.key == key && (e.Status == "running" || e.Status == "queued") {
			m.mu.Unlock()
			return "", errors.New("该插件已有任务运行中，请等待完成或取消")
		}
	}
	// Keep a bounded session history, without discarding active jobs.
	if len(m.items) >= 100 {
		var oldest *entry
		for _, e := range m.items {
			if e.Status != "running" && e.Status != "queued" && (oldest == nil || e.CreatedAt < oldest.CreatedAt) {
				oldest = e
			}
		}
		if oldest != nil {
			delete(m.items, oldest.ID)
		}
	}
	id := safefile.ID()
	ctx, cancel := context.WithCancel(context.Background())
	e := &entry{Job: Job{ID: id, Title: title, Status: "running", Phase: "准备中", CreatedAt: time.Now().UTC().Format(time.RFC3339)}, key: key, cancel: cancel}
	m.items[id] = e
	m.wg.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.wg.Done()
		defer cancel()
		var err error
		defer func() {
			if v := recover(); v != nil {
				err = fmt.Errorf("任务异常: %v", v)
			}
			m.mu.Lock()
			defer m.mu.Unlock()
			switch {
			case ctx.Err() != nil:
				e.Status = "cancelled"
				e.Phase = "已取消"
			case err != nil:
				e.Status = "failed"
				e.Error = err.Error()
				e.Phase = "失败"
			default:
				e.Status = "completed"
				e.Phase = "已完成"
				e.Done = e.Total
			}
		}()
		err = fn(ctx, func(phase string, done, total int) {
			m.mu.Lock()
			defer m.mu.Unlock()
			if ctx.Err() == nil {
				e.Phase = phase
				e.Done = done
				e.Total = total
			}
		})
	}()
	return id, nil
}
func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Job{}
	for _, e := range m.items {
		out = append(out, e.Job)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.items[id]
	if e == nil {
		return errors.New("任务不存在")
	}
	if e.Status != "running" && e.Status != "queued" {
		return errors.New("任务已经结束")
	}
	e.Phase = "正在取消…"
	e.cancel()
	return nil
}
func (m *Manager) Busy(prefix string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.items {
		if e.key == prefix && (e.Status == "running" || e.Status == "queued") {
			return true
		}
	}
	return false
}
func (m *Manager) Close() {
	m.mu.Lock()
	for _, e := range m.items {
		e.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}
