import { useState, type FormEvent } from 'react';
import { useAuth } from '../auth/useAuth';
import { useSharedNotes, useCreateSharedNote } from '../api/hooks';
import { getErrorMessage } from '../api/client';
import './pages.css';

/**
 * Tenant-shared data demo — the TENANT space, counterpart to My Entries (the
 * USER space). The `shared_notes` table is scoped by the deploy-pinned tenant
 * id ALONE, so every member of the workspace sees the same list. `author_sub`
 * records who wrote each note but never filters the read.
 *
 * Why this page exists: when an agent posts content into this app through a
 * Moses workspace-tool call, it arrives as the *agent's* identity
 * (X-Moses-User-ID), not the human's. Scope shared/agent-fed content by tenant
 * (this page) so it stays visible to everyone; reserve per-user scoping (My
 * Entries) for genuinely private data.
 *
 * Same data-layer rules as EntriesPage: read gated by `enabled`, create
 * invalidates the list, no useEffect.
 */
export default function SharedNotesPage() {
  const { phase, me, signIn } = useAuth();
  const authenticated = phase === 'authenticated';
  const { data: notes, isPending, isError, error } = useSharedNotes(authenticated);
  const createNote = useCreateSharedNote();
  const [draft, setDraft] = useState('');

  function onCreate(e: FormEvent) {
    e.preventDefault();
    const body = draft.trim();
    if (!body) return;
    createNote.mutate(body, { onSuccess: () => setDraft('') });
  }

  if (phase === 'loading') {
    return (
      <div className="page">
        <h2>Shared Notes</h2>
        <p className="empty-note">Checking your session…</p>
      </div>
    );
  }

  if (phase === 'anonymous' || !me) {
    return (
      <div className="page">
        <h2>Shared Notes</h2>
        <p className="page-intro">
          <code>/api/v1/shared-notes</code> is a protected route — the middleware
          returns 401 without a session.
        </p>
        <div className="card">
          <div className="banner banner-warn" style={{ marginBottom: 'var(--spacing-4)' }}>
            Sign in to view and post the workspace's shared notes.
          </div>
          <button className="btn btn-primary" onClick={signIn}>
            Sign in with Moses
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="page">
      <h2>Shared Notes</h2>
      <p className="page-intro">
        These rows are scoped to this deploy’s tenant, not to your user — so{' '}
        <strong>every member of the workspace sees the same list</strong>. This is
        also where content an agent posts via a Moses workspace tool shows up: that
        call arrives under the agent’s identity, so tenant-scoping is what keeps it
        visible to you. Contrast with <code>My Entries</code>, which is private to
        your OIDC subject.
      </p>

      <section className="card">
        <h3>
          Post a note <span className="badge badge-protected">protected</span>
        </h3>
        <form className="entry-form" onSubmit={onCreate}>
          <input
            type="text"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Write something the whole workspace can see…"
            aria-label="Shared note body"
            maxLength={500}
          />
          <button className="btn btn-primary" type="submit" disabled={!draft.trim() || createNote.isPending}>
            {createNote.isPending ? 'Posting…' : 'Post'}
          </button>
        </form>

        {createNote.isError && <div className="banner banner-error">{getErrorMessage(createNote.error)}</div>}
        {isError && <div className="banner banner-error">{getErrorMessage(error)}</div>}

        {isPending ? (
          <p className="empty-note">Loading shared notes…</p>
        ) : notes && notes.length === 0 ? (
          <p className="empty-note">No shared notes yet — post the first one above.</p>
        ) : (
          <ul className="entry-list">
            {(notes ?? []).map((note) => (
              <li key={note.id}>
                <span>
                  {note.body}
                  {/* author_sub is attribution only — it never filters the
                      list. A note whose author is not you was posted by another
                      member or by an agent via a workspace tool, yet you still
                      see it: that is the tenant space. */}
                  <small className="note-author">
                    {note.author_sub === me.subject ? 'you' : note.author_sub}
                  </small>
                </span>
                <time dateTime={note.created_at}>
                  {new Date(note.created_at).toLocaleString()}
                </time>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
