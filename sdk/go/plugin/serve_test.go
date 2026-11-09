package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type echoHandler struct {
	Base
	initConfig map[string]any
	shutdown   bool
}

func (h *echoHandler) Init(_ context.Context, cfg map[string]any) error {
	h.initConfig = cfg
	return nil
}

func (h *echoHandler) Execute(_ context.Context, sc StepContext) (StepResult, error) {
	if sc.Step.Value == "panic" {
		panic("boom")
	}
	return StepResult{Variables: map[string]string{"out": strings.ToUpper(sc.Step.Value)}}, nil
}

func (h *echoHandler) Shutdown(context.Context) error {
	h.shutdown = true
	return nil
}

func request(t *testing.T, id uint64, method string, params any) string {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(Request{ID: id, Method: method, Params: raw})
	if err != nil {
		t.Fatal(err)
	}
	return string(line) + "\n"
}

func TestServeIOProtocol(t *testing.T) {
	in := request(t, 1, MethodInit, InitParams{ProtocolVersion: ProtocolVersion, Config: map[string]any{"k": "v"}}) +
		request(t, 2, MethodExecute, StepContext{Step: Step{Action: "x", Value: "hi"}}) +
		request(t, 3, MethodExecute, StepContext{Step: Step{Action: "x", Value: "panic"}}) +
		request(t, 4, "bogus", nil) +
		request(t, 5, MethodHook, HookEvent{Point: HookPreExecute}) +
		request(t, 6, MethodShutdown, struct{}{}) +
		request(t, 7, MethodExecute, StepContext{})
	var out bytes.Buffer
	h := &echoHandler{}
	if err := ServeIO(context.Background(), strings.NewReader(in), &out, h); err != nil {
		t.Fatalf("ServeIO: %v", err)
	}

	dec := json.NewDecoder(&out)
	var resps []Response
	for dec.More() {
		var r Response
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		resps = append(resps, r)
	}
	if len(resps) != 6 {
		t.Fatalf("expected 6 responses (stop after shutdown), got %d", len(resps))
	}
	for i, r := range resps {
		if r.ID != uint64(i+1) {
			t.Fatalf("response %d has id %d", i, r.ID)
		}
	}
	if h.initConfig["k"] != "v" || resps[0].Error != "" {
		t.Fatalf("init: %+v cfg=%v", resps[0], h.initConfig)
	}
	var res StepResult
	if err := json.Unmarshal(resps[1].Result, &res); err != nil || res.Variables["out"] != "HI" {
		t.Fatalf("execute: %s %v", resps[1].Result, err)
	}
	if !strings.Contains(resps[2].Error, "plugin panic in execute: boom") {
		t.Fatalf("panic not recovered: %+v", resps[2])
	}
	if !strings.Contains(resps[3].Error, "unknown method") {
		t.Fatalf("unknown method: %+v", resps[3])
	}
	if resps[4].Error != "" || !h.shutdown {
		t.Fatalf("hook/shutdown: %+v shutdown=%v", resps[4], h.shutdown)
	}
}

func TestServeIORejectsProtocolMismatch(t *testing.T) {
	var out bytes.Buffer
	in := request(t, 1, MethodInit, InitParams{ProtocolVersion: ProtocolVersion + 1})
	if err := ServeIO(context.Background(), strings.NewReader(in), &out, &echoHandler{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unsupported protocol version") {
		t.Fatalf("protocol mismatch not reported: %s", out.String())
	}
}

func TestBaseExecuteNotImplemented(t *testing.T) {
	if _, err := (Base{}).Execute(context.Background(), StepContext{Step: Step{Action: "x"}}); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("Base.Execute: %v", err)
	}
}
