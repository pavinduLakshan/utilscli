package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// apiHistoryEntry is one previously sent request, remembered across uc api sessions.
type apiHistoryEntry struct {
	Method    string      `json:"method"`
	URL       string      `json:"url"`
	Headers   []apiHeader `json:"headers,omitempty"`
	Body      string      `json:"body,omitempty"`
	Pinned    bool        `json:"pinned"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// maxUnpinnedHistory bounds how many unpinned entries are kept; pinned entries are never evicted.
const maxUnpinnedHistory = 50

// apiHistoryDir resolves where uc persists request history; overridable in tests.
var apiHistoryDir = defaultAPIHistoryDir

func defaultAPIHistoryDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "uc"), nil
}

func apiHistoryPath() (string, error) {
	dir, err := apiHistoryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "api-history.json"), nil
}

// loadAPIHistory returns the persisted history, or nil if none exists yet or it can't be read.
func loadAPIHistory() []apiHistoryEntry {
	path, err := apiHistoryPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err != nil {
		return nil
	}
	var entries []apiHistoryEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
}

func saveAPIHistory(entries []apiHistoryEntry) error {
	dir, err := apiHistoryDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path, err := apiHistoryPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// addOrUpdateHistory records a sent request, updating an existing entry with the same
// method and URL in place instead of duplicating it.
func addOrUpdateHistory(entries []apiHistoryEntry, method, url string, headers []apiHeader, body string) []apiHistoryEntry {
	for i, e := range entries {
		if e.Method == method && e.URL == url {
			entries[i].Headers = headers
			entries[i].Body = body
			entries[i].UpdatedAt = time.Now()
			return reorderHistory(entries)
		}
	}
	entries = append(entries, apiHistoryEntry{
		Method:    method,
		URL:       url,
		Headers:   headers,
		Body:      body,
		UpdatedAt: time.Now(),
	})
	return reorderHistory(entries)
}

// togglePin flips the pinned state of the entry matching method and URL.
func togglePin(entries []apiHistoryEntry, method, url string) []apiHistoryEntry {
	for i, e := range entries {
		if e.Method == method && e.URL == url {
			entries[i].Pinned = !entries[i].Pinned
			break
		}
	}
	return reorderHistory(entries)
}

// reorderHistory puts pinned entries first (most recently updated first), then unpinned
// entries (most recently updated first), evicting unpinned entries beyond the cap.
func reorderHistory(entries []apiHistoryEntry) []apiHistoryEntry {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Pinned != entries[j].Pinned {
			return entries[i].Pinned
		}
		return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
	})
	var pinned, unpinned []apiHistoryEntry
	for _, e := range entries {
		if e.Pinned {
			pinned = append(pinned, e)
		} else {
			unpinned = append(unpinned, e)
		}
	}
	if len(unpinned) > maxUnpinnedHistory {
		unpinned = unpinned[:maxUnpinnedHistory]
	}
	return append(pinned, unpinned...)
}

// filterHistory returns the entries whose method or URL contains query (case-insensitive).
func filterHistory(entries []apiHistoryEntry, query string) []apiHistoryEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return entries
	}
	var out []apiHistoryEntry
	for _, e := range entries {
		haystack := strings.ToLower(e.Method + " " + e.URL)
		if strings.Contains(haystack, query) {
			out = append(out, e)
		}
	}
	return out
}
