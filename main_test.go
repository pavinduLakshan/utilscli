package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestPipedInput verifies a utility accepts text from standard input.
func TestPipedInput(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"b64-decode"}, strings.NewReader("aGVsbG8="), &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "hello\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestUtilityTransforms covers representative deterministic utility results.
func TestUtilityTransforms(t *testing.T) {
	tests := []struct{ command, input, want string }{
		{"url-encode", "a!b*c()'", "a%21b%2Ac%28%29%27"},
		{"html-encode", `<a&'">`, "&lt;a&amp;&#39;&quot;&gt;"},
		{"json-minify", "{ \"a\": 1 }", "{\"a\":1}"},
		{"http", "404", "404 Not Found"},
	}
	for _, tt := range tests {
		got, err := execute(tt.command, tt.input)
		if err != nil {
			t.Fatalf("%s: %v", tt.command, err)
		}
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.command, got, tt.want)
		}
	}
}

// TestPasswordAndUUIDDefaults ensures no-input generators use their defaults.
func TestPasswordAndUUIDDefaults(t *testing.T) {
	for _, command := range []string{"password", "uuid"} {
		var out bytes.Buffer
		if err := run([]string{command}, strings.NewReader(""), &out); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if strings.TrimSpace(out.String()) == "" {
			t.Errorf("%s generated no value", command)
		}
	}
}

// TestInteractiveMenu verifies the terminal UI includes its main panels.
func TestInteractiveMenu(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	drawTUI(screen, &tuiState{})
	r, _, _, _ := screen.GetContent(3, 0)
	if r != 'T' {
		t.Fatalf("tools panel was not rendered, got %q", r)
	}
}

func TestEditorCursorNavigation(t *testing.T) {
	state := tuiState{input: []rune("one\ntwo"), cursor: 1, inputWidth: 20}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if state.cursor != 2 {
		t.Fatalf("right arrow moved cursor to %d, want 2", state.cursor)
	}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if state.cursor != 6 {
		t.Fatalf("down arrow moved cursor to %d, want 6", state.cursor)
	}
}

func TestEditorEnterAddsNewline(t *testing.T) {
	state := tuiState{input: []rune("first"), cursor: 5}
	handleTUIKey(&state, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if got, want := string(state.input), "first\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRegexUsesSeparatePatternAndTextFields(t *testing.T) {
	state := tuiState{regexPattern: []rune("foo"), regexText: []rune("foo bar foo")}
	for index, tool := range tuiTools {
		if tool.command == "regex" {
			state.selected = index
			break
		}
	}
	runSelectedTool(&state)
	if !strings.Contains(state.result, "2 match(es)") {
		t.Fatalf("unexpected regex result: %q", state.result)
	}
}

func TestTUIToolsAreCommands(t *testing.T) {
	for _, tool := range tuiTools {
		if canonicalCommand(tool.command) == "" {
			t.Errorf("%q is not a supported command", tool.command)
		}
	}
}

func TestUnknownCommandDoesNotUsePromptRouting(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"base64 hello"}, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected an unknown command error")
	}
}
