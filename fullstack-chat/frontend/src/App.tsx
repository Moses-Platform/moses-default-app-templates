/**
 * Fullstack Chat Roundtrip — reference template demonstrating the app↔Moses-Manager
 * round-trip with all chat surfaces wired up.
 *
 * MOSES ROUTING (CHAT-pswm.2/.8/.9 — canonical reference impl):
 *   - Calls to THIS APP's backend use relative paths: `fetch('api/v1/entries')`.
 *     They route through the app's nginx proxy to our Go backend.
 *   - Calls to MOSES platform actions go through `window.moses.actions.invoke`,
 *     supplied by the iframe SDK loaded in index.html. The SDK POSTs to
 *     `/__moses/invoke` on this app's OWN backend (same-origin under the
 *     iframe's nginx subpath); the backend's mosesproxy-go handler forwards
 *     pod-to-pod to moses-backend with the user's JWT preserved. The
 *     iframe never contacts moses-backend directly — see iframe_sdk_handler.go
 *     for the SDK source and shared/mosesproxy-go/proxy.go for the proxy.
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

async function fetchEntries(): Promise<Entry[]> {
  const r = await fetch('api/v1/entries', { credentials: 'same-origin' });
  if (!r.ok) throw new Error(`entries fetch failed: ${r.status}`);
  const j = (await r.json()) as EntriesResponse;
  return j.entries ?? [];
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
      // CHAT-pswm.9 — fire the chat_prompt via the iframe SDK
      // (loaded by index.html from /api/v1/sdk/iframe-sdk.js).
      // The SDK POSTs to /__moses/invoke on this app's own backend,
      // which forwards pod-to-pod to moses-backend with the user's
      // JWT preserved. If the SDK script failed to load (offline,
      // gateway misconfig, or the proxy backend is down), `window.moses`
      // is undefined and we surface a clear error instead of a TypeError.
      const invoke = window.moses?.actions?.invoke;
      if (typeof invoke !== 'function') {
        throw new Error(
          'Moses SDK not loaded — /api/v1/sdk/iframe-sdk.js failed to fetch. ' +
            'Is the iframe served via the Moses platform?',
        );
      }
      const result = await invoke(ACTION_ID, { topic: trimmed });
      const conversationId =
        (result as { result?: { conversationId?: string } } | undefined)?.result?.conversationId;
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
      // The SDK reshapes the platform's structured 4xx envelope so .hint
      // (e.g. CHAT-mux7 action_not_activated) lands on the error object
      // directly. Prefer hint > message so the user sees the actionable
      // remediation rather than a status-code-prefixed JSON blob.
      const e = err as { hint?: string; message?: string; status?: number };
      const fallback =
        typeof e?.status === 'number'
          ? `invoke failed (${e.status}): ${e.message ?? 'unknown error'}`
          : e?.message ?? 'invoke failed';
      setStatus({ kind: 'error', message: e?.hint ?? fallback });
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
