package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

const maxMessageBytes = 1 << 20

var ErrNotImplemented = errors.New("not implemented")

type Handler interface {
	Init(ctx context.Context, config map[string]any) error
	Execute(ctx context.Context, sc StepContext) (StepResult, error)
	Hook(ctx context.Context, ev HookEvent) error
	Shutdown(ctx context.Context) error
}

type Base struct{}

func (Base) Init(context.Context, map[string]any) error { return nil }

func (Base) Execute(_ context.Context, sc StepContext) (StepResult, error) {
	return StepResult{}, fmt.Errorf("action %q: %w", sc.Step.Action, ErrNotImplemented)
}

func (Base) Hook(context.Context, HookEvent) error { return nil }

func (Base) Shutdown(context.Context) error { return nil }

var logMu sync.Mutex

func Logf(format string, args ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func Serve(h Handler) error {
	return ServeIO(context.Background(), os.Stdin, os.Stdout, h)
}

func ServeIO(ctx context.Context, r io.Reader, w io.Writer, h Handler) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), maxMessageBytes)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		var req Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			return fmt.Errorf("decode request: %w", err)
		}
		resp, stop := dispatch(ctx, h, req)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("write response: %w", err)
		}
		if stop {
			return nil
		}
	}
	return sc.Err()
}

func dispatch(ctx context.Context, h Handler, req Request) (resp Response, stop bool) {
	resp.ID = req.ID
	defer func() {
		if rec := recover(); rec != nil {
			resp.Result = nil
			resp.Error = fmt.Sprintf("plugin panic in %s: %v", req.Method, rec)
		}
	}()
	var (
		result any
		err    error
	)
	switch req.Method {
	case MethodInit:
		var p InitParams
		if err = decodeParams(req.Params, &p); err == nil {
			if p.ProtocolVersion != ProtocolVersion {
				err = fmt.Errorf("unsupported protocol version %d (plugin speaks %d)", p.ProtocolVersion, ProtocolVersion)
			} else {
				err = h.Init(ctx, p.Config)
			}
		}
	case MethodExecute:
		var p StepContext
		if err = decodeParams(req.Params, &p); err == nil {
			result, err = h.Execute(ctx, p)
		}
	case MethodHook:
		var p HookEvent
		if err = decodeParams(req.Params, &p); err == nil {
			err = h.Hook(ctx, p)
		}
	case MethodShutdown:
		err = h.Shutdown(ctx)
		stop = true
	default:
		err = fmt.Errorf("unknown method %q", req.Method)
	}
	if err != nil {
		resp.Error = err.Error()
		return resp, stop
	}
	if result != nil {
		raw, mErr := json.Marshal(result)
		if mErr != nil {
			resp.Error = fmt.Sprintf("encode result: %v", mErr)
			return resp, stop
		}
		resp.Result = raw
	}
	return resp, stop
}

func decodeParams(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("decode params: %w", err)
	}
	return nil
}
