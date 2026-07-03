# Issue Analysis — Issue #1: Cannot scroll through long output

## Classification
- **Type:** Bug. The interactive TUI truncates output that exceeds its visible output area and provides no way to reach the hidden lines. The issue also suggests a related copy-output enhancement.
- **Severity Assessment:** Medium
- **Affected Component(s):** Interactive terminal UI (`interactive.go`)
- **Affected Feature(s):** Tool output display and keyboard navigation

## Reproducibility
- **Reproducible:** Yes
- **Environment:** `fix/issue-1-scroll-output` from `main` at `82ff515`; Go 1.24.3; macOS arm64; tcell simulation screen at 100×20
- **Steps Executed:**
  1. Built the product with `GOCACHE=/tmp/utilscli-go-cache make build` and ran the baseline test suite successfully.
  2. Rendered the interactive frontend with ten output lines in a 100×20 tcell simulation screen.
  3. Confirmed that the first visible output row was `line 00`.
  4. Sent a Page Down key event and rendered the frontend again.
  5. Observed that the first visible output row remained `line 00` and the regression test failed.
- **Expected Behavior:** Long output can be scrolled so content below the output viewport is reachable.
- **Actual Behavior:** `drawOutput`/`drawLines` always render from the first wrapped line, while `handleTUIKey` has no output navigation path; content after the viewport height is unreachable.
- **Logs/Evidence:** `TestInteractiveOutputCanScroll` fails with `output did not scroll after Page Down; first row is still "line 00"`.

## Root Cause Analysis
The TUI state tracks selection, input cursor, focus, and result text, but it does not track an output scroll offset or output viewport height. The output pane is not focusable, Page Up/Page Down are ignored, and `drawLines` unconditionally begins at wrapped line zero. As a result, long output is clipped at the bottom of the available terminal area.

### Follow-up: Ctrl+Y does not update the system clipboard
- **Classification:** Bug, medium severity, affecting interactive output copying.
- **Reproducible:** Yes, on macOS arm64 with the real tcell TUI in an xterm-compatible PTY.
- **Steps Executed:** Built and started `./bin/uc`, entered `hello` in the Base64 encoder, ran it with Ctrl+R, pressed Ctrl+Y, exited, and compared the macOS clipboard with the expected `aGVsbG8=` output.
- **Expected Behavior:** The system clipboard contains the complete output.
- **Actual Behavior:** The TUI emitted OSC 52 with the correct Base64-encoded clipboard payload, but `pbpaste` did not return the expected output.
- **Logs/Evidence:** The frontend emitted `ESC ] 52 ; c ; YUdWc2JHOD0= ESC \\`; the subsequent clipboard comparison exited with status 1. tcell documents that terminals may block `SetClipboard` for security reasons.
- **Root Cause:** The implementation relies solely on tcell's OSC 52 terminal request. This request has no success signal and may be ignored, while the simulation screen only stores the request internally. A native OS clipboard path and user-visible result are required.
- **Post-fix verification:** Repeated the same real-TUI workflow after adding the native clipboard path. The footer reported `Output copied`, and `pbpaste` matched `aGVsbG8=` with exit status 0. The original clipboard was restored afterward.

## Test Coverage Assessment
- **Existing tests covering this path:** `TestInteractiveMenu` checks only that the primary panels render; input focus and cursor tests cover keyboard behavior for tools and the editor.
- **Coverage gaps identified:** No test covered output overflow, scrolling, offset clamping, output focus, or copying output.
- **Proposed test plan:**
  - Unit test: Verify output offsets clamp to the wrapped content range and reset when a tool is run or changed.
  - Integration test: Render long output in a simulation screen, send scrolling keys, and assert later lines become visible.
  - Negative/edge cases: Short/empty output must not scroll; resize and line wrapping must not leave the viewport beyond the end; copying with no output should be a no-op.

## Tests Summary
- **Tests created:** Long-output Page Down rendering, output offset clamping, short-output no-op scrolling, offset reset after execution, native and terminal clipboard copy paths, clipboard failure feedback, empty-output copy behavior, mouse-wheel scrolling, and Tab focus cycling through output.
- **Component and module targeted:** Interactive terminal UI state, event handling, rendering, and platform clipboard command selection in `interactive.go` and `clipboard.go`, exercised from `main_test.go` with tcell's simulation screen.
- **Assumptions or limitations:** Native copying uses `pbcopy` on macOS, `wl-copy`/`xclip`/`xsel` on Linux, and `clip` on Windows. OSC 52 remains as a fallback for remote or supported terminals.
