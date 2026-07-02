/**
 * App shell — minimal placeholder left by clean_out_template.sh.
 *
 * The Moses integration reference modules SURVIVED the cleanout — build on
 * them instead of re-deriving the contracts:
 *
 *   - src/moses/invoke.ts       — fire platform actions declared in
 *     moses-app.config.json via the iframe SDK (window.moses.actions.invoke
 *     → POST /__moses/invoke on this app's own backend → pod-to-pod to
 *     moses-backend with the user's JWT preserved). Use shapeInvokeError
 *     for user-facing failures (prefers the platform's structured hint).
 *   - src/moses/hostMessages.ts — origin-checked `moses_embed_chat_complete`
 *     listener hook + `moses_embed_open_chat` announce helper (pin the
 *     host's chat sidebar to a conversation you started).
 *   - src/api/client.ts         — typed fetch transport. RELATIVE paths only
 *     ('api/v1/...', never '/api/...') so the nginx subpath proxy works
 *     under /apps/<tenant>/<slug>/.
 *   - src/api/hooks.ts + queryKeys.ts — TanStack Query data layer: reads go
 *     through useQuery, never a useEffect fetch.
 *
 * index.html keeps the plumbing: moses-base-path meta, no-flash theme init,
 * and the iframe SDK <script>. main.tsx keeps QueryClientProvider + the
 * moses-browser-logger install.
 */
import { useState } from 'react';
import { useMosesChatComplete, type CompletionMessage } from './moses/hostMessages';
import ThemeToggle from './components/ThemeToggle';
import './App.css';

export default function App() {
  const [lastCompletion, setLastCompletion] = useState<CompletionMessage | null>(null);

  // Host-shell completion push channel stays wired so chat_prompt actions
  // you add are observable immediately. Replace the state update with your
  // own handling (e.g. invalidate a TanStack Query key).
  useMosesChatComplete(setLastCompletion);

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-text">
          <h1>Your app starts here</h1>
          <p className="lede">
            The demo was cleaned out. Add routes in the backend&apos;s{' '}
            <code>demo_routes.go</code> successor, declare actions in{' '}
            <code>moses-app.config.json</code>, and build UI from{' '}
            <code>src/moses/</code> + <code>src/api/</code>.
          </p>
        </div>
        <ThemeToggle />
      </header>

      {lastCompletion ? (
        <p>
          Last Moses Manager completion: <code>{lastCompletion.conversationId}</code>
        </p>
      ) : null}
    </div>
  );
}
