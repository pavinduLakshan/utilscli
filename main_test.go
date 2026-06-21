package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestNaturalLanguageBase64 verifies deterministic natural-language routing.
func TestNaturalLanguageBase64(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"base64 osidosodi"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "b3NpZG9zb2Rp\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

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

// TestParseClaudeResponse ensures an allow-listed Claude result is decoded safely.
func TestParseClaudeResponse(t *testing.T) {
	output := []byte(`{"result":"{\"command\":\"b64-encode\",\"input\":\"osidosodi\"}"}`)
	command, args, err := parseClaudeResponse(output)
	if err != nil {
		t.Fatal(err)
	}
	if command != "b64-encode" || len(args) != 1 || args[0] != "osidosodi" {
		t.Fatalf("unexpected routing result: %q %#v", command, args)
	}
}

// TestInteractiveMenu verifies natural-language execution from a bare uc invocation.
func TestInteractiveMenu(t *testing.T) {
	var out bytes.Buffer
	input := strings.NewReader("base64 osidosodi\n")
	if err := run(nil, input, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "What would you like to do?") || !strings.Contains(out.String(), "b3NpZG9zb2Rp") {
		t.Fatalf("interactive output did not include the prompt and encoded result: %q", out.String())
	}
}
