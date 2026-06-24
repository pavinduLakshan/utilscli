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
	selected           int
	input              []rune
	cursor             int
	regexPattern       []rune
	regexPatternCursor int
	regexText          []rune
	regexTextCursor    int
	inputWidth         int
	focus              tuiFocus
	result             string
}

type tuiFocus int

const (
	focusInput tuiFocus = iota
	focusTools
	focusRegexPattern
	focusRegexText
)

type toolInputMode int

const (
	inputText toolInputMode = iota
	inputRegex
	inputNone
)

func selectedInputMode(state *tuiState) toolInputMode {
	switch tuiTools[state.selected].command {
	case "regex":
		return inputRegex
	case "uuid", "password":
		return inputNone
	default:
		return inputText
	}
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
		drawTUI(screen, &state)
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
	case tcell.KeyTAB:
		advanceFocus(state)
	case tcell.KeyUp:
		if state.focus == focusTools && state.selected > 0 {
			state.selected--
		} else if input, cursor := activeInput(state); input != nil {
			*cursor = moveCursorVertical(*input, *cursor, -1, state.inputWidth)
		}
	case tcell.KeyDown:
		if state.focus == focusTools && state.selected < len(tuiTools)-1 {
			state.selected++
		} else if input, cursor := activeInput(state); input != nil {
			*cursor = moveCursorVertical(*input, *cursor, 1, state.inputWidth)
		}
	case tcell.KeyLeft:
		if _, cursor := activeInput(state); cursor != nil && *cursor > 0 {
			*cursor--
		}
	case tcell.KeyRight:
		if input, cursor := activeInput(state); input != nil && *cursor < len(*input) {
			*cursor++
		}
	case tcell.KeyEnter:
		if input, cursor := activeInput(state); input != nil {
			insertInputRune(input, cursor, '\n')
		} else if selectedInputMode(state) == inputNone {
			runSelectedTool(state)
		} else {
			state.focus = firstInputFocus(selectedInputMode(state))
		}
	case tcell.KeyCtrlR:
		runSelectedTool(state)
	case tcell.KeyCtrlJ:
		if input, cursor := activeInput(state); input != nil {
			insertInputRune(input, cursor, '\n')
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if input, cursor := activeInput(state); input != nil && *cursor > 0 {
			*input = append((*input)[:*cursor-1], (*input)[*cursor:]...)
			*cursor--
		}
	case tcell.KeyRune:
		if state.focus == focusTools && selectedInputMode(state) != inputNone {
			state.focus = firstInputFocus(selectedInputMode(state))
		}
		if input, cursor := activeInput(state); input != nil {
			insertInputRune(input, cursor, event.Rune())
		}
	}
	return false
}

func advanceFocus(state *tuiState) {
	switch selectedInputMode(state) {
	case inputNone:
		state.focus = focusTools
	case inputRegex:
		switch state.focus {
		case focusTools:
			state.focus = focusRegexPattern
		case focusRegexPattern:
			state.focus = focusRegexText
		default:
			state.focus = focusTools
		}
	default:
		if state.focus == focusTools {
			state.focus = focusInput
		} else {
			state.focus = focusTools
		}
	}
}

func firstInputFocus(mode toolInputMode) tuiFocus {
	if mode == inputRegex {
		return focusRegexPattern
	}
	return focusInput
}

func activeInput(state *tuiState) (*[]rune, *int) {
	switch state.focus {
	case focusInput:
		return &state.input, &state.cursor
	case focusRegexPattern:
		return &state.regexPattern, &state.regexPatternCursor
	case focusRegexText:
		return &state.regexText, &state.regexTextCursor
	default:
		return nil, nil
	}
}

func insertInputRune(input *[]rune, cursor *int, char rune) {
	*input = append(*input, 0)
	copy((*input)[*cursor+1:], (*input)[*cursor:])
	(*input)[*cursor] = char
	*cursor++
}

func runSelectedTool(state *tuiState) {
	tool := tuiTools[state.selected]
	input := string(state.input)
	if selectedInputMode(state) == inputRegex {
		input = string(state.regexPattern) + "\n" + string(state.regexText)
	}
	result, err := execute(tool.command, input)
	if err != nil {
		state.result = "Error: " + err.Error()
		return
	}
	state.result = result
}

func drawTUI(screen tcell.Screen, state *tuiState) {
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
	state.inputWidth = rightWidth - 4

	boxStyle := tcell.StyleDefault.Foreground(tcell.ColorMediumPurple)
	titleStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorAqua)
	mutedStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	toolsTitle := "Tools"
	if state.focus == focusTools {
		toolsTitle += " (active)"
	}
	drawBox(screen, 0, 0, leftWidth, height-2, toolsTitle, boxStyle)
	drawBox(screen, rightX, 0, rightWidth, height-2, tuiTools[state.selected].command, boxStyle)

	drawTools(screen, *state, 1, 1, leftWidth-2, height-4)
	drawText(screen, rightX+2, 2, rightWidth-4, tuiTools[state.selected].description, mutedStyle)
	switch selectedInputMode(state) {
	case inputRegex:
		drawRegexView(screen, *state, rightX+2, rightWidth-4, height, titleStyle)
	case inputNone:
		drawNoInputView(screen, state.result, rightX+2, rightWidth-4, height, titleStyle, mutedStyle)
	default:
		drawTextInputView(screen, *state, rightX+2, rightWidth-4, height, titleStyle)
	}
	drawText(screen, 0, height-1, width, "Tab switches panes · arrows move the active pane · Enter adds a line · Ctrl+R runs · Esc exits", mutedStyle)
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

