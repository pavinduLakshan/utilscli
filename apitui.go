package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// apiMethods are the HTTP methods selectable in the API client's method field.
var apiMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// defaultRequestHeaders pre-populates the Request Headers tab with common header names
// so a new request just needs values filled in; blank ones are omitted when sent (see
// parseHeaderLines). Kept to 4 lines so they're all visible without scrolling.
const defaultRequestHeaders = "Content-Type: \nAccept: \nAuthorization: \nUser-Agent: "

// defaultRequestHeadersCursor is the initial cursor position, right after the first
// header's colon, so typing immediately fills in its value.
const defaultRequestHeadersCursor = len("Content-Type: ")

// apiFocusHint replaces the default tool-TUI hint on API client boxes, since this screen
// also supports Shift+Tab to move focus backward.
const apiFocusHint = " (active · Tab/Shift+Tab switches focus)"

type apiTUIFocus int

const (
	apiFocusHistorySearch apiTUIFocus = iota
	apiFocusHistoryList
	apiFocusMethod
	apiFocusURL
	apiFocusReqHeaders
	apiFocusReqBody
	apiFocusRespHeaders
	apiFocusRespBody
)

const apiFocusCount = apiFocusRespBody + 1

// apiTUIState holds the request history, a single in-progress request, and its last
// response for the API client screen.
type apiTUIState struct {
	history             []apiHistoryEntry
	historyFilter       []rune
	historyFilterCursor int
	historyFilterWidth  int
	historySelected     int

	methodIndex int

	url       []rune
	urlCursor int
	urlWidth  int

	// reqActiveTab tracks which of Request Headers/Body is shown in the merged Request
	// box; it follows focus (see setFocus) so whichever field you Tab into is visible.
	reqActiveTab int // 0 = Headers, 1 = Body

	reqHeaders      []rune
	reqHeaderCursor int
	reqHeadersWidth int

	reqBody       []rune
	reqBodyCursor int
	reqBodyWidth  int

	focus apiTUIFocus

	// response and responseErr are mutually exclusive: a send either populates response
	// (and derives respHeadersText/respBodyText from it) or sets responseErr.
	response      *apiResponse
	responseErr   string
	respActiveTab int // 0 = Headers, 1 = Body

	respHeadersText   string
	respHeadersOffset int

	respBodyText   string
	respBodyOffset int

	respWidth  int
	respHeight int

	status string
}

// activeField returns the editable text and cursor for the currently focused field, or
// (nil, nil) when the focus is on a field with no free text (history list, method, response).
func (state *apiTUIState) activeField() (*[]rune, *int) {
	switch state.focus {
	case apiFocusHistorySearch:
		return &state.historyFilter, &state.historyFilterCursor
	case apiFocusURL:
		return &state.url, &state.urlCursor
	case apiFocusReqHeaders:
		return &state.reqHeaders, &state.reqHeaderCursor
	case apiFocusReqBody:
		return &state.reqBody, &state.reqBodyCursor
	default:
		return nil, nil
	}
}

func (state *apiTUIState) filteredHistory() []apiHistoryEntry {
	return filterHistory(state.history, string(state.historyFilter))
}

// setFocus moves focus and, when landing on a Headers/Body field, brings that tab to the
// front of its box (Request or Response) — the same Tab-based mechanism for both sections.
func setFocus(state *apiTUIState, focus apiTUIFocus) {
	state.focus = focus
	switch focus {
	case apiFocusReqHeaders:
		state.reqActiveTab = 0
	case apiFocusReqBody:
		state.reqActiveTab = 1
	case apiFocusRespHeaders:
		state.respActiveTab = 0
	case apiFocusRespBody:
		state.respActiveTab = 1
	}
}

// newAPITUIState builds the initial screen state: request history loaded from disk, and
// the Request Headers tab pre-populated with common header names ready to fill in.
func newAPITUIState() *apiTUIState {
	return &apiTUIState{
		focus:           apiFocusURL,
		history:         loadAPIHistory(),
		reqHeaders:      []rune(defaultRequestHeaders),
		reqHeaderCursor: defaultRequestHeadersCursor,
	}
}

