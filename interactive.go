package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type toolDefinition struct {
	command     string
	description string
}

var tuiTools = []toolDefinition{
	{"b64-encode", "Encode text as Base64"},
	{"b64-decode", "Decode Base64 text"},
	{"b64url-encode", "Encode URL-safe Base64"},
	{"b64url-decode", "Decode URL-safe Base64"},
	{"url-encode", "Percent-encode a URL component"},
	{"url-decode", "Decode a URL component"},
	{"html-encode", "Escape HTML special characters"},
	{"html-decode", "Decode HTML entities"},
	{"json-pretty", "Validate and format JSON"},
	{"json-minify", "Validate and minify JSON"},
	{"xml-pretty", "Validate and format XML"},
	{"xml-minify", "Validate and minify XML"},
	{"jwt", "Decode a JWT"},
	{"saml", "Decode a SAML message"},
	{"hash", "Generate common hashes"},
	{"uuid", "Generate a UUID"},
	{"password", "Generate a password"},
	{"timestamp", "Convert a timestamp or date"},
	{"regex", "Find regular-expression matches"},
	{"diff", "Compare text separated by ---"},
	{"http", "Look up an HTTP status"},
	{"cors", "Generate CORS headers from flags"},
}

type tuiState struct {
	selected int
	input    []rune
	result   string
}

// runInteractive opens a full-screen terminal UI for a bare uc invocation.
func runInteractive(in io.Reader, out io.Writer) error {
	inFile, inputIsFile := in.(*os.File)
	outFile, outputIsFile := out.(*os.File)
	if !inputIsFile || !outputIsFile || !isTerminal(inFile) || !isTerminal(outFile) {
		return fmt.Errorf("the interactive UI requires a terminal; run uc from a terminal window or use 'uc --help'")
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create terminal UI: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("start terminal UI: %w", err)
	}
	defer screen.Fini()

	state := tuiState{}
	for {
		drawTUI(screen, state)
		switch event := screen.PollEvent().(type) {
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventKey:
			if handleTUIKey(&state, event) {
				return nil
			}
		}
	}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// handleTUIKey updates the UI and reports whether it should exit.
func handleTUIKey(state *tuiState, event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyCtrlC, tcell.KeyEscape:
		return true
	case tcell.KeyUp:
		if state.selected > 0 {
			state.selected--
		}
	case tcell.KeyDown:
		if state.selected < len(tuiTools)-1 {
			state.selected++
		}
	case tcell.KeyEnter:
		runSelectedTool(state)
	case tcell.KeyCtrlJ:
		state.input = append(state.input, '\n')
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(state.input) > 0 {
			state.input = state.input[:len(state.input)-1]
		}
	case tcell.KeyRune:
		state.input = append(state.input, event.Rune())
	}
	return false
}

func runSelectedTool(state *tuiState) {
	tool := tuiTools[state.selected]
	result, err := execute(tool.command, string(state.input))
	if err != nil {
		state.result = "Error: " + err.Error()
		return
	}
	state.result = result
}

func drawTUI(screen tcell.Screen, state tuiState) {
	width, height := screen.Size()
	screen.Clear()
	if width < 60 || height < 14 {
		drawText(screen, 0, 0, width, "Please enlarge the terminal to at least 60×14.", tcell.StyleDefault.Foreground(tcell.ColorYellow))
		screen.Show()
		return
	}

	leftWidth := width / 3
	if leftWidth < 25 {
		leftWidth = 25
	}
	rightX := leftWidth + 1
	rightWidth := width - rightX

	boxStyle := tcell.StyleDefault.Foreground(tcell.ColorMediumPurple)
	titleStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorAqua)
	mutedStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	drawBox(screen, 0, 0, leftWidth, height-2, "Tools", boxStyle)
	drawBox(screen, rightX, 0, rightWidth, height-2, tuiTools[state.selected].command, boxStyle)

	drawTools(screen, state, 1, 1, leftWidth-2, height-4)
	drawText(screen, rightX+2, 2, rightWidth-4, tuiTools[state.selected].description, mutedStyle)
	drawText(screen, rightX+2, 4, rightWidth-4, "Input", titleStyle)
	drawLines(screen, rightX+2, 5, rightWidth-4, max(3, height/3), string(state.input), tcell.StyleDefault)
	drawText(screen, rightX+2, 6+max(3, height/3), rightWidth-4, "Output", titleStyle)
	output := state.result
	if output == "" {
		output = "Output appears here after you press Enter."
	}
	drawLines(screen, rightX+2, 7+max(3, height/3), rightWidth-4, height-(9+max(3, height/3)), output, tcell.StyleDefault)
	drawText(screen, 0, height-1, width, "Type in the editor · ↑/↓ choose a tool · Enter runs · Ctrl+J adds a line · Esc exits", mutedStyle)
	screen.Show()
}

func drawTools(screen tcell.Screen, state tuiState, x, y, width, height int) {
	offset := 0
	if state.selected >= height {
		offset = state.selected - height + 1
	}
	for row := 0; row < height; row++ {
		index := row + offset
		if index >= len(tuiTools) {
			return
		}
		style := tcell.StyleDefault
		prefix := "  "
		if index == state.selected {
			style = style.Foreground(tcell.ColorBlack).Background(tcell.ColorAqua).Bold(true)
			prefix = "› "
		}
		drawText(screen, x, y+row, width, prefix+tuiTools[index].command, style)
	}
}

func drawBox(screen tcell.Screen, x, y, width, height int, title string, style tcell.Style) {
	if width < 2 || height < 2 {
		return
	}
	for column := x; column < x+width; column++ {
		screen.SetContent(column, y, '─', nil, style)
		screen.SetContent(column, y+height-1, '─', nil, style)
	}
	for row := y; row < y+height; row++ {
		screen.SetContent(x, row, '│', nil, style)
		screen.SetContent(x+width-1, row, '│', nil, style)
	}
	screen.SetContent(x, y, '┌', nil, style)
	screen.SetContent(x+width-1, y, '┐', nil, style)
	screen.SetContent(x, y+height-1, '└', nil, style)
	screen.SetContent(x+width-1, y+height-1, '┘', nil, style)
	drawText(screen, x+2, y, width-4, " "+title+" ", style.Bold(true))
}

func drawLines(screen tcell.Screen, x, y, width, height int, value string, style tcell.Style) {
	lines := wrapTUI(value, width)
	if len(lines) == 0 {
		drawText(screen, x, y, width, "Type input here", style.Foreground(tcell.ColorGray))
		return
	}
	for row, line := range lines {
		if row >= height {
			return
		}
		drawText(screen, x, y+row, width, line, style)
	}
}

func drawText(screen tcell.Screen, x, y, width int, value string, style tcell.Style) {
	if width <= 0 {
		return
	}
	for index, char := range []rune(value) {
		if index >= width {
			return
		}
		screen.SetContent(x+index, y, char, nil, style)
	}
}

func wrapTUI(value string, width int) []string {
	if value == "" || width < 1 {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		runes := []rune(line)
		if len(runes) == 0 {
			lines = append(lines, "")
		}
		for len(runes) > 0 {
			end := min(width, len(runes))
			lines = append(lines, string(runes[:end]))
			runes = runes[end:]
		}
	}
	return lines
}
