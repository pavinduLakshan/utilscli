package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseAPIRequestBareURL(t *testing.T) {
	req, err := parseAPIRequest("https://example.com/health")
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" || req.URL != "https://example.com/health" {
		t.Errorf("got %+v", req)
	}
	if len(req.Headers) != 0 || req.Body != "" {
		t.Errorf("expected no headers/body, got %+v", req)
	}
}

func TestParseAPIRequestWithHeadersAndBody(t *testing.T) {
	spec := "post https://example.com/users\n" +
		"Content-Type: application/json\n" +
		"Authorization: Bearer abc123\n" +
		"\n" +
		"{\"name\":\"a\"}"
	req, err := parseAPIRequest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" || req.URL != "https://example.com/users" {
		t.Errorf("method/url: got %+v", req)
	}
	want := []apiHeader{
		{Key: "Content-Type", Value: "application/json"},
		{Key: "Authorization", Value: "Bearer abc123"},
	}
	if len(req.Headers) != len(want) {
		t.Fatalf("headers: got %+v", req.Headers)
	}
	for i, h := range want {
		if req.Headers[i] != h {
			t.Errorf("header %d: got %+v, want %+v", i, req.Headers[i], h)
		}
	}
	if req.Body != `{"name":"a"}` {
		t.Errorf("body: got %q", req.Body)
	}
}

func TestParseAPIRequestRejectsEmptyAndInvalid(t *testing.T) {
	if _, err := parseAPIRequest(""); err == nil {
		t.Error("expected an error for empty input")
	}
	if _, err := parseAPIRequest("GET https://a.com bogus"); err == nil {
		t.Error("expected an error for a malformed request line")
	}
	if _, err := parseAPIRequest("https://a.com\nnot-a-header"); err == nil {
		t.Error("expected an error for a header line without a colon")
	}
}

func TestSendAPIRequest(t *testing.T) {
	var gotMethod, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	spec := "POST " + server.URL + "/ping\nAuthorization: Bearer xyz\n\n{\"a\":1}"
	result, err := sendAPIRequest(spec)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method: got %q", gotMethod)
	}
	if gotAuth != "Bearer xyz" {
		t.Errorf("auth header: got %q", gotAuth)
	}
	if gotBody != `{"a":1}` {
		t.Errorf("body: got %q", gotBody)
	}
	if !strings.Contains(result, "200 OK") {
		t.Errorf("result missing status: %q", result)
	}
	if !strings.Contains(result, `"ok": true`) {
		t.Errorf("result should pretty-print the JSON body: %q", result)
	}
}

func TestSendAPIRequestInvalidSpec(t *testing.T) {
	if _, err := sendAPIRequest(""); err == nil {
		t.Fatal("expected an error for an empty spec")
	}
}

func TestParseHeaderLines(t *testing.T) {
	headers, err := parseHeaderLines("Content-Type: application/json\n\n  Authorization: Bearer x  \n")
	if err != nil {
		t.Fatal(err)
	}
	want := []apiHeader{
		{Key: "Content-Type", Value: "application/json"},
		{Key: "Authorization", Value: "Bearer x"},
	}
	if len(headers) != len(want) {
		t.Fatalf("got %+v", headers)
	}
	for i, h := range want {
		if headers[i] != h {
			t.Errorf("header %d: got %+v, want %+v", i, headers[i], h)
		}
	}
}

func TestParseHeaderLinesRejectsMissingColon(t *testing.T) {
	if _, err := parseHeaderLines("not-a-header"); err == nil {
		t.Fatal("expected an error for a header line without a colon")
	}
}

func TestParseHeaderLinesSkipsBlankValues(t *testing.T) {
	headers, err := parseHeaderLines("Content-Type: \nAccept: \nAuthorization: Bearer x")
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 || headers[0].Key != "Authorization" {
		t.Fatalf("expected unfilled headers to be skipped, got %+v", headers)
	}
}
