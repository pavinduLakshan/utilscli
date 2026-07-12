package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func keyEvent(key tcell.Key, r rune) *tcell.EventKey {
	return tcell.NewEventKey(key, r, tcell.ModNone)
}

func TestAPITUITabCyclesAllFields(t *testing.T) {
	state := &apiTUIState{focus: apiFocusHistorySearch}
	seen := []apiTUIFocus{state.focus}
	for i := 0; i < int(apiFocusCount); i++ {
		handleAPITUIKey(state, keyEvent(tcell.KeyTAB, 0))
		seen = append(seen, state.focus)
	}
	if seen[len(seen)-1] != apiFocusHistorySearch {
		t.Errorf("tabbing all the way around should return to history search, got %v", seen)
	}
}

func TestAPITUIShiftTabCyclesBackward(t *testing.T) {
	state := &apiTUIState{focus: apiFocusURL}
	handleAPITUIKey(state, keyEvent(tcell.KeyBacktab, 0))
	if state.focus != apiFocusMethod {
		t.Fatalf("Shift+Tab from URL should go back to Method, got %v", state.focus)
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyBacktab, 0))
	if state.focus != apiFocusHistoryList {
		t.Fatalf("Shift+Tab from Method should go back to History List, got %v", state.focus)
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyTAB, 0))
	if state.focus != apiFocusMethod {
		t.Errorf("Tab should undo the Shift+Tab it follows, got %v", state.focus)
	}
}

func TestNewAPITUIStatePrePopulatesCommonHeaders(t *testing.T) {
	withTempAPIHistoryDir(t)
	state := newAPITUIState()
	if !strings.Contains(string(state.reqHeaders), "Content-Type:") {
		t.Errorf("expected common headers to be pre-populated, got %q", string(state.reqHeaders))
	}
	if !strings.Contains(string(state.reqHeaders), "Authorization:") {
		t.Errorf("expected common headers to be pre-populated, got %q", string(state.reqHeaders))
	}
	if state.reqHeaderCursor != defaultRequestHeadersCursor {
		t.Errorf("expected the cursor placed after the first header's colon, got %d", state.reqHeaderCursor)
	}
}

func TestDefaultHeadersAllVisibleWithoutScrollingAtMinimumSize(t *testing.T) {
	withTempAPIHistoryDir(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(90, 24) // the smallest size the UI supports

	state := newAPITUIState()
	drawAPITUI(screen, state)

	contents, _, _ := screen.GetContents()
	var screenText strings.Builder
	for _, c := range contents {
		screenText.WriteRune(c.Runes[0])
	}
	text := screenText.String()
	for _, header := range []string{"Content-Type:", "Accept:", "Authorization:", "User-Agent:"} {
		if !strings.Contains(text, header) {
			t.Errorf("expected %q visible without scrolling at the minimum terminal size, screen:\n%s", header, text)
		}
	}
}

func TestAPITUIMethodCyclesWithArrowsAndUpDown(t *testing.T) {
	state := &apiTUIState{focus: apiFocusMethod}
	if apiMethods[state.methodIndex] != "GET" {
		t.Fatalf("expected default method GET, got %s", apiMethods[state.methodIndex])
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyDown, 0))
	if apiMethods[state.methodIndex] != "POST" {
		t.Errorf("Down should advance to POST, got %s", apiMethods[state.methodIndex])
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyUp, 0))
	if apiMethods[state.methodIndex] != "GET" {
		t.Errorf("Up should return to GET, got %s", apiMethods[state.methodIndex])
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyLeft, 0))
	if apiMethods[state.methodIndex] != apiMethods[len(apiMethods)-1] {
		t.Errorf("Left should wrap to the last method, got %s", apiMethods[state.methodIndex])
	}
}