// runAPITUI opens a full-screen request builder for `uc api` run with no arguments in a terminal.
func runAPITUI(in io.Reader, out io.Writer) error {
	inFile, inputIsFile := in.(*os.File)
	outFile, outputIsFile := out.(*os.File)
	if !inputIsFile || !outputIsFile || !isTerminal(inFile) || !isTerminal(outFile) {
		return fmt.Errorf(`the API client UI requires a terminal; run 'uc api "<METHOD> <URL>"' instead`)
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create terminal UI: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("start terminal UI: %w", err)
	}
	defer screen.Fini()
	screen.EnableMouse()
	defer screen.DisableMouse()

	state := newAPITUIState()
	for {
		drawAPITUI(screen, state)
		if handleAPITUIEvent(screen, state, screen.PollEvent()) {
			return nil
		}
		for screen.HasPendingEvent() {
			if handleAPITUIEvent(screen, state, screen.PollEvent()) {
				return nil
			}
		}
	}
}

func handleAPITUIEvent(screen tcell.Screen, state *apiTUIState, event tcell.Event) bool {
	switch event := event.(type) {
	case *tcell.EventResize:
		screen.Sync()
	case *tcell.EventKey:
		switch event.Key() {
		case tcell.KeyCtrlY:
			copyAPIResponse(screen, state)
			return false
		case tcell.KeyCtrlC:
			copyFocusedContent(screen, state)
			return false
		}
		return handleAPITUIKey(state, event)
	case *tcell.EventMouse:
		switch event.Buttons() {
		case tcell.WheelUp:
			scrollActiveResponseTab(state, -3)
		case tcell.WheelDown:
			scrollActiveResponseTab(state, 3)
		}
	}
	return false
}

