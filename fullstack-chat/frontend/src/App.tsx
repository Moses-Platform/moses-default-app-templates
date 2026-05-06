/**
 * Fullstack Chat Roundtrip — reference template demonstrating the app↔Moses-Manager
 * round-trip with all chat surfaces wired up.
 *
 * MOSES ROUTING:
 *   - Calls to THIS APP's backend use relative paths: `fetch('api/v1/entries')`.
 *     They route through the app's nginx proxy to our Go backend.
 *   - Calls to MOSES BACKEND use absolute paths via VITE_MOSES_API_BASE:
 *     `fetch(`${import.meta.env.VITE_MOSES_API_BASE}/apps/${slug}/actions/${id}/invoke`)`.
 *     The platform injects VITE_MOSES_API_BASE at build time
 *     (kaniko_build_service.go); falls back to relative for local dev.
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import './App.css';

type Entry = {
  id: string;
  text: string;
  source: string;
  created_at: string;
};

type EntriesResponse = { entries: Entry[]; count: number };

type CompletionMessage = {
  type: 'moses_embed_chat_complete';
  v: 1;
  conversationId: string;
  preview?: string;
  finishReason?: 'stop' | 'length' | 'error' | 'credential_unset';
};

// moses_embed_open_chat (v1) is iframe → host-shell. The app posts it to
// `window.parent` after a successful chat_prompt invoke so the host can
// (a) open the global chat sidebar pinned to the new conversation, and
// (b) register the conversationId so it can later forward
// `auto_response_ready` WS events back here as `moses_embed_chat_complete`.

type StatusBanner =
  | { kind: 'idle' }
  | { kind: 'invoking' }
  | { kind: 'awaiting'; conversationId: string }
  | { kind: 'complete'; conversationId: string; preview?: string; reason?: string }
  | { kind: 'error'; message: string };

const APP_SLUG = 'fullstack-chat';
const ACTION_ID = 'generate-entry';

const POLL_INTERVAL_MS = 2_000;

function mosesApiBase(): string {
  // Platform-injected at build time; relative fallback for standalone dev.
  return (import.meta.env.VITE_MOSES_API_BASE as string | undefined)?.replace(/\/+$/, '') ?? '';
}

async function fetchEntries(): Promise<Entry[]> {
  const r = await fetch('api/v1/entries', { credentials: 'same-origin' });
  if (!r.ok) throw new Error(`entries fetch failed: ${r.status}`);
  const j = (await r.json()) as EntriesResponse;
  return j.entries ?? [];
}

async function invokeChatPrompt(topic: string): Promise<{ conversationId?: string }> {
  const url = `${mosesApiBase()}/api/v1/apps/${APP_SLUG}/actions/${ACTION_ID}/invoke`;
  const r = await fetch(url, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ variables: { topic } }),
  });
  if (!r.ok) {
    const detail = await r.text();
    throw new Error(`invoke failed (${r.status}): ${detail.slice(0, 200)}`);
  }
  const j = (await r.json()) as { result?: { conversationId?: string } };
  return { conversationId: j.result?.conversationId };
}

export default function App() {
  const [entries, setEntries] = useState<Entry[]>([]);
  const [topic, setTopic] = useState('');
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<StatusBanner>({ kind: 'idle' });
  const lastSeenRef = useRef<number>(0);

  const refresh = useCallback(async () => {
    try {
      const next = await fetchEntries();
      setEntries(next);
    } catch (err) {
      console.warn('[fullstack-chat] poll failed', err);
    }
  }, []);

  // Initial fetch + steady-state polling.
  useEffect(() => {
    void refresh();
    const id = window.setInterval(() => {
      if (document.visibilityState === 'visible') void refresh();
    }, POLL_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [refresh]);

  // postMessage listener: opens host's chat sidebar for this conversation,
  // and reflects completion events into the status banner.
  useEffect(() => {
    function onMessage(e: MessageEvent) {
      if (e.origin !== window.location.origin) return;
      const data = e.data;
      if (typeof data !== 'object' || data === null) return;

      if (data.type === 'moses_embed_chat_complete') {
        const msg = data as CompletionMessage;
        if (msg.v !== 1 || typeof msg.conversationId !== 'string') return;
        setStatus({
          kind: 'complete',
          conversationId: msg.conversationId,
          preview: msg.preview,
          reason: msg.finishReason,
        });
        // Faster refresh on completion than the 2s steady-state poll.
        void refresh();
      }
      // moses_embed_open_chat is host-shell-bound (the host opens its own
      // sidebar). The app neither sends nor consumes it directly; it's
      // listed here for documentation and for tests that assert non-handling.
    }
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [refresh]);

  // Visual signal when new entries land via the roundtrip.
  useEffect(() => {
    if (entries.length > lastSeenRef.current) lastSeenRef.current = entries.length;
  }, [entries.length]);

  async function onGenerate() {
    const trimmed = topic.trim();
    if (!trimmed) {
      setStatus({ kind: 'error', message: 'topic is required' });
      return;
    }
    setBusy(true);
    setStatus({ kind: 'invoking' });
    try {
      const { conversationId } = await invokeChatPrompt(trimmed);
      setStatus(
        conversationId
          ? { kind: 'awaiting', conversationId }
          : { kind: 'awaiting', conversationId: '(unknown)' },
      );
      // Tell the host shell about this conversation so it (a) opens the
      // global chat sidebar pinned to this conversationId, and (b) forwards
      // the eventual `auto_response_ready` WS event back to us as a
      // `moses_embed_chat_complete` postMessage. Without this announce step
      // the host has no record of which conversations belong to this iframe.
      if (conversationId && window.parent && window.parent !== window) {
        try {
          window.parent.postMessage(
            {
              type: 'moses_embed_open_chat',
              v: 1,
              conversationId,
              app: APP_SLUG,
            },
            window.location.origin,
          );
        } catch (err) {
          // Cross-origin posts will throw — non-fatal; polling still works.
          console.warn('[fullstack-chat] moses_embed_open_chat post failed', err);
        }
      }
      // Optimistically refresh: MM may complete fast.
      window.setTimeout(() => void refresh(), 1_000);
    } catch (err) {
      setStatus({ kind: 'error', message: (err as Error).message });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="app">
      <header>
        <h1>Fullstack Chat Roundtrip</h1>
        <p className="lede">
          Demonstrates the full app ↔ Moses-Manager roundtrip: button-fired{' '}
          <code>chat_prompt</code> action, MM call-back via the workspace-tools wedge,
          completion webhook, and host-shell postMessage signalling.
        </p>
      </header>

      <section className="trigger">
        <label htmlFor="topic">Topic</label>
        <input
          id="topic"
          type="text"
          value={topic}
          onChange={(e) => setTopic(e.target.value)}
          placeholder="e.g. coffee, deployments, octopus"
          disabled={busy}
          maxLength={200}
        />
        <button type="button" onClick={onGenerate} disabled={busy || !topic.trim()}>
          {busy ? 'Asking Moses Manager…' : 'Generate entry via Moses Manager'}
        </button>
      </section>

      <StatusBlock status={status} />

      <section className="feed" aria-label="Entries feed">
        <h2>Feed ({entries.length})</h2>
        {entries.length === 0 ? (
          <p className="empty">
            No entries yet. Click the button to ask Moses Manager to generate one — it
            will appear here within a couple of seconds.
          </p>
        ) : (
          <ul>
            {entries.map((e) => (
              <li key={e.id} data-source={e.source}>
                <span className="text">{e.text}</span>
                <span className="meta">
                  <span className={`source source-${e.source}`}>{e.source}</span>
                  <time dateTime={e.created_at}>
                    {new Date(e.created_at).toLocaleTimeString()}
                  </time>
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function StatusBlock({ status }: { status: StatusBanner }) {
  switch (status.kind) {
    case 'idle':
      return null;
    case 'invoking':
      return <p className="status status-invoking">Invoking Moses Manager…</p>;
    case 'awaiting':
      return (
        <p className="status status-awaiting">
          Waiting for Moses Manager to respond. Conversation:{' '}
          <code>{status.conversationId}</code>
        </p>
      );
    case 'complete':
      return (
        <p className="status status-complete">
          Moses Manager completed (<code>{status.reason ?? 'stop'}</code>) —{' '}
          conversation <code>{status.conversationId}</code>.
          {status.preview ? (
            <>
              <br />
              <em>{status.preview}</em>
            </>
          ) : null}
        </p>
      );
    case 'error':
      return <p className="status status-error">Error: {status.message}</p>;
  }
}
