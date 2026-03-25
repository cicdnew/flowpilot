package logs

import (
	"sync"
	"testing"

	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/network"
)

func TestNewWebSocketLogger(t *testing.T) {
	wl := NewWebSocketLogger("flow-ws")
	if wl.flowID != "flow-ws" {
		t.Errorf("flowID: got %q, want %q", wl.flowID, "flow-ws")
	}
	if len(wl.Logs()) != 0 {
		t.Errorf("initial Logs: got %d, want 0", len(wl.Logs()))
	}
}

func TestWSSetStepIndex(t *testing.T) {
	wl := NewWebSocketLogger("flow-step")
	wl.SetStepIndex(5)
	if wl.stepIndex != 5 {
		t.Errorf("stepIndex: got %d, want 5", wl.stepIndex)
	}
}

func TestHandleCreated(t *testing.T) {
	wl := NewWebSocketLogger("flow-created")
	wl.SetStepIndex(1)
	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-ws-1",
		URL:       "wss://example.com/ws",
	})

	logs := wl.Logs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	log := logs[0]
	if log.FlowID != "flow-created" {
		t.Errorf("FlowID: got %q", log.FlowID)
	}
	if log.StepIndex != 1 {
		t.Errorf("StepIndex: got %d, want 1", log.StepIndex)
	}
	if log.URL != "wss://example.com/ws" {
		t.Errorf("URL: got %q", log.URL)
	}
	if log.EventType != models.WSEventCreated {
		t.Errorf("EventType: got %q, want %q", log.EventType, models.WSEventCreated)
	}
	if log.RequestID != "req-ws-1" {
		t.Errorf("RequestID: got %q", log.RequestID)
	}
	if log.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestHandleCreatedNil(t *testing.T) {
	wl := NewWebSocketLogger("flow-nil")
	wl.HandleCreated(nil)
	if len(wl.Logs()) != 0 {
		t.Error("nil event should not produce a log")
	}
}

func TestHandleHandshake(t *testing.T) {
	wl := NewWebSocketLogger("flow-hs")
	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-hs",
		URL:       "wss://example.com/hs",
	})
	wl.HandleHandshake(&network.EventWebSocketHandshakeResponseReceived{
		RequestID: "req-hs",
	})

	logs := wl.Logs()
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[1].EventType != models.WSEventHandshake {
		t.Errorf("EventType: got %q, want %q", logs[1].EventType, models.WSEventHandshake)
	}
	if logs[1].URL != "wss://example.com/hs" {
		t.Errorf("URL: got %q", logs[1].URL)
	}
}

func TestHandleHandshakeNil(t *testing.T) {
	wl := NewWebSocketLogger("flow-nil-hs")
	wl.HandleHandshake(nil)
	if len(wl.Logs()) != 0 {
		t.Error("nil event should not produce a log")
	}
}

func TestHandleFrameSent(t *testing.T) {
	wl := NewWebSocketLogger("flow-sent")
	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-sent",
		URL:       "wss://example.com/sent",
	})
	wl.HandleFrameSent(&network.EventWebSocketFrameSent{
		RequestID: "req-sent",
		Response: &network.WebSocketFrame{
			Opcode:      1,
			PayloadData: "hello server",
		},
	})

	logs := wl.Logs()
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	frame := logs[1]
	if frame.EventType != models.WSEventFrameSent {
		t.Errorf("EventType: got %q", frame.EventType)
	}
	if frame.Direction != "send" {
		t.Errorf("Direction: got %q, want send", frame.Direction)
	}
	if frame.Opcode != 1 {
		t.Errorf("Opcode: got %d, want 1", frame.Opcode)
	}
	if frame.PayloadSize != 12 {
		t.Errorf("PayloadSize: got %d, want 12", frame.PayloadSize)
	}
	if frame.PayloadSnippet != "hello server" {
		t.Errorf("PayloadSnippet: got %q", frame.PayloadSnippet)
	}
}

func TestHandleFrameSentNilEvent(t *testing.T) {
	wl := NewWebSocketLogger("flow-nil-sent")
	wl.HandleFrameSent(nil)
	if len(wl.Logs()) != 0 {
		t.Error("nil event should not produce a log")
	}
}

func TestHandleFrameSentNilResponse(t *testing.T) {
	wl := NewWebSocketLogger("flow-nilresp-sent")
	wl.HandleFrameSent(&network.EventWebSocketFrameSent{
		RequestID: "req-nil",
		Response:  nil,
	})
	if len(wl.Logs()) != 0 {
		t.Error("nil response should not produce a log")
	}
}

func TestHandleFrameReceived(t *testing.T) {
	wl := NewWebSocketLogger("flow-recv")
	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-recv",
		URL:       "wss://example.com/recv",
	})
	wl.HandleFrameReceived(&network.EventWebSocketFrameReceived{
		RequestID: "req-recv",
		Response: &network.WebSocketFrame{
			Opcode:      1,
			PayloadData: "hello client",
		},
	})

	logs := wl.Logs()
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	frame := logs[1]
	if frame.EventType != models.WSEventFrameReceived {
		t.Errorf("EventType: got %q", frame.EventType)
	}
	if frame.Direction != "receive" {
		t.Errorf("Direction: got %q, want receive", frame.Direction)
	}
	if frame.PayloadSize != 12 {
		t.Errorf("PayloadSize: got %d, want 12", frame.PayloadSize)
	}
}

