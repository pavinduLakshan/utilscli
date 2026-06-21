package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// routeWithClaudeCode delegates only the ambiguous classification step to the
// user's authenticated Claude Code client. Its reply is data, not a command:
// uc still executes only an allow-listed local utility.
func routeWithClaudeCode(prompt string) (string, []string, error) {
	claude, err := exec.LookPath("claude")
	if err != nil {
		return "", nil, errors.New("couldn't identify a utility and Claude Code is unavailable; install and log in with 'claude', or run 'uc --help'")
	}

	systemPrompt := `You are a request router for a local command-line utility. Choose one supported command and extract its exact text input. Supported commands: b64-encode, b64-decode, b64url-encode, b64url-decode, url-encode, url-decode, html-encode, html-decode, json-pretty, json-minify, xml-pretty, xml-minify, jwt, saml, hash, timestamp, http. Do not use tools, inspect files, or explain anything. Return exactly one JSON object: {"command":"supported-command","input":"exact input"}.`

	// The fixed "User request:" prefix keeps a request beginning with a dash
	// from being mistaken for a Claude Code command-line flag.
	cmd := exec.Command(claude, "-p", "--output-format", "json", "--max-turns", "1", "--system-prompt", systemPrompt, "User request:\n"+prompt)
	output, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("Claude Code routing failed; ensure 'claude' is logged in: %w", err)
	}
	return parseClaudeResponse(output)
}

// parseClaudeResponse validates Claude Code's JSON envelope and extracted utility choice.
func parseClaudeResponse(output []byte) (string, []string, error) {
	var envelope struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		return "", nil, fmt.Errorf("Claude Code returned invalid JSON: %w", err)
	}
	var answer struct {
		Command string `json:"command"`
		Input   string `json:"input"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(envelope.Result)), &answer); err != nil {
		return "", nil, fmt.Errorf("Claude Code did not return a utility choice: %w", err)
	}
	command := canonicalCommand(answer.Command)
	if command == "" {
		return "", nil, errors.New("Claude Code selected an unsupported utility")
	}
	return command, []string{answer.Input}, nil
}
