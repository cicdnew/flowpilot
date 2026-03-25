package logs

import (
	"fmt"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestNetworkLoggerCapsEntries(t *testing.T) {
	nl := NewNetworkLogger("test-cap")

	for i := 0; i < MaxNetworkLogEntries+100; i++ {
		reqID := network.RequestID(fmt.Sprintf("req-%d", i))
		nl.HandleRequestWillBeSent(&network.EventRequestWillBeSent{
			RequestID: reqID,
			Request:   &network.Request{Method: "GET", URL: fmt.Sprintf("https://example.com/%d", i)},
		})
		nl.HandleResponseReceived(&network.EventResponseReceived{
			RequestID: reqID,
			Response: &network.Response{
				URL:      fmt.Sprintf("https://example.com/%d", i),
				Status:   200,
				MimeType: "text/html",
			},
		})
		nl.HandleLoadingFinished(&network.EventLoadingFinished{
			RequestID:         reqID,
			EncodedDataLength: 1024,
		}, nil)
	}

	logs := nl.Logs()
	if len(logs) > MaxNetworkLogEntries {
		t.Errorf("expected at most %d logs, got %d", MaxNetworkLogEntries, len(logs))
	}
	if len(logs) != MaxNetworkLogEntries {
		t.Errorf("expected exactly %d logs, got %d", MaxNetworkLogEntries, len(logs))
	}
}

func TestNetworkLoggerCapsPendingRequests(t *testing.T) {
	nl := NewNetworkLogger("test-pending-cap")

	for i := 0; i < MaxPendingRequests+100; i++ {
		reqID := network.RequestID(fmt.Sprintf("pending-%d", i))
		nl.HandleRequestWillBeSent(&network.EventRequestWillBeSent{
			RequestID: reqID,
			Request:   &network.Request{Method: "GET", URL: fmt.Sprintf("https://example.com/%d", i)},
		})
	}

	nl.mu.Lock()
	pendingCount := len(nl.startTimes)
	nl.mu.Unlock()

	if pendingCount > MaxPendingRequests {
		t.Errorf("pending requests should be capped at %d, got %d", MaxPendingRequests, pendingCount)
	}
}

func TestNetworkLoggerDroppedCount(t *testing.T) {
	nl := NewNetworkLogger("test-dropped")

	for i := 0; i < MaxNetworkLogEntries+50; i++ {
		reqID := network.RequestID(fmt.Sprintf("req-%d", i))
		nl.HandleRequestWillBeSent(&network.EventRequestWillBeSent{
			RequestID: reqID,
			Request:   &network.Request{Method: "GET", URL: fmt.Sprintf("https://example.com/%d", i)},
		})
		nl.HandleResponseReceived(&network.EventResponseReceived{
			RequestID: reqID,
			Response: &network.Response{
				URL:      fmt.Sprintf("https://example.com/%d", i),
				Status:   200,
				MimeType: "text/html",
			},
		})
		nl.HandleLoadingFinished(&network.EventLoadingFinished{
			RequestID:         reqID,
			EncodedDataLength: 100,
		}, nil)
	}

	nl.mu.Lock()
	dropped := nl.dropped
	nl.mu.Unlock()

	if dropped != 50 {
		t.Errorf("expected 50 dropped entries, got %d", dropped)
	}
}

func TestNetworkLoggerBasicFlow(t *testing.T) {
	nl := NewNetworkLogger("test-basic")
	nl.SetStepIndex(0)

	reqID := network.RequestID("req-1")
	nl.HandleRequestWillBeSent(&network.EventRequestWillBeSent{
		RequestID: reqID,
		Request:   &network.Request{Method: "GET", URL: "https://example.com"},
	})
	nl.HandleResponseReceived(&network.EventResponseReceived{
		RequestID: reqID,
		Response: &network.Response{
			URL:      "https://example.com",
			Status:   200,
			MimeType: "text/html",
		},
	})
	nl.HandleLoadingFinished(&network.EventLoadingFinished{
		RequestID:         reqID,
		EncodedDataLength: 2048,
	}, nil)

	logs := nl.Logs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Method != "GET" {
		t.Errorf("method: got %q, want GET", logs[0].Method)
	}
	if logs[0].StatusCode != 200 {
		t.Errorf("status: got %d, want 200", logs[0].StatusCode)
	}
	if logs[0].ResponseSize != 2048 {
		t.Errorf("response size: got %d, want 2048", logs[0].ResponseSize)
	}
}

func TestNetworkLoggerHandleLoadingFailed(t *testing.T) {
	nl := NewNetworkLogger("test-failed")

	reqID := network.RequestID("req-fail")
	nl.HandleRequestWillBeSent(&network.EventRequestWillBeSent{
		RequestID: reqID,
		Request:   &network.Request{Method: "GET", URL: "https://example.com"},
	})
	nl.HandleLoadingFailed(reqID)

	nl.mu.Lock()
	_, hasStart := nl.startTimes[reqID]
	_, hasReq := nl.requests[reqID]
	nl.mu.Unlock()

	if hasStart {
		t.Error("start time should be cleaned up after failure")
	}
	if hasReq {
		t.Error("request should be cleaned up after failure")
	}

	logs := nl.Logs()
	if len(logs) != 0 {
		t.Errorf("expected 0 logs after failure, got %d", len(logs))
	}
}