func TestAPITUITypingGoesToFocusedField(t *testing.T) {
	state := &apiTUIState{focus: apiFocusURL}
	for _, r := range "https://example.com" {
		handleAPITUIKey(state, keyEvent(tcell.KeyRune, r))
	}
	if got := string(state.url); got != "https://example.com" {
		t.Errorf("url: got %q", got)
	}

	state.focus = apiFocusReqHeaders
	for _, r := range "X-Test: 1" {
		handleAPITUIKey(state, keyEvent(tcell.KeyRune, r))
	}
	if got := string(state.reqHeaders); got != "X-Test: 1" {
		t.Errorf("headers: got %q", got)
	}

	state.focus = apiFocusReqBody
	for _, r := range `{"a":1}` {
		handleAPITUIKey(state, keyEvent(tcell.KeyRune, r))
	}
	if got := string(state.reqBody); got != `{"a":1}` {
		t.Errorf("body: got %q", got)
	}

	state.focus = apiFocusHistorySearch
	for _, r := range "login" {
		handleAPITUIKey(state, keyEvent(tcell.KeyRune, r))
	}
	if got := string(state.historyFilter); got != "login" {
		t.Errorf("history filter: got %q", got)
	}
}

func TestAPITUIEnterInsertsNewlineInRequestHeadersAndBodyOnly(t *testing.T) {
	state := &apiTUIState{focus: apiFocusReqHeaders}
	handleAPITUIKey(state, keyEvent(tcell.KeyEnter, 0))
	if string(state.reqHeaders) != "\n" {
		t.Errorf("expected a newline inserted into request headers, got %q", string(state.reqHeaders))
	}

	state = &apiTUIState{focus: apiFocusReqBody}
	handleAPITUIKey(state, keyEvent(tcell.KeyEnter, 0))
	if string(state.reqBody) != "\n" {
		t.Errorf("expected a newline inserted into request body, got %q", string(state.reqBody))
	}
}

