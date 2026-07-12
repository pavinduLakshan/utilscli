package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// apiHeader is a single HTTP header on a parsed request.
type apiHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// apiRequest is a single HTTP request built from the api tool's plain-text spec.
type apiRequest struct {
	Method  string
	URL     string
	Headers []apiHeader
	Body    string
}

// parseAPIRequest reads a request from a small plain-text spec:
//
//	<METHOD> <URL>
//	Header-Name: value
//	Header-Name: value
//	(blank line)
//	body...
//
// The method may be omitted, in which case a bare URL on the first line defaults to GET.
func parseAPIRequest(input string) (*apiRequest, error) {
	if strings.TrimSpace(input) == "" {
		return nil, errors.New("empty request; expected '<METHOD> <URL>' on the first line")
	}
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")

	first := strings.TrimSpace(lines[0])
	if first == "" {
		return nil, errors.New("first line must be '<METHOD> <URL>' or a bare URL")
	}
	req := &apiRequest{Method: "GET"}
	switch fields := strings.Fields(first); len(fields) {
	case 1:
		req.URL = fields[0]
	case 2:
		req.Method = strings.ToUpper(fields[0])
		req.URL = fields[1]
	default:
		return nil, fmt.Errorf("invalid request line %q; expected '<METHOD> <URL>'", first)
	}

	i := 1
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q; expected 'Name: value'", line)
		}
		req.Headers = append(req.Headers, apiHeader{Key: strings.TrimSpace(key), Value: strings.TrimSpace(value)})
	}
	if i < len(lines) {
		req.Body = strings.Join(lines[i:], "\n")
	}
	return req, nil
}

// parseHeaderLines parses one "Name: value" header per non-empty line, as used by the
// API client TUI's dedicated headers field. A line whose value is left blank (e.g. an
// unfilled entry from the pre-populated common-header template) is silently skipped
// rather than sent as an empty header.
func parseHeaderLines(text string) ([]apiHeader, error) {
	var headers []apiHeader
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q; expected 'Name: value'", line)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if value == "" {
			continue
		}
		headers = append(headers, apiHeader{Key: key, Value: value})
	}
	return headers, nil
}

// apiResponse is a structured HTTP response, kept separate from its text formatting so the
// TUI can display headers and body as distinct sections instead of one flat block of text.
type apiResponse struct {
	Status  string
	Elapsed time.Duration
	Size    int
	Headers []apiHeader
	Body    string
}

// sendAPIRequest parses the plain-text spec and executes the HTTP request.
func sendAPIRequest(input string) (string, error) {
	req, err := parseAPIRequest(input)
	if err != nil {
		return "", err
	}
	resp, err := executeAPIRequest(req)
	if err != nil {
		return "", err
	}
	return formatAPIResponse(resp), nil
}

// executeAPIRequest runs an already-built request and returns its structured response.
func executeAPIRequest(req *apiRequest) (*apiResponse, error) {
	httpReq, err := http.NewRequest(req.Method, req.URL, strings.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	for _, h := range req.Headers {
		httpReq.Header.Set(h.Key, h.Value)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	headerKeys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)
	headers := make([]apiHeader, 0, len(headerKeys))
	for _, k := range headerKeys {
		headers = append(headers, apiHeader{Key: k, Value: strings.Join(resp.Header[k], ", ")})
	}

	body := string(respBody)
	if pretty, err := prettyJSON(body); err == nil {
		body = pretty
	}

	return &apiResponse{
		Status:  resp.Status,
		Elapsed: elapsed.Round(time.Millisecond),
		Size:    len(respBody),
		Headers: headers,
		Body:    body,
	}, nil
}

// formatAPIResponse renders a structured response as the single block of text the
// one-shot CLI tool prints (status line, then headers, then a blank line, then the body).
func formatAPIResponse(resp *apiResponse) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s (%s, %d bytes)\n", resp.Status, resp.Elapsed, resp.Size)
	for _, h := range resp.Headers {
		fmt.Fprintf(&out, "%s: %s\n", h.Key, h.Value)
	}
	out.WriteString("\n")
	out.WriteString(resp.Body)
	return out.String()
}
