package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sdk "flowpilot/sdk/go/plugin"
)

const (
	maxMessageBytes     = 1 << 20
	stderrTailBytes     = 8 << 10
	shutdownGrace       = 2 * time.Second
	defaultMaxRestarts  = 5
	restartWindow       = time.Minute
	initialRestartDelay = 100 * time.Millisecond
	maxRestartDelay     = 10 * time.Second
)

var ErrProcessExited = errors.New("plugin process exited")

type Process struct {
	manifest Manifest
	dir      string

	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan []byte
	exited  chan struct{}
	stop    chan struct{}
	stopped sync.Once
	stderr  *tailBuffer
	nextID  uint64
	broken  error
	started bool

	config       map[string]any
	shutdown     bool
	restarts     int
	restartTimes []time.Time
	nextRestart  time.Time
	backoff      time.Duration
	lastErr      string
}

type RuntimeStatus struct {
	Running   bool   `json:"running"`
	Restarts  int    `json:"restarts"`
	LastError string `json:"lastError,omitempty"`
}

func NewProcess(m Manifest, dir string) (*Process, error) {
	m.Type = TypeProcess
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if _, err := resolveEntrypoint(dir, m.Entrypoint); err != nil {
		return nil, err
	}
	return &Process{manifest: m, dir: dir}, nil
}

func (p *Process) Manifest() Manifest { return p.manifest }

func resolveEntrypoint(dir, entrypoint string) (string, error) {
	if filepath.IsAbs(entrypoint) {
		return "", fmt.Errorf("%w: entrypoint %q must be relative to the plugin directory", ErrInvalidManifest, entrypoint)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	full := filepath.Join(absDir, entrypoint)
	rel, err := filepath.Rel(absDir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: entrypoint %q escapes the plugin directory", ErrInvalidManifest, entrypoint)
	}
	return full, nil
}

func pluginEnv(id string) []string {
	env := []string{"FLOWPILOT_PLUGIN_ID=" + id}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "SYSTEMROOT", "LANG"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	return env
}

func (p *Process) start() error {
	path, err := resolveEntrypoint(p.dir, p.manifest.Entrypoint)
	if err != nil {
		return err
	}
	cmd := exec.Command(path, p.manifest.Args...)
	cmd.Dir = p.dir
	cmd.Env = pluginEnv(p.manifest.ID)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.stderr = &tailBuffer{limit: stderrTailBytes}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start plugin %s: %w", p.manifest.ID, err)
	}
	p.cmd, p.stdin, p.started = cmd, stdin, true
	p.lines = make(chan []byte)
	p.exited = make(chan struct{})
	p.stop = make(chan struct{})

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(p.lines)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64<<10), maxMessageBytes)
		for sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			select {
			case p.lines <- line:
			case <-p.stop:
				return
			}
		}
		if sc.Err() != nil {
			_ = cmd.Process.Kill()
		}
	}()
	go func() {
		<-readerDone
		_ = cmd.Wait()
		close(p.exited)
	}()
	return nil
}

func (p *Process) kill(reason error) {
	if p.broken == nil {
		p.broken = reason
		if reason != nil {
			p.lastErr = reason.Error()
		}
	}
	if p.stop != nil {
		p.stopped.Do(func() { close(p.stop) })
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}

func (p *Process) exitError() error {
	tail := strings.TrimSpace(p.stderr.String())
	if tail == "" {
		return fmt.Errorf("%w: %s", ErrProcessExited, p.manifest.ID)
	}
	return fmt.Errorf("%w: %s: %s", ErrProcessExited, p.manifest.ID, tail)
}

func (p *Process) call(ctx context.Context, method string, params, out any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callLocked(ctx, method, params, out)
}

func (p *Process) callLocked(ctx context.Context, method string, params, out any) error {
	if !p.started {
		return fmt.Errorf("plugin %s is not running", p.manifest.ID)
	}
	if p.broken == nil {
		select {
		case <-p.exited:
			p.kill(p.exitError())
		default:
		}
	}
	if p.broken != nil {
		if err := p.restartLocked(ctx); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	p.nextID++
	req, err := json.Marshal(sdk.Request{ID: p.nextID, Method: method, Params: raw})
	if err != nil {
		return err
	}
	if _, err := p.stdin.Write(append(req, '\n')); err != nil {
		p.kill(p.exitError())
		return p.broken
	}

	select {
	case <-ctx.Done():
		p.kill(fmt.Errorf("plugin %s stopped after %s call was abandoned: %w", p.manifest.ID, method, ctx.Err()))
		return fmt.Errorf("plugin %s %s: %w", p.manifest.ID, method, ctx.Err())
	case line, ok := <-p.lines:
		if !ok {
			<-p.exited
			p.kill(p.exitError())
			return p.broken
		}
		var resp sdk.Response
		if err := json.Unmarshal(line, &resp); err != nil || resp.ID != p.nextID {
			p.kill(fmt.Errorf("plugin %s: protocol error on %s response", p.manifest.ID, method))
			return p.broken
		}
		if resp.Error != "" {
			return errors.New(resp.Error)
		}
		if out != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, out); err != nil {
				return fmt.Errorf("plugin %s: decode %s result: %w", p.manifest.ID, method, err)
			}
		}
		return nil
	}
}

