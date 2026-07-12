package main

import (
	"fmt"
	"testing"
	"time"
)

func withTempAPIHistoryDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	original := apiHistoryDir
	apiHistoryDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { apiHistoryDir = original })
}

func TestAddOrUpdateHistoryDeduplicatesByMethodAndURL(t *testing.T) {
	var entries []apiHistoryEntry
	entries = addOrUpdateHistory(entries, "GET", "https://a.example.com", nil, "")
	entries = addOrUpdateHistory(entries, "GET", "https://a.example.com", []apiHeader{{Key: "X-A", Value: "1"}}, "body")
	if len(entries) != 1 {
		t.Fatalf("expected a repeated method+URL to update in place, got %d entries", len(entries))
	}
	if entries[0].Body != "body" || len(entries[0].Headers) != 1 {
		t.Errorf("expected the existing entry to be updated, got %+v", entries[0])
	}
}

func TestAddOrUpdateHistoryOrdersMostRecentFirst(t *testing.T) {
	var entries []apiHistoryEntry
	entries = addOrUpdateHistory(entries, "GET", "https://a.example.com", nil, "")
	time.Sleep(2 * time.Millisecond)
	entries = addOrUpdateHistory(entries, "GET", "https://b.example.com", nil, "")
	if entries[0].URL != "https://b.example.com" {
		t.Errorf("expected the most recently sent request first, got %+v", entries)
	}
}

func TestTogglePinMovesEntryBeforeUnpinned(t *testing.T) {
	var entries []apiHistoryEntry
	entries = addOrUpdateHistory(entries, "GET", "https://a.example.com", nil, "")
	entries = addOrUpdateHistory(entries, "GET", "https://b.example.com", nil, "")
	entries = togglePin(entries, "GET", "https://a.example.com")
	if !entries[0].Pinned || entries[0].URL != "https://a.example.com" {
		t.Fatalf("expected the pinned entry first, got %+v", entries)
	}
	entries = togglePin(entries, "GET", "https://a.example.com")
	if entries[0].Pinned {
		t.Errorf("expected toggling pin again to unpin, got %+v", entries[0])
	}
}

func TestReorderHistoryEvictsOldestUnpinnedBeyondCap(t *testing.T) {
	var entries []apiHistoryEntry
	for i := 0; i < maxUnpinnedHistory+5; i++ {
		entries = addOrUpdateHistory(entries, "GET", fmt.Sprintf("https://example.com/%d", i), nil, "")
	}
	if len(entries) != maxUnpinnedHistory {
		t.Errorf("expected unpinned entries capped at %d, got %d", maxUnpinnedHistory, len(entries))
	}
}

func TestFilterHistoryMatchesMethodOrURLCaseInsensitively(t *testing.T) {
	entries := []apiHistoryEntry{
		{Method: "GET", URL: "https://a.example.com/Login"},
		{Method: "POST", URL: "https://b.example.com/users"},
	}
	filtered := filterHistory(entries, "login")
	if len(filtered) != 1 || filtered[0].URL != "https://a.example.com/Login" {
		t.Fatalf("got %+v", filtered)
	}
	filtered = filterHistory(entries, "post")
	if len(filtered) != 1 || filtered[0].Method != "POST" {
		t.Fatalf("got %+v", filtered)
	}
	if got := filterHistory(entries, ""); len(got) != 2 {
		t.Errorf("empty query should return everything, got %+v", got)
	}
}

func TestSaveAndLoadAPIHistoryRoundTrip(t *testing.T) {
	withTempAPIHistoryDir(t)

	var entries []apiHistoryEntry
	entries = addOrUpdateHistory(entries, "GET", "https://a.example.com", []apiHeader{{Key: "X-A", Value: "1"}}, "body")
	if err := saveAPIHistory(entries); err != nil {
		t.Fatal(err)
	}

	loaded := loadAPIHistory()
	if len(loaded) != 1 || loaded[0].URL != "https://a.example.com" || loaded[0].Headers[0].Key != "X-A" {
		t.Fatalf("got %+v", loaded)
	}
}

func TestLoadAPIHistoryReturnsNilWhenMissing(t *testing.T) {
	withTempAPIHistoryDir(t)
	if entries := loadAPIHistory(); entries != nil {
		t.Errorf("expected nil when no history file exists, got %+v", entries)
	}
}
