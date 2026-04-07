package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInitialModel(t *testing.T) {
	m := InitialModel()

	if m.connected {
		t.Error("initial model should not be connected")
	}
	if m.viewMode != ViewModeChat {
		t.Errorf("initial view mode = %v; want %v", m.viewMode, ViewModeChat)
	}
	if len(m.commands) == 0 {
		t.Error("initial model should have commands")
	}
}

func TestModelSetConnected(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
	}{
		{"openai", "openai", "gpt-4"},
		{"anthropic", "anthropic", "claude-3"},
		{"empty model", "openai", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitialModel()
			m = m.SetConnected(true, tt.provider, tt.model)

			if !m.connected {
				t.Error("model should be connected")
			}
			if m.Provider != tt.provider {
				t.Errorf("provider = %q; want %q", m.Provider, tt.provider)
			}
			if m.modelName != tt.model {
				t.Errorf("modelName = %q; want %q", m.modelName, tt.model)
			}
		})
	}
}

func TestModelAddMessage(t *testing.T) {
	m := InitialModel()

	m = m.AddMessage("user", "Hello")
	if len(m.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(m.messages))
	}

	if m.messages[0].Role != "user" {
		t.Errorf("role = %q; want %q", m.messages[0].Role, "user")
	}
	if m.messages[0].Content != "Hello" {
		t.Errorf("content = %q; want %q", m.messages[0].Content, "Hello")
	}
	if m.messages[0].Streaming {
		t.Error("message should not be streaming by default")
	}

	// Add second message
	m = m.AddMessage("assistant", "Hi there!")
	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}
}

func TestModelAppendToLastMessage(t *testing.T) {
	t.Run("appends to existing message", func(t *testing.T) {
		m := InitialModel()
		m = m.AddMessage("assistant", "Hello")
		m = m.AppendToLastMessage(" world")

		if m.messages[0].Content != "Hello world" {
			t.Errorf("content = %q; want %q", m.messages[0].Content, "Hello world")
		}
	})

	t.Run("no-op when no messages", func(t *testing.T) {
		m := InitialModel()
		m = m.AppendToLastMessage("test")

		if len(m.messages) != 0 {
			t.Error("should not create message when none exist")
		}
	})
}

func TestModelClearMessages(t *testing.T) {
	m := InitialModel()
	m = m.AddMessage("user", "test")
	m = m.AddMessage("assistant", "response")

	m = m.ClearMessages()

	if len(m.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(m.messages))
	}
	if m.scrollPos != 0 {
		t.Errorf("scrollPos = %d; want 0", m.scrollPos)
	}
}

func TestModelFilterCommands(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		wantLen int
	}{
		{"no filter", "", 13},
		{"filter connect", "connect", 1},
		{"filter model", "model", 2},
		{"filter status", "status", 2},
		{"no match", "xyz", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitialModel()
			m.cmdFilter = tt.filter

			filtered := m.filterCommands()
			if len(filtered) != tt.wantLen {
				t.Errorf("filterCommands(%q) returned %d commands; want %d", tt.filter, len(filtered), tt.wantLen)
			}
		})
	}
}

func TestIsCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/connect", true},
		{"/help", true},
		{"  /status", true},
		{"hello", false},
		{"", false},
		{"not a command", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isCommand(tt.input)
			if got != tt.want {
				t.Errorf("isCommand(%q) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{"/connect openai key", "/connect", []string{"openai", "key"}},
		{"/status", "/status", nil},
		{"/set-model gpt-4", "/set-model", []string{"gpt-4"}},
		{"", "", nil},
		{"no-slash", "no-slash", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := ParseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("cmd = %q; want %q", cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args length = %d; want %d", len(args), len(tt.wantArgs))
				return
			}
			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q; want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

// ── StreamTokenMsg ────────────────────────────────────────────────────────

// TestModel_StreamTokenMsg_AppendsToLastAssistantMessage verifies that
// receiving a StreamTokenMsg appends the token to the last assistant message's
// content, leaving other messages untouched.
func TestModel_StreamTokenMsg_AppendsToLastAssistantMessage(t *testing.T) {
	m := InitialModel()
	m = m.AddMessage("user", "hello")
	m = m.AddMessage("assistant", "start")

	tea.NewProgram(m) // compile-time proof that Model satisfies tea.Model
	// Process the StreamTokenMsg through appendToLastAssistantMessage directly
	// (Update delegates to it — we test the helper method directly here so the
	// test does not depend on a running tea.Program loop).
	m = m.appendToLastAssistantMessage(" world")

	if len(m.messages) != 2 {
		t.Fatalf("message count = %d; want 2", len(m.messages))
	}
	if m.messages[1].Content != "start world" {
		t.Errorf("assistant content = %q; want %q", m.messages[1].Content, "start world")
	}
	// User message must be untouched.
	if m.messages[0].Content != "hello" {
		t.Errorf("user content = %q; want %q (must not be modified)", m.messages[0].Content, "hello")
	}
}

// TestModel_StreamTokenMsg_MultipleTokens verifies sequential token delivery
// builds the full response incrementally.
func TestModel_StreamTokenMsg_MultipleTokens(t *testing.T) {
	m := InitialModel()
	m = m.AddMessage("assistant", "")

	tokens := []string{"Hello", ",", " world", "!"}
	for _, tok := range tokens {
		m = m.appendToLastAssistantMessage(tok)
	}

	if got := m.messages[0].Content; got != "Hello, world!" {
		t.Errorf("assembled content = %q; want %q", got, "Hello, world!")
	}
}

// TestModel_StreamTokenMsg_NoAssistantMessage verifies that the helper is a
// safe no-op when there is no assistant message to append to.
func TestModel_StreamTokenMsg_NoAssistantMessage(t *testing.T) {
	m := InitialModel()
	m = m.AddMessage("user", "hello")

	// Must not panic or modify any message.
	m = m.appendToLastAssistantMessage("orphaned token")

	if len(m.messages) != 1 {
		t.Fatalf("message count = %d; want 1 (no new message added)", len(m.messages))
	}
	if m.messages[0].Content != "hello" {
		t.Errorf("user content = %q; want %q (must not be modified)", m.messages[0].Content, "hello")
	}
}

// TestModel_StreamTokenMsg_FindsLastAssistant verifies that the helper targets
// the LAST assistant message, not an earlier one, when multiple assistant
// messages exist in the history.
func TestModel_StreamTokenMsg_FindsLastAssistant(t *testing.T) {
	m := InitialModel()
	m = m.AddMessage("assistant", "first response")
	m = m.AddMessage("user", "follow-up")
	m = m.AddMessage("assistant", "second response")

	m = m.appendToLastAssistantMessage(" — appended")

	if m.messages[0].Content != "first response" {
		t.Errorf("first assistant content = %q; want unchanged", m.messages[0].Content)
	}
	if m.messages[2].Content != "second response — appended" {
		t.Errorf("last assistant content = %q; want %q", m.messages[2].Content, "second response — appended")
	}
}

// ── Cursor movement ───────────────────────────────────────────────────────

// TestModel_CursorLeft_MovesLeft verifies that pressing Left decrements the
// cursor by one rune position.
func TestModel_CursorLeft_MovesLeft(t *testing.T) {
	m := InitialModel()
	m.input = "hello"
	m.cursor = 5 // end of string

	newM, _ := m.handleCursorLeft()
	updated := newM.(Model)

	if updated.cursor != 4 {
		t.Errorf("cursor = %d; want 4", updated.cursor)
	}
}

// TestModel_CursorLeft_ClampsAtZero verifies that the cursor cannot go below 0.
func TestModel_CursorLeft_ClampsAtZero(t *testing.T) {
	m := InitialModel()
	m.input = "hi"
	m.cursor = 0

	newM, _ := m.handleCursorLeft()
	updated := newM.(Model)

	if updated.cursor != 0 {
		t.Errorf("cursor = %d; want 0 (clamped)", updated.cursor)
	}
}

// TestModel_CursorRight_MovesRight verifies that pressing Right increments the
// cursor by one rune position.
func TestModel_CursorRight_MovesRight(t *testing.T) {
	m := InitialModel()
	m.input = "hello"
	m.cursor = 0

	newM, _ := m.handleCursorRight()
	updated := newM.(Model)

	if updated.cursor != 1 {
		t.Errorf("cursor = %d; want 1", updated.cursor)
	}
}

// TestModel_CursorRight_ClampsAtEnd verifies that the cursor cannot exceed
// len(input).
func TestModel_CursorRight_ClampsAtEnd(t *testing.T) {
	m := InitialModel()
	m.input = "hi"
	m.cursor = 2 // already at end

	newM, _ := m.handleCursorRight()
	updated := newM.(Model)

	if updated.cursor != 2 {
		t.Errorf("cursor = %d; want 2 (clamped at end)", updated.cursor)
	}
}

// TestModel_CursorRoundTrip verifies that moving left then right returns the
// cursor to its original position.
func TestModel_CursorRoundTrip(t *testing.T) {
	m := InitialModel()
	m.input = "abcde"
	m.cursor = 3

	m1, _ := m.handleCursorLeft()
	m2, _ := m1.(Model).handleCursorRight()
	final := m2.(Model)

	if final.cursor != 3 {
		t.Errorf("cursor after left+right = %d; want 3 (original position)", final.cursor)
	}
}

// TestModel_HandleRunes_InsertsAtCursor verifies that typed characters are
// inserted at the current cursor position, not always appended.
func TestModel_HandleRunes_InsertsAtCursor(t *testing.T) {
	m := InitialModel()
	m.input = "hllo"
	m.cursor = 1 // between 'h' and 'l'

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	newM, _ := m.handleRunes(msg)
	updated := newM.(Model)

	if updated.input != "hello" {
		t.Errorf("input = %q; want %q", updated.input, "hello")
	}
	if updated.cursor != 2 {
		t.Errorf("cursor = %d; want 2 (advanced past inserted rune)", updated.cursor)
	}
}

// TestModel_HandleBackspace_DeletesAtCursor verifies that Backspace removes the
// character immediately to the left of the cursor, not always the last char.
func TestModel_HandleBackspace_DeletesAtCursor(t *testing.T) {
	m := InitialModel()
	m.input = "helo"
	m.cursor = 3 // after the extra 'l': h-e-l-|o

	newM, _ := m.handleBackspace()
	updated := newM.(Model)

	if updated.input != "heo" {
		t.Errorf("input after backspace = %q; want %q", updated.input, "heo")
	}
	if updated.cursor != 2 {
		t.Errorf("cursor = %d; want 2", updated.cursor)
	}
}