func drawTextInputView(screen tcell.Screen, state tuiState, x, width, screenHeight int, titleStyle tcell.Style) {
	inputHeight := max(3, screenHeight/3)
	title := "Input"
	if state.focus == focusInput {
		title += " (active)"
	}
	drawText(screen, x, 4, width, title, titleStyle)
	drawInput(screen, state.input, state.cursor, state.focus == focusInput, x, 5, width, inputHeight, "Type input here")
	drawOutput(screen, state.result, x, 6+inputHeight, width, screenHeight-(9+inputHeight), titleStyle)
}

func drawRegexView(screen tcell.Screen, state tuiState, x, width, screenHeight int, titleStyle tcell.Style) {
	fieldHeight := max(2, (screenHeight-12)/3)
	patternTitle := "Pattern"
	if state.focus == focusRegexPattern {
		patternTitle += " (active)"
	}
	textTitle := "Text to test"
	if state.focus == focusRegexText {
		textTitle += " (active)"
	}
	drawText(screen, x, 4, width, patternTitle, titleStyle)
	drawInput(screen, state.regexPattern, state.regexPatternCursor, state.focus == focusRegexPattern, x, 5, width, fieldHeight, "Regular expression")
	textY := 6 + fieldHeight
	drawText(screen, x, textY, width, textTitle, titleStyle)
	drawInput(screen, state.regexText, state.regexTextCursor, state.focus == focusRegexText, x, textY+1, width, fieldHeight, "Text to test")
	drawOutput(screen, state.result, x, textY+fieldHeight+2, width, screenHeight-(textY+fieldHeight+5), titleStyle)
}

func drawNoInputView(screen tcell.Screen, result string, x, width, screenHeight int, titleStyle, mutedStyle tcell.Style) {
	drawText(screen, x, 4, width, "No input required", titleStyle)
	drawText(screen, x, 5, width, "Press Enter or Ctrl+R to generate a result.", mutedStyle)
	drawOutput(screen, result, x, 7, width, screenHeight-10, titleStyle)
}

func drawOutput(screen tcell.Screen, result string, x, y, width, height int, titleStyle tcell.Style) {
	drawText(screen, x, y, width, "Output", titleStyle)
	if result == "" {
		result = "Output appears here after you run the tool."
	}
	drawLines(screen, x, y+1, width, height, result, tcell.StyleDefault)
}

func drawInput(screen tcell.Screen, input []rune, cursor int, active bool, x, y, width, height int, placeholder string) {
	lines := wrapTUI(string(input), width)
	cursorLine, cursorColumn := inputCursorPosition(input, cursor, width)
	if cursorLine >= len(lines) {
		lines = append(lines, "")
	}
	firstLine := max(0, cursorLine-height+1)
	if len(input) == 0 {
		drawText(screen, x, y, width, placeholder, tcell.StyleDefault.Foreground(tcell.ColorGray))
	}
	for row := 0; row < height; row++ {
		lineIndex := firstLine + row
		if lineIndex >= len(lines) {
			break
		}
		drawText(screen, x, y+row, width, lines[lineIndex], tcell.StyleDefault)
	}
	if !active || cursorLine < firstLine || cursorLine >= firstLine+height {
		return
	}
	line := []rune(lines[cursorLine])
	cursorChar := ' '
	if cursorColumn < len(line) {
		cursorChar = line[cursorColumn]
	}
	screen.SetContent(x+cursorColumn, y+cursorLine-firstLine, cursorChar, nil, tcell.StyleDefault.Reverse(true))
}

func inputCursorPosition(input []rune, cursor, width int) (int, int) {
	if width < 1 {
		width = 1
	}
	cursor = min(max(cursor, 0), len(input))
	line, column := 0, 0
	for _, char := range input[:cursor] {
		if char == '\n' {
			line, column = line+1, 0
			continue
		}
		column++
		if column == width {
			line, column = line+1, 0
		}
	}
	return line, column
}

func moveCursorVertical(input []rune, cursor, direction, width int) int {
	currentLine, currentColumn := inputCursorPosition(input, cursor, width)
	lastLine, _ := inputCursorPosition(input, len(input), width)
	targetLine := min(max(currentLine+direction, 0), lastLine)
	bestPosition, bestDistance := cursor, int(^uint(0)>>1)
	for position := 0; position <= len(input); position++ {
		line, column := inputCursorPosition(input, position, width)
		if line != targetLine {
			continue
		}
		distance := column - currentColumn
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestPosition, bestDistance = position, distance
		}
	}
	return bestPosition
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
