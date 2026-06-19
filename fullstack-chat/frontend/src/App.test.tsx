import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';
import type { Entry } from './api/client';

// Canonical test convention for the Query data layer: mock the typed client so
// the entries query resolves deterministically (relative URLs reject in jsdom),
// then wrap renders in a QueryClientProvider with retries OFF. `setEntries`
// lets each test control what the feed loads. Copy this wrapper into any test
// that renders components using ../api/hooks.
const getEntriesMock = vi.fn<() => Promise<{ entries: Entry[]; count: number }>>();
vi.mock('./api/client', () => ({
  getEntries: () => getEntriesMock(),
}));

function setEntries(entries: Entry[]): void {
  getEntriesMock.mockResolvedValue({ entries, count: entries.length });
}

function renderApp() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
}

/**
 * CHAT-pswm.9 — the in-iframe SDK is supplied via <script src="..."> on the
 * real platform. In tests we stub the runtime shape directly on window.moses.
 */
function installMosesSDK(
  invoke?: (actionId: string, variables?: Record<string, unknown>) => Promise<unknown>,
): ReturnType<typeof vi.fn> {
  const spy = vi.fn(
    invoke ??
      (async () => ({ status: 'succeeded', result: { conversationId: 'conv-default-1' } })),
  );
  (window as unknown as { moses?: unknown }).moses = {
    __iframeSDKVersion: '1.0.0-test',
    actions: { invoke: spy },
  };
  return spy;
}

