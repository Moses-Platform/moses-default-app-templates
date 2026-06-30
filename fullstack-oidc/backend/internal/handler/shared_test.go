package handler

import (
	"strings"
	"testing"
)

// TestSharedNotesListQueryIsTenantScopedOnly guards the single invariant that
// makes `shared_notes` the TENANT space: reads filter by tenant id and NOTHING
// else. If a future edit adds an `author_sub` predicate, notes an agent posts
// through a workspace-tool call (which arrive under the agent's X-Moses-User-ID)
// become invisible to the human — the exact bug this table exists to prevent.
func TestSharedNotesListQueryIsTenantScopedOnly(t *testing.T) {
	q := listSharedNotesByTenantQuery

	if !strings.Contains(q, "WHERE tenant_id = $1") {
		t.Fatalf("shared-notes list must filter by tenant_id; query=%q", q)
	}
	// The read must never narrow by author — that is what keeps the list shared.
	if strings.Contains(q, "author_sub =") || strings.Contains(q, "author_sub=") {
		t.Fatalf("shared-notes list must NOT filter by author_sub (it is the tenant space); query=%q", q)
	}
}
