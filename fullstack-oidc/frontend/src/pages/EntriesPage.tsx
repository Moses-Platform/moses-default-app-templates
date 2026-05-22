import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '../auth/useAuth';
import { createEntry, listEntries, type Entry } from '../auth/api';
import './pages.css';

/**
 * Per-user data demo. The `entries` table is scoped by BOTH the deploy-
 * pinned tenant id and the authenticated OIDC subject — so the validated
 * identity from `oidcauth` feeds row-level ownership. A protected
 * read/write route.
 */
export default function EntriesPage() {
  const { phase, me, signIn } = useAuth();
  const [entries, setEntries] = useState<Entry[]>([]);
  const [draft, setDraft] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const reload = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setEntries(await listEntries());
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to load entries');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (phase === 'authenticated') void reload();
  }, [phase, reload]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    const body = draft.trim();
    if (!body) return;
    setError('');
    try {
      const created = await createEntry(body);
      setEntries((prev) => [created, ...prev]);
      setDraft('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'failed to create entry');
    }
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
          <button className="btn btn-primary" type="submit" disabled={!draft.trim()}>
            Save
          </button>
        </form>

        {error && <div className="banner banner-error">{error}</div>}

        {loading ? (
          <p className="empty-note">Loading your entries…</p>
        ) : entries.length === 0 ? (
          <p className="empty-note">No entries yet — add your first one above.</p>
        ) : (
          <ul className="entry-list">
            {entries.map((entry) => (
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