func TestAPITUIEnterOnURLSendsRequest(t *testing.T) {
	withTempAPIHistoryDir(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	state := &apiTUIState{focus: apiFocusURL, url: []rune(server.URL)}
	handleAPITUIKey(state, keyEvent(tcell.KeyEnter, 0))
	if state.response == nil || !strings.Contains(state.response.Status, "200") {
		t.Fatalf("expected Enter on the URL field to send the request, got %+v", state.response)
	}
}

func TestSendAPITUIRequestRequiresURL(t *testing.T) {
	state := &apiTUIState{}
	sendAPITUIRequest(state)
	if state.response != nil {
		t.Errorf("expected no response on validation failure, got %+v", state.response)
	}
	if !strings.Contains(state.responseErr, "URL is required") {
		t.Errorf("expected a URL-required error, got %q", state.responseErr)
	}
}

func TestSendAPITUIRequestUsesMethodHeadersAndBodyAndRecordsHistory(t *testing.T) {
	withTempAPIHistoryDir(t)
	var gotMethod, gotHeader, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Test")
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		gotBody = string(body)
		w.Header().Set("X-Resp", "yes")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	state := &apiTUIState{
		methodIndex: 1, // POST
		url:         []rune(server.URL),
		reqHeaders:  []rune("X-Test: hello"),
		reqBody:     []rune(`{"a":1}`),
	}
	sendAPITUIRequest(state)
	if gotMethod != "POST" {
		t.Errorf("method: got %q", gotMethod)
	}
	if gotHeader != "hello" {
		t.Errorf("header: got %q", gotHeader)
	}
	if gotBody != `{"a":1}` {
		t.Errorf("body: got %q", gotBody)
	}
	if state.responseErr != "" {
		t.Fatalf("unexpected error: %q", state.responseErr)
	}
	if !strings.Contains(state.respHeadersText, "X-Resp: yes") {
		t.Errorf("response headers tab should contain the response header, got %q", state.respHeadersText)
	}
	if state.respBodyText != "ok" {
		t.Errorf("response body tab should contain the response body, got %q", state.respBodyText)
	}
	if len(state.history) != 1 || state.history[0].Method != "POST" || state.history[0].URL != server.URL {
		t.Fatalf("expected the request to be recorded in history, got %+v", state.history)
	}
}

func TestSendAPITUIRequestReportsInvalidHeader(t *testing.T) {
	withTempAPIHistoryDir(t)
	state := &apiTUIState{url: []rune("https://example.com"), reqHeaders: []rune("not-a-header")}
	sendAPITUIRequest(state)
	if state.responseErr == "" {
		t.Error("expected an error for a malformed header")
	}
	if state.response != nil {
		t.Errorf("expected no response when the request is invalid, got %+v", state.response)
	}
	if len(state.history) != 0 {
		t.Errorf("a failed request should not be recorded in history, got %+v", state.history)
	}
}

func TestAPITUIEscAndCtrlXExit(t *testing.T) {
	state := &apiTUIState{}
	if !handleAPITUIKey(state, keyEvent(tcell.KeyEscape, 0)) {
		t.Error("Escape should signal the event loop to exit")
	}
	if !handleAPITUIKey(state, keyEvent(tcell.KeyCtrlX, 0)) {
		t.Error("Ctrl+X should signal the event loop to exit")
	}
}

func TestAPITUICtrlCDoesNotExit(t *testing.T) {
	state := &apiTUIState{focus: apiFocusURL, url: []rune("https://example.com")}
	if handleAPITUIKey(state, keyEvent(tcell.KeyCtrlC, 0)) {
		t.Error("Ctrl+C should no longer exit the API client UI")
	}
}

func TestCopyFocusedContentCopiesWhicheverFieldIsFocused(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()

	state := &apiTUIState{focus: apiFocusURL, url: []rune("https://example.com/x")}
	copyFocusedContent(screen, state)
	if state.status != "Copied" {
		t.Fatalf("expected a Copied status, got %q", state.status)
	}
	if got := string(screen.GetClipboardData()); got != "https://example.com/x" {
		t.Errorf("expected the URL to be copied, got %q", got)
	}

	state = &apiTUIState{focus: apiFocusReqBody, reqBody: []rune(`{"a":1}`)}
	copyFocusedContent(screen, state)
	if got := string(screen.GetClipboardData()); got != `{"a":1}` {
		t.Errorf("expected the request body to be copied, got %q", got)
	}

	state = &apiTUIState{focus: apiFocusRespBody, respActiveTab: 1, respBodyText: "resp-body", respHeadersText: "resp-headers"}
	copyFocusedContent(screen, state)
	if got := string(screen.GetClipboardData()); got != "resp-body" {
		t.Errorf("expected the active response tab (body) to be copied, got %q", got)
	}

	state = &apiTUIState{focus: apiFocusMethod, methodIndex: 1}
	copyFocusedContent(screen, state)
	if state.status != "Nothing to copy" {
		// methodIndex 1 is POST, which is non-empty, so this should have copied "POST".
		if got := string(screen.GetClipboardData()); got != "POST" {
			t.Errorf("expected the method to be copied, got %q", got)
		}
	}
}

func TestSetFocusSyncsRequestActiveTab(t *testing.T) {
	state := &apiTUIState{}
	setFocus(state, apiFocusReqBody)
	if state.reqActiveTab != 1 {
		t.Errorf("focusing Request Body should show the Body tab, got %d", state.reqActiveTab)
	}
	setFocus(state, apiFocusReqHeaders)
	if state.reqActiveTab != 0 {
		t.Errorf("focusing Request Headers should show the Headers tab, got %d", state.reqActiveTab)
	}
}

// TestAPITUIResponseTabsSwitchViaTabAndScrollIndependently mirrors the Request section:
// Tab/Shift+Tab moves between the Headers and Body tabs (not the arrow keys), and each
// tab remembers its own scroll position.
func TestAPITUIResponseTabsSwitchViaTabAndScrollIndependently(t *testing.T) {
	state := &apiTUIState{
		focus:           apiFocusRespHeaders,
		respHeadersText: "h1\nh2\nh3\nh4\nh5",
		respBodyText:    "b1\nb2\nb3\nb4\nb5",
		respWidth:       40,
		respHeight:      2,
	}
	if state.respActiveTab != 0 {
		t.Fatalf("expected Headers tab active when focus starts there, got %d", state.respActiveTab)
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyDown, 0))
	if state.respHeadersOffset != 1 {
		t.Errorf("headers offset: got %d", state.respHeadersOffset)
	}
	if state.respBodyOffset != 0 {
		t.Errorf("body offset should be untouched: got %d", state.respBodyOffset)
	}

	handleAPITUIKey(state, keyEvent(tcell.KeyTAB, 0))
	if state.focus != apiFocusRespBody || state.respActiveTab != 1 {
		t.Fatalf("Tab should move focus to Response Body and show its tab, got focus=%v tab=%d", state.focus, state.respActiveTab)
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyDown, 0))
	if state.respBodyOffset != 1 {
		t.Errorf("body offset: got %d", state.respBodyOffset)
	}
	if state.respHeadersOffset != 1 {
		t.Errorf("headers offset should be untouched after switching tabs: got %d", state.respHeadersOffset)
	}

	handleAPITUIKey(state, keyEvent(tcell.KeyBacktab, 0))
	if state.focus != apiFocusRespHeaders || state.respActiveTab != 0 {
		t.Errorf("Shift+Tab should switch back to the Headers tab, got focus=%v tab=%d", state.focus, state.respActiveTab)
	}
}

