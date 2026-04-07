package tui

import (
	"testing"
)

func TestInitializeCommands(t *testing.T) {
	cmds := InitializeCommands()

	expectedCommands := []string{
		"/connect", "/models", "/set-model",
		"/status", "/clear", "/help", "/exit",
	}

	expectedCommands = append(expectedCommands,
		"/worker-start", "/worker-stop", "/worker-status",
		"/headless", "/no-headless", "/run-task",
	)

	if len(cmds) != len(expectedCommands) {
		t.Errorf("InitializeCommands returned %d commands; want %d", len(cmds), len(expectedCommands))
	}

	for i, expected := range expectedCommands {
		if i < len(cmds) && cmds[i].Name != expected {
			t.Errorf("commands[%d].Name = %q; want %q", i, cmds[i].Name, expected)
		}
	}
}

func TestGetCommandActions(t *testing.T) {
	actions := GetCommandActions()

	expectedActions := []string{
		"/connect", "/models", "/set-model",
		"/status", "/clear", "/help", "/exit",
		"/worker-start", "/worker-stop", "/worker-status",
		"/headless", "/no-headless", "/run-task",
	}

	for _, expected := range expectedActions {
		if _, ok := actions[expected]; !ok {
			t.Errorf("missing action for %q", expected)
		}
	}
}

func TestActionClear(t *testing.T) {
	m := InitialModel()
	m = m.AddMessage("user", "test message")
	m = m.AddMessage("assistant", "response")

	m, _ = actionClear(m, nil)

	if len(m.messages) != 1 {
		t.Errorf("expected 1 message after clear, got %d", len(m.messages))
	}
	if m.messages[0].Role != "system" {
		t.Errorf("expected system message, got %q", m.messages[0].Role)
	}
}

func TestActionStatus(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		m := InitialModel()
		m = m.SetConnected(true, "openai", "gpt-4")

		m, _ = actionStatus(m, nil)

		if len(m.messages) != 1 {
			t.Error("expected status message to be added")
		}
	})

	t.Run("not connected", func(t *testing.T) {
		m := InitialModel()

		m, _ = actionStatus(m, nil)

		if len(m.messages) != 1 {
			t.Error("expected status message to be added")
		}
	})
}

func TestActionHelp(t *testing.T) {
	m := InitialModel()
	m, _ = actionHelp(m, nil)

	if m.viewMode != ViewModeHelp {
		t.Errorf("viewMode = %v; want %v", m.viewMode, ViewModeHelp)
	}
}

func TestActionExit(t *testing.T) {
	m := InitialModel()
	m, cmd := actionExit(m, nil)

	if cmd == nil {
		t.Error("expected quit command")
	}
	_ = m // m unchanged
}

func TestProcessSlashCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{"valid command", "/clear", false},
		{"valid command with args", "/status", false},
		{"invalid command", "/invalid", false}, // adds error message, not error return
		{"help command", "/help", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitialModel()
			m, _ = m.ProcessSlashCommand(tt.input)
			// Commands add system messages or change view mode
		})
	}
}