func TestHandleFrameReceivedNil(t *testing.T) {
	wl := NewWebSocketLogger("flow-nil-recv")
	wl.HandleFrameReceived(nil)
	wl.HandleFrameReceived(&network.EventWebSocketFrameReceived{
		RequestID: "req-nil",
		Response:  nil,
	})
	if len(wl.Logs()) != 0 {
		t.Error("nil events should not produce logs")
	}
}

func TestHandleClosed(t *testing.T) {
	wl := NewWebSocketLogger("flow-closed")
	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-close",
		URL:       "wss://example.com/close",
	})
	wl.HandleClosed(&network.EventWebSocketClosed{
		RequestID: "req-close",
	})

	logs := wl.Logs()
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[1].EventType != models.WSEventClosed {
		t.Errorf("EventType: got %q", logs[1].EventType)
	}

	wl.mu.Lock()
	_, exists := wl.urls["req-close"]
	wl.mu.Unlock()
	if exists {
		t.Error("URL should be cleaned up after close")
	}
}

func TestHandleClosedNil(t *testing.T) {
	wl := NewWebSocketLogger("flow-nil-close")
	wl.HandleClosed(nil)
	if len(wl.Logs()) != 0 {
		t.Error("nil event should not produce a log")
	}
}

func TestHandleFrameError(t *testing.T) {
	wl := NewWebSocketLogger("flow-err")
	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-err",
		URL:       "wss://example.com/err",
	})
	wl.HandleFrameError(&network.EventWebSocketFrameError{
		RequestID:    "req-err",
		ErrorMessage: "connection reset",
	})

	logs := wl.Logs()
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	errLog := logs[1]
	if errLog.EventType != models.WSEventError {
		t.Errorf("EventType: got %q", errLog.EventType)
	}
	if errLog.ErrorMessage != "connection reset" {
		t.Errorf("ErrorMessage: got %q", errLog.ErrorMessage)
	}
}

func TestHandleFrameErrorNil(t *testing.T) {
	wl := NewWebSocketLogger("flow-nil-err")
	wl.HandleFrameError(nil)
	if len(wl.Logs()) != 0 {
		t.Error("nil event should not produce a log")
	}
}

func TestMaxLogsLimit(t *testing.T) {
	wl := NewWebSocketLogger("flow-max")
	wl.SetMaxLogs(3)

	for i := 0; i < 10; i++ {
		wl.HandleCreated(&network.EventWebSocketCreated{
			RequestID: network.RequestID("req-" + string(rune('a'+i))),
			URL:       "wss://example.com",
		})
	}

	if len(wl.Logs()) != 3 {
		t.Errorf("expected 3 logs (max), got %d", len(wl.Logs()))
	}
}

func TestSetMaxLogsInvalidIgnored(t *testing.T) {
	wl := NewWebSocketLogger("flow-inv")
	wl.SetMaxLogs(0)
	if wl.maxLogs != defaultMaxWSLogs {
		t.Errorf("maxLogs should remain default, got %d", wl.maxLogs)
	}
	wl.SetMaxLogs(-5)
	if wl.maxLogs != defaultMaxWSLogs {
		t.Errorf("maxLogs should remain default, got %d", wl.maxLogs)
	}
}

func TestPayloadTruncation(t *testing.T) {
	wl := NewWebSocketLogger("flow-trunc")
	bigPayload := make([]byte, 1024)
	for i := range bigPayload {
		bigPayload[i] = 'x'
	}

	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-trunc",
		URL:       "wss://example.com/trunc",
	})
	wl.HandleFrameReceived(&network.EventWebSocketFrameReceived{
		RequestID: "req-trunc",
		Response: &network.WebSocketFrame{
			Opcode:      1,
			PayloadData: string(bigPayload),
		},
	})

	logs := wl.Logs()
	frame := logs[1]
	if frame.PayloadSize != 1024 {
		t.Errorf("PayloadSize: got %d, want 1024", frame.PayloadSize)
	}
	if len(frame.PayloadSnippet) != models.MaxWSPayloadSnippet {
		t.Errorf("PayloadSnippet length: got %d, want %d", len(frame.PayloadSnippet), models.MaxWSPayloadSnippet)
	}
}

func TestLogsCopyIsolation(t *testing.T) {
	wl := NewWebSocketLogger("flow-copy")
	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-copy",
		URL:       "wss://example.com",
	})

	logs1 := wl.Logs()
	logs1[0].FlowID = "mutated"

	logs2 := wl.Logs()
	if logs2[0].FlowID == "mutated" {
		t.Error("Logs() should return an isolated copy")
	}
}

