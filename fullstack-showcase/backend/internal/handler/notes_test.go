package handler

import (
	"testing"
)

// TestExtractNoteID verifies the literal-segment ID extraction works at
// both root and MOSES_BASE_PATH-prefixed mounts. Pre-CHAT-pbup.17 the
// dispatcher's TrimPrefix(r.URL.Path, "/api/v1/notes") returned the
// sub-path-prefixed string at the sub-path mount, mis-classifying
// list/create requests as "has-an-ID" and crashing the GetNote handler
// with a sub-path-as-id query. FAILS on the old code, PASSES on the fix.
func TestExtractNoteID(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		// Collection (LIST / CREATE) at both mounts → empty ID.
		{"root collection", "/api/v1/notes", ""},
		{"root collection trailing slash", "/api/v1/notes/", ""},
		{"sub-path collection", "/apps/tenant/slug/api/v1/notes", ""},
		{"sub-path collection trailing slash", "/apps/tenant/slug/api/v1/notes/", ""},

		// Item-level (GET / DELETE) at both mounts → ID extracted.
		{"root item", "/api/v1/notes/abc-123", "abc-123"},
		{"root item trailing slash", "/api/v1/notes/abc-123/", "abc-123"},
		{"sub-path item", "/apps/tenant/slug/api/v1/notes/abc-123", "abc-123"},
		{"sub-path item trailing slash", "/apps/tenant/slug/api/v1/notes/abc-123/", "abc-123"},

		// Defensive: an unrelated URL must return "" (handler's own
		// guards then return 400/404; the helper itself does not panic).
		{"unrelated path", "/health", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractNoteID(tc.url)
			if got != tc.want {
				t.Errorf("extractNoteID(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