func (p *Process) Init(ctx context.Context, config map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return fmt.Errorf("plugin %s already started", p.manifest.ID)
	}
	p.config = config
	if err := p.start(); err != nil {
		return err
	}
	return p.initLocked(ctx)
}

func (p *Process) initLocked(ctx context.Context) error {
	return p.callLocked(ctx, sdk.MethodInit, sdk.InitParams{ProtocolVersion: sdk.ProtocolVersion, Config: p.config}, nil)
}

func (p *Process) maxRestarts() int {
	switch {
	case p.manifest.MaxRestarts < 0:
		return 0
	case p.manifest.MaxRestarts == 0:
		return defaultMaxRestarts
	default:
		return p.manifest.MaxRestarts
	}
}

func (p *Process) restartLocked(ctx context.Context) error {
	cause := p.broken
	if p.shutdown {
		return cause
	}
	limit := p.maxRestarts()
	if limit == 0 {
		return cause
	}
	now := time.Now()
	if now.Before(p.nextRestart) {
		return fmt.Errorf("%w (restart in %s)", cause, p.nextRestart.Sub(now).Round(time.Millisecond))
	}
	recent := p.restartTimes[:0]
	for _, t := range p.restartTimes {
		if now.Sub(t) < restartWindow {
			recent = append(recent, t)
		}
	}
	p.restartTimes = recent
	if len(recent) == 0 {
		p.backoff = 0
	}
	if len(recent) >= limit {
		return fmt.Errorf("%w (restart limit of %d per %s reached)", cause, limit, restartWindow)
	}

	p.kill(cause)
	<-p.exited
	p.restartTimes = append(p.restartTimes, now)
	p.restarts++
	if p.backoff == 0 {
		p.backoff = initialRestartDelay
	} else {
		p.backoff = min(2*p.backoff, maxRestartDelay)
	}
	p.nextRestart = now.Add(p.backoff)

	p.broken, p.started, p.stopped = nil, false, sync.Once{}
	if err := p.start(); err != nil {
		p.broken = fmt.Errorf("restart plugin %s: %w", p.manifest.ID, err)
		p.lastErr = p.broken.Error()
		return p.broken
	}
	if err := p.initLocked(ctx); err != nil {
		if p.broken == nil {
			p.kill(fmt.Errorf("restart plugin %s: init: %w", p.manifest.ID, err))
		}
		return p.broken
	}
	return nil
}

func (p *Process) Status() RuntimeStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	running := p.started && p.broken == nil
	if running {
		select {
		case <-p.exited:
			running = false
		default:
		}
	}
	return RuntimeStatus{Running: running, Restarts: p.restarts, LastError: p.lastErr}
}

func (p *Process) Execute(ctx context.Context, sc StepContext) (StepResult, error) {
	var res StepResult
	err := p.call(ctx, sdk.MethodExecute, sc, &res)
	return res, err
}

func (p *Process) Hook(ctx context.Context, ev HookEvent) error {
	return p.call(ctx, sdk.MethodHook, ev, nil)
}

func (p *Process) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shutdown = true
	if !p.started {
		return nil
	}
	var callErr error
	if p.broken == nil {
		callErr = p.callLocked(ctx, sdk.MethodShutdown, struct{}{}, nil)
	}
	_ = p.stdin.Close()
	select {
	case <-p.exited:
	case <-time.After(shutdownGrace):
		p.kill(errors.New("plugin shut down"))
		<-p.exited
	}
	if p.broken == nil {
		p.broken = fmt.Errorf("plugin %s has been shut down", p.manifest.ID)
	}
	return callErr
}

func (p *Process) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return false
	}
	select {
	case <-p.exited:
		return false
	default:
		return p.broken == nil
	}
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (b *tailBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, data...)
	if over := len(b.buf) - b.limit; over > 0 {
		b.buf = append([]byte(nil), b.buf[over:]...)
	}
	return len(data), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