func TestConcurrentAccess(t *testing.T) {
	wl := NewWebSocketLogger("flow-conc")
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			reqID := network.RequestID("req-conc-" + string(rune('A'+n%26)))
			wl.HandleCreated(&network.EventWebSocketCreated{
				RequestID: reqID,
				URL:       "wss://example.com",
			})
			wl.SetStepIndex(n)
			wl.HandleFrameReceived(&network.EventWebSocketFrameReceived{
				RequestID: reqID,
				Response: &network.WebSocketFrame{
					Opcode:      1,
					PayloadData: "test",
				},
			})
			_ = wl.Logs()
		}(i)
	}
	wg.Wait()

	if len(wl.Logs()) == 0 {
		t.Error("expected some logs from concurrent access")
	}
}

func TestFullLifecycle(t *testing.T) {
	wl := NewWebSocketLogger("flow-lifecycle")
	wl.SetStepIndex(0)

	wl.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-lc",
		URL:       "wss://example.com/lc",
	})
	wl.HandleHandshake(&network.EventWebSocketHandshakeResponseReceived{
		RequestID: "req-lc",
	})
	wl.HandleFrameSent(&network.EventWebSocketFrameSent{
		RequestID: "req-lc",
		Response:  &network.WebSocketFrame{Opcode: 1, PayloadData: "ping"},
	})
	wl.HandleFrameReceived(&network.EventWebSocketFrameReceived{
		RequestID: "req-lc",
		Response:  &network.WebSocketFrame{Opcode: 1, PayloadData: "pong"},
	})
	wl.HandleClosed(&network.EventWebSocketClosed{
		RequestID: "req-lc",
	})

	logs := wl.Logs()
	if len(logs) != 5 {
		t.Fatalf("expected 5 lifecycle logs, got %d", len(logs))
	}

	expectedTypes := []models.WebSocketEventType{
		models.WSEventCreated,
		models.WSEventHandshake,
		models.WSEventFrameSent,
		models.WSEventFrameReceived,
		models.WSEventClosed,
	}
	for i, expected := range expectedTypes {
		if logs[i].EventType != expected {
			t.Errorf("logs[%d].EventType: got %q, want %q", i, logs[i].EventType, expected)
		}
	}
}

// --- SetCallback Tests ---

func TestSetCallbackAndInvoke(t *testing.T) {
	logger := NewWebSocketLogger("ws-callback-1")

	var received []models.WebSocketLog
	var mu sync.Mutex
	logger.SetCallback(func(log models.WebSocketLog) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, log)
	})

	// Trigger a WebSocket event that fires the callback
	logger.HandleFrameReceived(&network.EventWebSocketFrameReceived{
		RequestID: "ws-callback-1",
		Timestamp: nil,
		Response: &network.WebSocketFrame{
			Opcode:      1,
			PayloadData: "callback test data",
		},
	})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 callback invocation, got %d", len(received))
	}
	if received[0].PayloadSnippet != "callback test data" {
		t.Errorf("unexpected payload: %q", received[0].PayloadSnippet)
	}
}

func TestSetCallbackNil(t *testing.T) {
	logger := NewWebSocketLogger("ws-callback-nil")

	// Set callback then set to nil - should not panic
	logger.SetCallback(func(log models.WebSocketLog) {
		t.Fatal("should not be called after nil override")
	})
	logger.SetCallback(nil)

	logger.HandleFrameReceived(&network.EventWebSocketFrameReceived{
		RequestID: "ws-callback-nil",
		Timestamp: nil,
		Response: &network.WebSocketFrame{
			Opcode:      1,
			PayloadData: "no callback",
		},
	})
}

func TestCallbackNoDeadlock(t *testing.T) {
	logger := NewWebSocketLogger("ws-deadlock")

	var received []models.WebSocketLog
	var mu sync.Mutex
	logger.SetCallback(func(log models.WebSocketLog) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, log)
		_ = logger.Logs()
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			reqID := network.RequestID("req-dl-" + string(rune('A'+n%26)))
			logger.HandleCreated(&network.EventWebSocketCreated{
				RequestID: reqID,
				URL:       "wss://example.com/dl",
			})
			logger.HandleFrameError(&network.EventWebSocketFrameError{
				RequestID:    reqID,
				ErrorMessage: "test error",
			})
			logger.HandleClosed(&network.EventWebSocketClosed{
				RequestID: reqID,
			})
		}(i)
	}
	wg.Wait()

	mu.Lock()
	count := len(received)
	mu.Unlock()
	if count == 0 {
		t.Error("expected callback invocations from concurrent events")
	}
}

func TestHandleFrameErrorCallback(t *testing.T) {
	logger := NewWebSocketLogger("ws-err-cb")
	var got models.WebSocketLog
	logger.SetCallback(func(log models.WebSocketLog) {
		got = log
	})

	logger.HandleCreated(&network.EventWebSocketCreated{
		RequestID: "req-err-cb",
		URL:       "wss://example.com/err",
	})
	logger.HandleFrameError(&network.EventWebSocketFrameError{
		RequestID:    "req-err-cb",
		ErrorMessage: "frame decode error",
	})

	if got.EventType != models.WSEventError {
		t.Errorf("expected callback with WSEventError, got %q", got.EventType)
	}
	if got.ErrorMessage != "frame decode error" {
		t.Errorf("expected error message %q, got %q", "frame decode error", got.ErrorMessage)
	}
}
