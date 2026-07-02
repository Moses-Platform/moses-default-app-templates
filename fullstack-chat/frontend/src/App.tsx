/**
 * Fullstack Chat Roundtrip — DEMO UI for the app↔Moses-Manager round-trip.
 * clean_out_template.sh replaces this file with a minimal placeholder; the
 * integration reference code it consumes lives in src/moses/ (invoke.ts +
 * hostMessages.ts) and SURVIVES the cleanout — build on those modules.
 *
 * MOSES ROUTING (CHAT-pswm.2/.8/.9 — canonical reference impl):
 *   - Calls to THIS APP's backend use relative paths: `fetch('api/v1/entries')`.
 *     They route through the app's nginx proxy to our Go backend. The entries
 *     list (non-stream server state) lives in TanStack Query (api/hooks.ts).
 *   - Calls to MOSES platform actions go through src/moses/invoke.ts →
 *     `window.moses.actions.invoke`, supplied by the iframe SDK loaded in
 *     index.html. The SDK POSTs to `/__moses/invoke` on this app's OWN
 *     backend (same-origin under the iframe's nginx subpath); the backend's
 *     mosesproxy-go handler forwards pod-to-pod to moses-backend with the
 *     user's JWT preserved. The iframe never contacts moses-backend directly
 *     — see iframe_sdk_handler.go for the SDK source and
 *     shared/mosesproxy-go/proxy.go for the proxy.
 *
 * DATA-LAYER SCOPE (FRONTEND_DATA_LAYER.md):
 *   - Non-stream data (the entries list) is migrated to TanStack Query.
 *   - The imperative surfaces stay imperative on purpose: the SDK action invoke
 *     (an explicit on-click) and the host-shell postMessage listener (a push
 *     channel, not a data fetch). The completion postMessage invalidates the
 *     entries query so the feed reconciles immediately.
 */
import { useCallback, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useEntries } from './api/hooks';
import { queryKeys } from './api/queryKeys';
import {
  announceOpenChat,
  useMosesChatComplete,
  type CompletionMessage,
} from './moses/hostMessages';
import { conversationIdOf, invokeAction, shapeInvokeError } from './moses/invoke';
import ThemeToggle from './components/ThemeToggle';
import './App.css';

type StatusBanner =
  | { kind: 'idle' }
  | { kind: 'invoking' }
  | { kind: 'awaiting'; conversationId: string }
  | { kind: 'complete'; conversationId: string; preview?: string; reason?: string }
  | { kind: 'error'; message: string };

const APP_SLUG = 'fullstack-chat';
const ACTION_ID = 'generate-entry';

export default function App() {
  // Server state: the entries list lives in TanStack Query (with a steady-state
  // refetchInterval backstop — see api/hooks.ts). Client/UI state stays local.
  const { data: entries = [] } = useEntries();
  const queryClient = useQueryClient();
  const [topic, setTopic] = useState('');
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<StatusBanner>({ kind: 'idle' });

  // Invalidate the entries query so the feed reconciles with server truth.
  const refreshEntries = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.entries });
  }, [queryClient]);

  // Host-shell completion push channel (origin-checked in the hook):
  // reflect completion events into the status banner and reconcile the feed
  // immediately (faster than the steady-state refetch interval).
  const onComplete = useCallback(
    (msg: CompletionMessage) => {
      setStatus({
        kind: 'complete',
        conversationId: msg.conversationId,
        preview: msg.preview,
        reason: msg.finishReason,
      });
      refreshEntries();
    },
    [refreshEntries],
  );
  useMosesChatComplete(onComplete);

  async function onGenerate() {
    const trimmed = topic.trim();
    if (!trimmed) {
      setStatus({ kind: 'error', message: 'topic is required' });
      return;
    }
    setBusy(true);
    setStatus({ kind: 'invoking' });
    try {
      // CHAT-pswm.9 — fire the chat_prompt via the iframe SDK (guarded +
      // typed in src/moses/invoke.ts).
      const result = await invokeAction(ACTION_ID, { topic: trimmed });
      const conversationId = conversationIdOf(result);
      setStatus({ kind: 'awaiting', conversationId: conversationId ?? '(unknown)' });
      // Announce the conversation up to the host shell (sidebar pin +
      // completion-event forwarding) — see src/moses/hostMessages.ts.
      if (conversationId) {
        announceOpenChat(conversationId, APP_SLUG);
      }
      // Optimistically refresh: MM may complete fast.
      window.setTimeout(() => refreshEntries(), 1_000);
    } catch (err) {
      // hint > status-prefixed fallback — see shapeInvokeError.
      setStatus({ kind: 'error', message: shapeInvokeError(err) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-text">
          <h1>Fullstack Chat Roundtrip</h1>
          <p className="lede">
            Demonstrates the full app ↔ Moses-Manager roundtrip: button-fired{' '}
            <code>chat_prompt</code> action, MM call-back via the workspace-tools wedge,
            completion webhook, and host-shell postMessage signalling.
          </p>
        </div>
        <ThemeToggle />
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
        <button type="button" className="btn" onClick={onGenerate} disabled={busy || !topic.trim()}>
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
