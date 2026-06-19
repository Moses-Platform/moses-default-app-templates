import { useState, type FormEvent } from 'react';
import { useAuth } from '../auth/useAuth';
import { useEntries, useCreateEntry } from '../api/hooks';
import { getErrorMessage } from '../api/client';
import './pages.css';

/**
 * Per-user data demo. The `entries` table is scoped by BOTH the deploy-
 * pinned tenant id and the authenticated OIDC subject — so the validated
 * identity from `oidcauth` feeds row-level ownership. A protected
 * read/write route.
 *
 * Data lives in TanStack Query: the read is gated by auth state via
 * `enabled`, so the protected route is never hit while anonymous; the
 * create mutation invalidates the list. No useEffect, no manual loading
 * state — only the draft input is true client state.
 */
export default function EntriesPage() {
  const { phase, me, signIn } = useAuth();
  const authenticated = phase === 'authenticated';
  const { data: entries, isPending, isError, error } = useEntries(authenticated);
  const createEntry = useCreateEntry();
  const [draft, setDraft] = useState('');

  function onCreate(e: FormEvent) {
    e.preventDefault();
    const body = draft.trim();
    if (!body) return;
    createEntry.mutate(body, { onSuccess: () => setDraft('') });
  }

  if (phase === 'loading') {
    return (
      <div className="page">
        <h2>My Entries</h2>
        <p className="empty-note">Checking your session…</p>
      </div>
    );
  }

  if (phase === 'anonymous' || !me) {
    return (
      <div className="page">
        <h2>My Entries</h2>
        <p className="page-intro">
          <code>/api/v1/entries</code> is a protected route — the middleware
          returns 401 without a session.
        </p>
        <div className="card">
          <div className="banner banner-warn" style={{ marginBottom: 'var(--spacing-4)' }}>
            Sign in to view and create your entries.
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
      <h2>My Entries</h2>
      <p className="page-intro">
        These rows are owned by your OIDC subject (<code>{me.subject}</code>)
        inside this deploy’s tenant. Another signed-in user would see a
        different list — the authenticated identity drives row ownership.
      </p>

      <section className="card">
        <h3>
          Add an entry <span className="badge badge-protected">protected</span>
        </h3>
        <form className="entry-form" onSubmit={onCreate}>
          <input
            type="text"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Write something only you will see…"
            aria-label="Entry body"
            maxLength={500}
          />
          <button className="btn btn-primary" type="submit" disabled={!draft.trim() || createEntry.isPending}>
            {createEntry.isPending ? 'Saving…' : 'Save'}
          </button>
        </form>

        {createEntry.isError && <div className="banner banner-error">{getErrorMessage(createEntry.error)}</div>}
        {isError && <div className="banner banner-error">{getErrorMessage(error)}</div>}

        {isPending ? (
          <p className="empty-note">Loading your entries…</p>
        ) : entries && entries.length === 0 ? (
          <p className="empty-note">No entries yet — add your first one above.</p>
        ) : (
          <ul className="entry-list">
            {(entries ?? []).map((entry) => (
              <li key={entry.id}>
                <span>{entry.body}</span>
                <time dateTime={entry.created_at}>
                  {new Date(entry.created_at).toLocaleString()}
                </time>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