func handleAPITUIKey(state *apiTUIState, event *tcell.EventKey) bool {
	state.status = ""
	switch event.Key() {
	case tcell.KeyCtrlX, tcell.KeyEscape:
		return true
	case tcell.KeyTAB:
		setFocus(state, (state.focus+1)%apiFocusCount)
	case tcell.KeyBacktab:
		setFocus(state, (state.focus-1+apiFocusCount)%apiFocusCount)
	case tcell.KeyCtrlR:
		sendAPITUIRequest(state)
	case tcell.KeyUp:
		switch state.focus {
		case apiFocusHistoryList:
			if state.historySelected > 0 {
				state.historySelected--
			}
		case apiFocusMethod:
			cycleMethod(state, -1)
		case apiFocusReqHeaders:
			state.reqHeaderCursor = moveCursorVertical(state.reqHeaders, state.reqHeaderCursor, -1, max(1, state.reqHeadersWidth))
		case apiFocusReqBody:
			state.reqBodyCursor = moveCursorVertical(state.reqBody, state.reqBodyCursor, -1, max(1, state.reqBodyWidth))
		case apiFocusRespHeaders, apiFocusRespBody:
			scrollActiveResponseTab(state, -1)
		}
	case tcell.KeyDown:
		switch state.focus {
		case apiFocusHistoryList:
			if state.historySelected < len(state.filteredHistory())-1 {
				state.historySelected++
			}
		case apiFocusMethod:
			cycleMethod(state, 1)
		case apiFocusReqHeaders:
			state.reqHeaderCursor = moveCursorVertical(state.reqHeaders, state.reqHeaderCursor, 1, max(1, state.reqHeadersWidth))
		case apiFocusReqBody:
			state.reqBodyCursor = moveCursorVertical(state.reqBody, state.reqBodyCursor, 1, max(1, state.reqBodyWidth))
		case apiFocusRespHeaders, apiFocusRespBody:
			scrollActiveResponseTab(state, 1)
		}
	case tcell.KeyLeft:
		switch state.focus {
		case apiFocusMethod:
			cycleMethod(state, -1)
		default:
			if _, cursor := state.activeField(); cursor != nil && *cursor > 0 {
				*cursor--
			}
		}
	case tcell.KeyRight:
		switch state.focus {
		case apiFocusMethod:
			cycleMethod(state, 1)
		default:
			if input, cursor := state.activeField(); input != nil && *cursor < len(*input) {
				*cursor++
			}
		}
	case tcell.KeyPgUp:
		setFocus(state, responsePagingFocus(state))
		scrollActiveResponseTab(state, -max(1, state.respHeight))
	case tcell.KeyPgDn:
		setFocus(state, responsePagingFocus(state))
		scrollActiveResponseTab(state, max(1, state.respHeight))
	case tcell.KeyHome:
		if state.focus == apiFocusRespHeaders || state.focus == apiFocusRespBody {
			setActiveResponseOffset(state, 0)
		}
	case tcell.KeyEnd:
		if state.focus == apiFocusRespHeaders || state.focus == apiFocusRespBody {
			setActiveResponseOffset(state, maxTextOffset(activeResponseText(state), state.respWidth, state.respHeight))
		}
	case tcell.KeyEnter:
		switch state.focus {
		case apiFocusReqHeaders, apiFocusReqBody:
			if input, cursor := state.activeField(); input != nil {
				insertInputRune(input, cursor, '\n')
			}
		case apiFocusHistoryList:
			loadHistoryEntry(state)
		default:
			sendAPITUIRequest(state)
		}
	case tcell.KeyCtrlJ:
		if input, cursor := state.activeField(); input != nil {
			insertInputRune(input, cursor, '\n')
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if input, cursor := state.activeField(); input != nil && *cursor > 0 {
			*input = append((*input)[:*cursor-1], (*input)[*cursor:]...)
			*cursor--
		}
	case tcell.KeyRune:
		if state.focus == apiFocusHistoryList && event.Rune() == 'p' {
			togglePinSelected(state)
		} else if input, cursor := state.activeField(); input != nil {
			insertInputRune(input, cursor, event.Rune())
		}
	}
	return false
}

func cycleMethod(state *apiTUIState, delta int) {
	state.methodIndex = (state.methodIndex + delta + len(apiMethods)) % len(apiMethods)
}

// responsePagingFocus picks which response tab Page Up/Down acts on: whichever is
// already focused, or the Body tab by default when focus is elsewhere.
func responsePagingFocus(state *apiTUIState) apiTUIFocus {
	if state.focus == apiFocusRespHeaders {
		return apiFocusRespHeaders
	}
	return apiFocusRespBody
}

// loadHistoryEntry copies the selected history entry into the request fields and moves
// focus to the URL field, ready to review, tweak, or resend.
func loadHistoryEntry(state *apiTUIState) {
	filtered := state.filteredHistory()
	if state.historySelected < 0 || state.historySelected >= len(filtered) {
		return
	}
	entry := filtered[state.historySelected]
	state.methodIndex = 0
	for i, m := range apiMethods {
		if m == entry.Method {
			state.methodIndex = i
			break
		}
	}
	state.url = []rune(entry.URL)
	state.urlCursor = len(state.url)

	var headerLines []string
	for _, h := range entry.Headers {
		headerLines = append(headerLines, h.Key+": "+h.Value)
	}
	state.reqHeaders = []rune(strings.Join(headerLines, "\n"))
	state.reqHeaderCursor = len(state.reqHeaders)

	state.reqBody = []rune(entry.Body)
	state.reqBodyCursor = len(state.reqBody)

	setFocus(state, apiFocusURL)
}

func togglePinSelected(state *apiTUIState) {
	filtered := state.filteredHistory()
	if state.historySelected < 0 || state.historySelected >= len(filtered) {
		return
	}
	entry := filtered[state.historySelected]
	state.history = togglePin(state.history, entry.Method, entry.URL)
	saveAPIHistory(state.history)
}

// sendAPITUIRequest builds an apiRequest from the current fields and executes it, splitting
// the result into separate response-headers and response-body text for the two tabs, and
// records the request in history on success.
func sendAPITUIRequest(state *apiTUIState) {
	state.respHeadersOffset = 0
	state.respBodyOffset = 0
	state.status = ""

	url := strings.TrimSpace(string(state.url))
	if url == "" {
		state.response = nil
		state.responseErr = "URL is required"
		return
	}
	headers, err := parseHeaderLines(string(state.reqHeaders))
	if err != nil {
		state.response = nil
		state.responseErr = err.Error()
		return
	}
	req := &apiRequest{
		Method:  apiMethods[state.methodIndex],
		URL:     url,
		Headers: headers,
		Body:    string(state.reqBody),
	}
	resp, err := executeAPIRequest(req)
	if err != nil {
		state.response = nil
		state.responseErr = err.Error()
		return
	}

	state.responseErr = ""
	state.response = resp
	var headerLines []string
	for _, h := range resp.Headers {
		headerLines = append(headerLines, h.Key+": "+h.Value)
	}
	state.respHeadersText = strings.Join(headerLines, "\n")
	state.respBodyText = resp.Body

	state.history = addOrUpdateHistory(state.history, req.Method, req.URL, req.Headers, req.Body)
	saveAPIHistory(state.history)
}

func activeResponseText(state *apiTUIState) string {
	if state.respActiveTab == 0 {
		return state.respHeadersText
	}
	return state.respBodyText
}

func setActiveResponseOffset(state *apiTUIState, offset int) {
	if state.respActiveTab == 0 {
		state.respHeadersOffset = offset
	} else {
		state.respBodyOffset = offset
	}
}

func scrollActiveResponseTab(state *apiTUIState, delta int) {
	offset := clampOffset(currentResponseOffset(state)+delta, state.respWidth, state.respHeight, activeResponseText(state))
	setActiveResponseOffset(state, offset)
}

func currentResponseOffset(state *apiTUIState) int {
	if state.respActiveTab == 0 {
		return state.respHeadersOffset
	}
	return state.respBodyOffset
}

func clampOffset(offset, width, height int, text string) int {
	return min(max(offset, 0), maxTextOffset(text, width, height))
}

func maxTextOffset(text string, width, height int) int {
	if width < 1 || height < 1 || text == "" {
		return 0
	}
	return max(0, len(wrapTUI(text, width))-height)
}

// copyAPIResponse copies the full formatted response (status, headers, and body together),
// matching what `uc api "<spec>"` would print — bound to Ctrl+Y regardless of which
// response tab is in view.
func copyAPIResponse(screen tcell.Screen, state *apiTUIState) {
	if state.response == nil {
		state.status = "No response to copy"
		return
	}
	text := formatAPIResponse(state.response)
	screen.SetClipboard([]byte(text))
	if err := writeSystemClipboard(text); err != nil {
		state.status = "Copy sent to terminal; clipboard access may be blocked"
		return
	}
	state.status = "Response copied"
}

// copyFocusedContent copies whatever text is in the currently focused field or tab —
// bound to Ctrl+C, so copying never requires switching focus first.
func copyFocusedContent(screen tcell.Screen, state *apiTUIState) {
	var text string
	switch state.focus {
	case apiFocusHistorySearch:
		text = string(state.historyFilter)
	case apiFocusHistoryList:
		if filtered := state.filteredHistory(); state.historySelected >= 0 && state.historySelected < len(filtered) {
			text = filtered[state.historySelected].URL
		}
	case apiFocusMethod:
		text = apiMethods[state.methodIndex]
	case apiFocusURL:
		text = string(state.url)
	case apiFocusReqHeaders:
		text = string(state.reqHeaders)
	case apiFocusReqBody:
		text = string(state.reqBody)
	case apiFocusRespHeaders, apiFocusRespBody:
		if state.responseErr != "" {
			text = state.responseErr
		} else {
			text = activeResponseText(state)
		}
	}
	if text == "" {
		state.status = "Nothing to copy"
		return
	}
	screen.SetClipboard([]byte(text))
	if err := writeSystemClipboard(text); err != nil {
		state.status = "Copy sent to terminal; clipboard access may be blocked"
		return
	}
	state.status = "Copied"
}

func drawAPITUI(screen tcell.Screen, state *apiTUIState) {
	width, height := screen.Size()
	screen.Clear()
	if width < 90 || height < 24 {
		drawText(screen, 0, 0, width, "Please enlarge the terminal to at least 90×24.", tcell.StyleDefault.Foreground(tcell.ColorYellow))
		screen.Show()
		return
	}

	requestStyle := tcell.StyleDefault.Foreground(tcell.ColorMediumPurple)
	responseStyle := tcell.StyleDefault.Foreground(tcell.ColorSeaGreen)
	historyStyle := tcell.StyleDefault.Foreground(tcell.ColorSteelBlue)
	activeStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorWhite).Background(tcell.ColorDarkCyan)
	mutedStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)

	sidebarWidth := max(26, width/4)
	mainX := sidebarWidth + 1
	mainWidth := width - mainX

	drawHistorySidebar(screen, state, sidebarWidth, height-1, historyStyle, activeStyle, mutedStyle)

	available := (height - 1)
	requestBoxHeight := max(13, available/2)
	responseBoxHeight := max(9, available-requestBoxHeight)

	drawRequestBox(screen, state, mainX, 0, mainWidth, requestBoxHeight, requestStyle, activeStyle)
	drawResponseBox(screen, state, mainX, requestBoxHeight, mainWidth, responseBoxHeight, responseStyle, activeStyle)

	drawFooter(screen, state, width, height-1, mutedStyle)
	screen.Show()
}

