package copilot

import "sync"

// ConversationHistory maintains an ordered list of chat messages for multi-turn
// LLM context. All methods are safe for concurrent use.
type ConversationHistory struct {
	mu       sync.RWMutex
	messages []Message
}

// Append adds a new message to the history.
func (h *ConversationHistory) Append(role, content string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, Message{Role: role, Content: content})
}

// Messages returns a snapshot copy of all messages.
// The returned slice is safe to read and modify without affecting internal state.
func (h *ConversationHistory) Messages() []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Message, len(h.messages))
	copy(out, h.messages)
	return out
}

// Clear removes all messages from the history.
func (h *ConversationHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = h.messages[:0]
}

// Trim keeps only the most recent max messages. No-op if len <= max or max <= 0.
func (h *ConversationHistory) Trim(max int) {
	if max <= 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.messages) > max {
		h.messages = h.messages[len(h.messages)-max:]
	}
}
