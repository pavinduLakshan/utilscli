package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// runInteractive accepts one natural-language request for a bare uc invocation.
func runInteractive(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	fmt.Fprintln(out, terminalStyle(out, "1;36", "What would you like to do?"))
	fmt.Fprint(out, terminalStyle(out, "2;36", "Enter a blank line when you are ready to submit.\n> "))
	prompt, err := readPrompt(scanner, out)
	if err != nil {
		return err
	}
	command, args, err := resolvePrompt(prompt)
	if err != nil {
		return err
	}
	return executeAndPrint(command, joinInput(args), out)
}

// readPrompt collects multiline input until a blank line or end of input.
func readPrompt(scanner *bufio.Scanner, out io.Writer) (string, error) {
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, line)
		fmt.Fprint(out, terminalStyle(out, "2;36", "> "))
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", errors.New("interactive input was cancelled")
	}
	return strings.Join(lines, "\n"), nil
}

// terminalStyle adds ANSI color only when writing directly to an interactive terminal.
func terminalStyle(out io.Writer, code, text string) string {
	file, ok := out.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return text
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// joinInput restores routed text arguments to the input shape expected by a utility.
func joinInput(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// resolvePrompt finds a local route first, then asks Claude Code only when necessary.
func resolvePrompt(prompt string) (string, []string, error) {
	command, args, err := routePrompt(prompt)
	if err != nil {
		return routeWithClaudeCode(prompt)
	}
	return command, args, nil
}

// executeAndPrint runs a command and appends its result to an output stream.
func executeAndPrint(command, input string, out io.Writer) error {
	result, err := execute(command, input)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, terminalStyle(out, "1;32", result))
	return nil
}