// drawFooter shows the keybinding hint, replaced momentarily by a copy confirmation or
// warning after Ctrl+C/Ctrl+Y — status can originate from any focused field, so it's
// shown here rather than inside a specific box.
func drawFooter(screen tcell.Screen, state *apiTUIState, width, y int, mutedStyle tcell.Style) {
	if state.status == "" {
		drawText(screen, 0, y, width, "Tab/Shift+Tab fields+tabs · Enter loads/sends · p pins · ←→ method · Ctrl+R send · Ctrl+C copy focused · Ctrl+X exit", mutedStyle)
		return
	}
	statusStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorYellow)
	prefix := "! "
	if strings.HasSuffix(state.status, "opied") {
		statusStyle = tcell.StyleDefault.Bold(true).Foreground(tcell.ColorGreen)
		prefix = "✓ "
	}
	drawText(screen, 0, y, width, prefix+state.status, statusStyle)
}

func drawHistorySidebar(screen tcell.Screen, state *apiTUIState, width, height int, boxStyle, activeStyle, mutedStyle tcell.Style) {
	drawBoxWithHint(screen, 0, 0, width, 3, "Search", state.focus == apiFocusHistorySearch, boxStyle, activeStyle, apiFocusHint)
	state.historyFilterWidth = max(1, width-4)
	drawInput(screen, state.historyFilter, state.historyFilterCursor, state.focus == apiFocusHistorySearch, 2, 1, width-4, 1, "Filter by method/URL")

	listY := 3
	listHeight := height - listY
	drawBoxWithHint(screen, 0, listY, width, listHeight, "History", state.focus == apiFocusHistoryList, boxStyle, activeStyle, apiFocusHint)

	filtered := state.filteredHistory()
	if len(filtered) == 0 {
		state.historySelected = 0
	} else if state.historySelected >= len(filtered) {
		state.historySelected = len(filtered) - 1
	} else if state.historySelected < 0 {
		state.historySelected = 0
	}

	contentHeight := max(0, listHeight-2)
	if len(filtered) == 0 {
		drawText(screen, 2, listY+1, width-4, "No requests yet.", mutedStyle)
		return
	}
	offset := 0
	if state.historySelected >= contentHeight {
		offset = state.historySelected - contentHeight + 1
	}
	for row := 0; row < contentHeight; row++ {
		index := row + offset
		if index >= len(filtered) {
			break
		}
		entry := filtered[index]
		style := tcell.StyleDefault
		marker := "  "
		if entry.Pinned {
			marker = "★ "
		}
		if index == state.historySelected {
			style = style.Foreground(tcell.ColorBlack).Background(tcell.ColorAqua).Bold(true)
		}
		line := fmt.Sprintf("%s%-6s %s", marker, entry.Method, entry.URL)
		drawText(screen, 2, listY+1+row, width-4, line, style)
	}
}