// TestAPITUIArrowKeysNoLongerSwitchResponseTabs locks in the fix for the Request/Response
// inconsistency: both sections now switch their Headers/Body tabs the same way (Tab), not
// via the arrow keys.
func TestAPITUIArrowKeysNoLongerSwitchResponseTabs(t *testing.T) {
	state := &apiTUIState{focus: apiFocusRespHeaders, respActiveTab: 0}
	handleAPITUIKey(state, keyEvent(tcell.KeyRight, 0))
	if state.respActiveTab != 0 || state.focus != apiFocusRespHeaders {
		t.Errorf("Right should no longer switch response tabs, got focus=%v tab=%d", state.focus, state.respActiveTab)
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyLeft, 0))
	if state.respActiveTab != 0 || state.focus != apiFocusRespHeaders {
		t.Errorf("Left should no longer switch response tabs, got focus=%v tab=%d", state.focus, state.respActiveTab)
	}
}

func TestAPITUIHistoryNavigateLoadAndPin(t *testing.T) {
	withTempAPIHistoryDir(t)
	state := &apiTUIState{
		focus: apiFocusHistoryList,
		history: []apiHistoryEntry{
			{Method: "GET", URL: "https://a.example.com", Headers: []apiHeader{{Key: "X-A", Value: "1"}}},
			{Method: "POST", URL: "https://b.example.com", Body: `{"b":1}`},
		},
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyDown, 0))
	if state.historySelected != 1 {
		t.Fatalf("Down should select the second entry, got %d", state.historySelected)
	}
	handleAPITUIKey(state, keyEvent(tcell.KeyUp, 0))
	if state.historySelected != 0 {
		t.Fatalf("Up should select the first entry, got %d", state.historySelected)
	}

	handleAPITUIKey(state, keyEvent(tcell.KeyEnter, 0))
	if state.focus != apiFocusURL {
		t.Errorf("Enter on a history entry should move focus to the URL field, got %v", state.focus)
	}
	if string(state.url) != "https://a.example.com" {
		t.Errorf("Enter should load the entry's URL, got %q", string(state.url))
	}
	if !strings.Contains(string(state.reqHeaders), "X-A: 1") {
		t.Errorf("Enter should load the entry's headers, got %q", string(state.reqHeaders))
	}

	state.focus = apiFocusHistoryList
	handleAPITUIKey(state, keyEvent(tcell.KeyRune, 'p'))
	if !state.history[0].Pinned && !state.history[1].Pinned {
		t.Fatal("expected the selected entry to become pinned")
	}
}

func TestAPITUIHistorySearchFiltersList(t *testing.T) {
	state := &apiTUIState{
		history: []apiHistoryEntry{
			{Method: "GET", URL: "https://a.example.com/login"},
			{Method: "POST", URL: "https://b.example.com/users"},
		},
	}
	state.historyFilter = []rune("login")
	filtered := state.filteredHistory()
	if len(filtered) != 1 || filtered[0].URL != "https://a.example.com/login" {
		t.Fatalf("expected the filter to narrow to the login entry, got %+v", filtered)
	}
}

func TestAPITUIRendersMainPanels(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 32)

	state := &apiTUIState{
		focus: apiFocusURL,
		url:   []rune("https://example.com"),
		response: &apiResponse{
			Status:  "200 OK",
			Elapsed: 0,
			Size:    2,
			Headers: []apiHeader{{Key: "Content-Type", Value: "text/plain"}},
			Body:    "ok",
		},
		respHeadersText: "Content-Type: text/plain",
		respBodyText:    "ok",
		history: []apiHistoryEntry{
			{Method: "GET", URL: "https://example.com"},
		},
	}
	drawAPITUI(screen, state)

	r, _, _, _ := screen.GetContent(3, 0)
	if r != 'S' { // "Search" sidebar box title
		t.Fatalf("history sidebar was not rendered, got %q", r)
	}
}

func TestAPITUITooSmallShowsMessageWithoutPanicking(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 10)
	drawAPITUI(screen, &apiTUIState{})
}
