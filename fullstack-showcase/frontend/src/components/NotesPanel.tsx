import { useState, type FormEvent } from 'react';
import { useNotes, useCreateNote, useDeleteNote } from '../api/hooks';
import { getErrorMessage } from '../api/client';
import './NotesPanel.css';

// Canonical Moses data-layer demo: a read query + write mutations with automatic
// cache invalidation. Note what is ABSENT — no useEffect, no manual
// loading/error/data useState, no hand-rolled refetch. Query owns server state;
// the component owns only the form input (true client state).
export default function NotesPanel() {
  const { data: notes, isPending, isError, error } = useNotes();
  const createNote = useCreateNote();
  const deleteNote = useDeleteNote();
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');

  const handleCreate = (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;
    createNote.mutate(
      { title: title.trim(), content: content.trim() },
      { onSuccess: () => { setTitle(''); setContent(''); } },
    );
  };

  return (
    <div className="notes-panel">
      <p className="notes-panel__hint">
        Reads via <code>useNotes()</code>; create/delete via <code>useMutation</code> with automatic
        cache invalidation. Notes are tenant-scoped by the backend.
      </p>

      <form className="notes-panel__form" onSubmit={handleCreate}>
        <input
          className="notes-panel__title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Title"
          aria-label="Note title"
        />
        <input
          className="notes-panel__content"
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="Content"
          aria-label="Note content"
        />
        <button type="submit" className="btn" disabled={createNote.isPending || !title.trim()}>
          {createNote.isPending ? 'Adding…' : 'Add note'}
        </button>
      </form>

      {createNote.isError && <p className="notes-panel__error">Create failed: {getErrorMessage(createNote.error)}</p>}
      {deleteNote.isError && <p className="notes-panel__error">Delete failed: {getErrorMessage(deleteNote.error)}</p>}

      {isPending ? (
        <p className="notes-panel__muted">Loading notes…</p>
      ) : isError ? (
        <p className="notes-panel__error">Error: {getErrorMessage(error)}</p>
      ) : notes && notes.length > 0 ? (
        <ul className="notes-panel__list">
          {notes.map((n) => {
            const removing = deleteNote.isPending && deleteNote.variables === n.id;
            return (
              <li key={n.id} className="notes-panel__item">
                <span className="notes-panel__text">
                  <strong>{n.title}</strong>{n.content ? ` — ${n.content}` : ''}
                </span>
                <button
                  type="button"
                  className="btn btn-danger btn-sm btn-icon"
                  onClick={() => deleteNote.mutate(n.id)}
                  disabled={removing}
                  aria-label={`Delete ${n.title}`}
                >
                  {removing ? '…' : '✕'}
                </button>
              </li>
            );
          })}
        </ul>
      ) : (
        <p className="notes-panel__muted">No notes yet — add one above.</p>
      )}
    </div>
  );
}