// drawRequestBox renders Method, URL, and the Request Headers/Body tabs together under
// one "Request" section, mirroring the Response box's layout below it.
func drawRequestBox(screen tcell.Screen, state *apiTUIState, x, y, width, height int, boxStyle, activeStyle tcell.Style) {
	active := state.focus == apiFocusMethod || state.focus == apiFocusURL || state.focus == apiFocusReqHeaders || state.focus == apiFocusReqBody
	drawBoxWithHint(screen, x, y, width, height, "Request", active, boxStyle, activeStyle, apiFocusHint)

	// Method + URL get their own boxed bar, as prominent as the History Search field,
	// instead of sitting as a plain line inside the outer Request box.
	urlBarActive := state.focus == apiFocusMethod || state.focus == apiFocusURL
	urlBarStyle := boxStyle
	if urlBarActive {
		urlBarStyle = tcell.StyleDefault.Bold(true).Foreground(tcell.ColorAqua)
	}
	drawBox(screen, x+1, y+1, width-2, 3, "", false, urlBarStyle, activeStyle)

	method := apiMethods[state.methodIndex]
	methodStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorAqua)
	if state.focus == apiFocusMethod {
		methodStyle = tcell.StyleDefault.Bold(true).Foreground(tcell.ColorBlack).Background(tcell.ColorAqua)
	}
	drawText(screen, x+3, y+2, 9, fmt.Sprintf("%-7s", method), methodStyle)

	urlX := x + 14
	state.urlWidth = max(1, width-14-3)
	drawInput(screen, state.url, state.urlCursor, state.focus == apiFocusURL, urlX, y+2, state.urlWidth, 1, "https://api.example.com/path")

	// A blank row separates the URL bar from the Headers/Body tabs below it.
	tabY := y + 5
	tabStyle := func(active bool) tcell.Style {
		if active {
			return tcell.StyleDefault.Bold(true).Foreground(tcell.ColorBlack).Background(tcell.ColorAqua)
		}
		return tcell.StyleDefault.Foreground(tcell.ColorGray)
	}
	drawText(screen, x+2, tabY, 10, " Headers ", tabStyle(state.reqActiveTab == 0))
	drawText(screen, x+13, tabY, 7, " Body ", tabStyle(state.reqActiveTab == 1))

	contentY := tabY + 1
	contentHeight := max(0, height-(contentY-y)-1)
	contentWidth := max(1, width-4)
	if state.reqActiveTab == 0 {
		state.reqHeadersWidth = contentWidth
		drawInput(screen, state.reqHeaders, state.reqHeaderCursor, state.focus == apiFocusReqHeaders, x+2, contentY, contentWidth, contentHeight, "Name: value (one per line)")
	} else {
		state.reqBodyWidth = contentWidth
		drawInput(screen, state.reqBody, state.reqBodyCursor, state.focus == apiFocusReqBody, x+2, contentY, contentWidth, contentHeight, "Request body")
	}
}

