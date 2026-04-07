package tui

import (
	"testing"
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
