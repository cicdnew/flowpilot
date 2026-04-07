package copilot

import (
	"fmt"
	"sync"
	"testing"
)

// TestConversationHistory_Append verifies that messages are appended in order
// and each message carries the correct role and content.
func TestConversationHistory_Append(t *testing.T) {
	h := &ConversationHistory{}

	h.Append("user", "hello")
	h.Append("assistant", "world")

	msgs := h.Messages()

	if len(msgs) != 2 {
		t.Fatalf("len = %d; want 2", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("msgs[0].Role = %q; want %q", msgs[0].Role, "user")
	}
	if msgs[0].Content != "hello" {
		t.Errorf("msgs[0].Content = %q; want %q", msgs[0].Content, "hello")
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("msgs[1].Role = %q; want %q", msgs[1].Role, "assistant")
	}
	if msgs[1].Content != "world" {
		t.Errorf("msgs[1].Content = %q; want %q", msgs[1].Content, "world")
	}
}

// TestConversationHistory_Append_Empty verifies that an empty-string content
// message is accepted (LLMs sometimes emit empty turns).
func TestConversationHistory_Append_Empty(t *testing.T) {
	h := &ConversationHistory{}

	h.Append("system", "")

	msgs := h.Messages()
	if len(msgs) != 1 {
		t.Fatalf("len = %d; want 1", len(msgs))
	}
	if msgs[0].Content != "" {
		t.Errorf("content = %q; want empty string", msgs[0].Content)
	}
}

// TestConversationHistory_Clear verifies that Clear wipes all messages and
// subsequent Append starts from zero.
func TestConversationHistory_Clear(t *testing.T) {
	h := &ConversationHistory{}
	h.Append("user", "hello")
	h.Append("assistant", "hi")

	h.Clear()

	msgs := h.Messages()
	if len(msgs) != 0 {
		t.Errorf("len after Clear = %d; want 0", len(msgs))
	}

	// Append after Clear must work normally.
	h.Append("user", "fresh start")
	if got := len(h.Messages()); got != 1 {
		t.Errorf("len after post-Clear Append = %d; want 1", got)
	}
}

// TestConversationHistory_Trim_KeepsLatest verifies that Trim(max) retains
// exactly the max most-recent messages when len > max.
func TestConversationHistory_Trim_KeepsLatest(t *testing.T) {
	h := &ConversationHistory{}
	for i := 0; i < 5; i++ {
		h.Append("user", fmt.Sprintf("msg%d", i))
	}

	h.Trim(3)

	msgs := h.Messages()
	if len(msgs) != 3 {
		t.Fatalf("len after Trim(3) = %d; want 3", len(msgs))
	}
	// The three most-recent messages are msg2, msg3, msg4.
	if msgs[0].Content != "msg2" {
		t.Errorf("msgs[0].Content = %q; want %q", msgs[0].Content, "msg2")
	}
	if msgs[1].Content != "msg3" {
		t.Errorf("msgs[1].Content = %q; want %q", msgs[1].Content, "msg3")
	}
	if msgs[2].Content != "msg4" {
		t.Errorf("msgs[2].Content = %q; want %q", msgs[2].Content, "msg4")
	}
}

// TestConversationHistory_Trim_NoopWhenBelowMax verifies that Trim is a no-op
// when the history length is already <= max.
func TestConversationHistory_Trim_NoopWhenBelowMax(t *testing.T) {
	h := &ConversationHistory{}
	h.Append("user", "only one")

	h.Trim(10)

	if got := len(h.Messages()); got != 1 {
		t.Errorf("len after Trim(10) on 1-message history = %d; want 1", got)
	}
}

// TestConversationHistory_Trim_ExactBoundary verifies that Trim(n) on a
// history of exactly n messages leaves it unchanged.
func TestConversationHistory_Trim_ExactBoundary(t *testing.T) {
	h := &ConversationHistory{}
	for i := 0; i < 4; i++ {
		h.Append("user", fmt.Sprintf("m%d", i))
	}

	h.Trim(4)

	if got := len(h.Messages()); got != 4 {
		t.Errorf("len after Trim(4) on 4-message history = %d; want 4", got)
	}
}

// TestConversationHistory_Trim_ZeroOrNegative verifies that non-positive max
// values are treated as a no-op (defensive guard).
func TestConversationHistory_Trim_ZeroOrNegative(t *testing.T) {
	h := &ConversationHistory{}
	h.Append("user", "kept")
	h.Append("assistant", "also kept")

	h.Trim(0)
	if got := len(h.Messages()); got != 2 {
		t.Errorf("Trim(0): len = %d; want 2 (no-op)", got)
	}

	h.Trim(-5)
	if got := len(h.Messages()); got != 2 {
		t.Errorf("Trim(-5): len = %d; want 2 (no-op)", got)
	}
}

// TestConversationHistory_Messages_ReturnsCopy verifies that the slice returned
// by Messages() is a defensive copy — mutating it must not affect internal state.
func TestConversationHistory_Messages_ReturnsCopy(t *testing.T) {
	h := &ConversationHistory{}
	h.Append("user", "original")

	snapshot := h.Messages()
	snapshot[0].Content = "tampered"

	fresh := h.Messages()
	if fresh[0].Content != "original" {
		t.Errorf("Messages() returned a reference, not a copy; mutation changed internal state to %q", fresh[0].Content)
	}
}

// TestConversationHistory_ThreadSafe spawns 50 concurrent goroutines that each
// call Append while 10 other goroutines call Messages() — the race detector
// must not report any data race, and the final count must equal 50.
func TestConversationHistory_ThreadSafe(t *testing.T) {
	h := &ConversationHistory{}
	var wg sync.WaitGroup

	const writers = 50
	const readers = 10

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Append("user", "concurrent write")
		}()
	}

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.Messages()
		}()
	}

	wg.Wait()

	if got := len(h.Messages()); got != writers {
		t.Errorf("len after %d concurrent Appends = %d; want %d", writers, got, writers)
	}
}

// TestConversationHistory_Trim_ThreadSafe exercises concurrent Trim and Append
// to verify no data races on the internal slice pointer swap.
func TestConversationHistory_Trim_ThreadSafe(t *testing.T) {
	h := &ConversationHistory{}
	for i := 0; i < 100; i++ {
		h.Append("user", fmt.Sprintf("msg%d", i))
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Trim(50)
		}()
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Append("user", "late message")
		}()
	}
	wg.Wait()

	// After concurrent trims and appends, the history must be non-empty and
	// not in a corrupted state (just call Messages without panicking).
	msgs := h.Messages()
	if len(msgs) == 0 {
		t.Error("history is empty after concurrent Trim+Append; expected at least some messages")
	}
}