func drawResponseBox(screen tcell.Screen, state *apiTUIState, x, y, width, height int, boxStyle, activeStyle tcell.Style) {
	title := "Response"
	if state.response != nil {
		title = fmt.Sprintf("Response — %s (%s, %d bytes)", state.response.Status, state.response.Elapsed, state.response.Size)
	}
	active := state.focus == apiFocusRespHeaders || state.focus == apiFocusRespBody
	drawBoxWithHint(screen, x, y, width, height, title, active, boxStyle, activeStyle, apiFocusHint)

	tabStyle := func(active bool) tcell.Style {
		if active {
			return tcell.StyleDefault.Bold(true).Foreground(tcell.ColorBlack).Background(tcell.ColorAqua)
		}
		return tcell.StyleDefault.Foreground(tcell.ColorGray)
	}
	drawText(screen, x+2, y+1, 10, " Headers ", tabStyle(state.respActiveTab == 0))
	drawText(screen, x+13, y+1, 7, " Body ", tabStyle(state.respActiveTab == 1))

	contentY := y + 2
	contentHeight := max(0, height-3)

	state.respWidth = max(1, width-4)
	state.respHeight = contentHeight
	state.respHeadersOffset = clampOffset(state.respHeadersOffset, state.respWidth, state.respHeight, state.respHeadersText)
	state.respBodyOffset = clampOffset(state.respBodyOffset, state.respWidth, state.respHeight, state.respBodyText)

	var content string
	var offset int
	switch {
	case state.responseErr != "":
		content = "Error: " + state.responseErr
	case state.respActiveTab == 0:
		content = state.respHeadersText
		offset = state.respHeadersOffset
		if content == "" {
			content = "Send a request (Ctrl+R) to see response headers here."
		}
	default:
		content = state.respBodyText
		offset = state.respBodyOffset
		if content == "" {
			content = "Send a request (Ctrl+R) to see the response body here."
		}
	}
	drawLines(screen, x+2, contentY, state.respWidth, contentHeight, offset, content, tcell.StyleDefault)
}