describe('Fullstack Chat Roundtrip — App', () => {
  beforeEach(() => {
    setEntries([]); // default: empty feed
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    delete (window as unknown as { moses?: unknown }).moses;
  });

  it('renders the empty-state message when no entries exist', async () => {
    renderApp();
    expect(await screen.findByText(/no entries yet/i)).toBeTruthy();
  });

  it('renders entries returned by the backend', async () => {
    setEntries([
      {
        id: '00000000-0000-0000-0000-000000000001',
        text: 'A witty first entry',
        source: 'moses_manager',
        created_at: '2026-01-01T12:00:00Z',
      },
    ]);
    renderApp();
    expect(await screen.findByText('A witty first entry')).toBeTruthy();
  });

  it('disables the generate button until a topic is typed', async () => {
    renderApp();
    const button = await screen.findByRole('button', { name: /generate entry/i });
    expect((button as HTMLButtonElement).disabled).toBe(true);
  });

  // CHAT-pswm.9 — the round-trip goes through window.moses.actions.invoke. We
  // stub the runtime contract; App.tsx hands the typed topic to the SDK with
  // the (actionId, variables) shape.
  it('invokes window.moses.actions.invoke with the action id and typed topic', async () => {
    const spy = installMosesSDK(async () => ({
      status: 'succeeded',
      result: { conversationId: 'conv-test-1' },
    }));
    renderApp();
    fireEvent.change((await screen.findByLabelText(/topic/i)) as HTMLInputElement, {
      target: { value: 'octopus' },
    });
    fireEvent.click(await screen.findByRole('button', { name: /generate entry/i }));

    await waitFor(() => expect(spy).toHaveBeenCalled());
    expect(spy).toHaveBeenCalledWith('generate-entry', { topic: 'octopus' });
  });

  it('posts moses_embed_open_chat to window.parent after a successful invoke', async () => {
    installMosesSDK(async () => ({
      status: 'succeeded',
      result: { conversationId: 'conv-announce-1' },
    }));
    const parentPostSpy = vi.fn();
    const original = Object.getOwnPropertyDescriptor(window, 'parent');
    Object.defineProperty(window, 'parent', {
      configurable: true,
      get: () => ({ postMessage: parentPostSpy } as unknown as Window),
    });

    try {
      renderApp();
      fireEvent.change((await screen.findByLabelText(/topic/i)) as HTMLInputElement, {
        target: { value: 'walrus' },
      });
      fireEvent.click(await screen.findByRole('button', { name: /generate entry/i }));

      await waitFor(() => expect(parentPostSpy).toHaveBeenCalled());
      const announce = parentPostSpy.mock.calls.find(
        (args) => (args[0] as Record<string, unknown> | undefined)?.type === 'moses_embed_open_chat',
      );
      expect(announce).toBeTruthy();
      const envelope = announce?.[0] as Record<string, unknown>;
      expect(envelope.v).toBe(1);
      expect(envelope.conversationId).toBe('conv-announce-1');
      expect(envelope.app).toBe('fullstack-chat');
      expect(announce?.[1]).toBe(window.location.origin); // targeted origin, not '*'
    } finally {
      if (original) Object.defineProperty(window, 'parent', original);
      else Object.defineProperty(window, 'parent', { configurable: true, get: () => window });
    }
  });

  // CHAT-pswm.9 — when the SDK script fails to load, window.moses is undefined.
  // App.tsx must surface a clear "Moses SDK not loaded" error.
  it('shows "Moses SDK not loaded" when window.moses is undefined', async () => {
    renderApp();
    fireEvent.change((await screen.findByLabelText(/topic/i)) as HTMLInputElement, {
      target: { value: 'orca' },
    });
    fireEvent.click(await screen.findByRole('button', { name: /generate entry/i }));
    await waitFor(() => expect(screen.getByText(/Moses SDK not loaded/i)).toBeTruthy());
  });

  it('updates the status banner on a moses_embed_chat_complete postMessage', async () => {
    renderApp();
    await screen.findByText(/no entries yet/i);

    await act(async () => {
      window.dispatchEvent(
        new MessageEvent('message', {
          origin: window.location.origin,
          data: {
            type: 'moses_embed_chat_complete',
            v: 1,
            conversationId: 'conv-finished-1',
            finishReason: 'stop',
            preview: 'Done.',
          },
        }),
      );
    });

    await waitFor(() => expect(screen.getByText(/Moses Manager completed/i)).toBeTruthy());
  });

  // CHAT-mux7 / CHAT-pswm.9 — a 409 action_not_activated carries a structured
  // `hint`; the template renders the hint verbatim, NOT the opaque "(409): {json}".
  it('renders the platform 409 action_not_activated hint instead of raw JSON', async () => {
    const HINT =
      "Open the app's tab in Moses Manager and approve permissions in the banner above the panel.";
    installMosesSDK(async () => {
      const err = new Error('action not yet activated; awaiting grant approval') as Error & {
        status?: number;
        code?: string;
        hint?: string;
      };
      err.status = 409;
      err.code = 'action_not_activated';
      err.hint = HINT;
      throw err;
    });

    renderApp();
    fireEvent.change((await screen.findByLabelText(/topic/i)) as HTMLInputElement, {
      target: { value: 'octopus' },
    });
    fireEvent.click(await screen.findByRole('button', { name: /generate entry/i }));

    await waitFor(() => expect(screen.getByText(new RegExp(HINT.slice(0, 30), 'i'))).toBeTruthy());
    expect(screen.queryByText(/\(409\):/)).toBeNull();
  });

  // CHAT-mux7 fallback: a 4xx without a hint falls back to "invoke failed (status): detail".
  it('falls back to "invoke failed" when the 4xx body has no hint', async () => {
    installMosesSDK(async () => {
      const err = new Error('rate limit exceeded for this action') as Error & {
        status?: number;
        code?: string;
      };
      err.status = 429;
      err.code = 'rate_limited';
      throw err;
    });

    renderApp();
    fireEvent.change((await screen.findByLabelText(/topic/i)) as HTMLInputElement, {
      target: { value: 'walrus' },
    });
    fireEvent.click(await screen.findByRole('button', { name: /generate entry/i }));

    await waitFor(() => expect(screen.getByText(/invoke failed \(429\)/i)).toBeTruthy());
  });

  it('ignores postMessage events from a different origin', async () => {
    renderApp();
    await screen.findByText(/no entries yet/i);

    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://attacker.example.com',
        data: { type: 'moses_embed_chat_complete', v: 1, conversationId: 'conv-attacker', finishReason: 'stop' },
      }),
    );

    expect(screen.queryByText(/Moses Manager completed/i)).toBeNull();
  });
});
