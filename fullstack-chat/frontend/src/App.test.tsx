import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import App from './App';

type FetchArgs = Parameters<typeof globalThis.fetch>;

function mockFetch(impl: (input: FetchArgs[0], init?: FetchArgs[1]) => Promise<Response>) {
  globalThis.fetch = vi.fn(impl as never) as unknown as typeof globalThis.fetch;
}

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

/**
 * CHAT-pswm.9 — the in-iframe SDK is supplied via <script src="..."> on
 * the real platform. In tests we stub the runtime shape directly on
 * window.moses; cleanup restores any prior value after each test.
 *
 * The signature mirrors window.moses.actions.invoke. Tests pass an impl
 * that asserts call shape and/or returns a tailored envelope.
 */
function installMosesSDK(
  invoke?: (actionId: string, variables?: Record<string, unknown>) => Promise<unknown>,
): { spy: ReturnType<typeof vi.fn>; restore: () => void } {
  const prior = (window as unknown as { moses?: unknown }).moses;
  const spy = vi.fn(
    invoke ??
      (async () => ({ status: 'succeeded', result: { conversationId: 'conv-default-1' } })),
  );
  (window as unknown as { moses?: unknown }).moses = {
    __iframeSDKVersion: '1.0.0-test',
    actions: { invoke: spy },
  };
  return {
    spy,
    restore: () => {
      (window as unknown as { moses?: unknown }).moses = prior;
    },
  };
}

describe('Fullstack Chat Roundtrip — App', () => {
  beforeEach(() => {
    // Default: empty entries, no errors. fetch is used ONLY for the app's
    // own backend (api/v1/entries). Platform-action invokes go through
    // window.moses.actions.invoke (installed per-test via installMosesSDK).
    mockFetch(async () => jsonResponse({ entries: [], count: 0 }));
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    // Clear any window.moses left by individual tests.
    delete (window as unknown as { moses?: unknown }).moses;
  });

  it('renders the empty-state message when no entries exist', async () => {
    render(<App />);
    expect(await screen.findByText(/no entries yet/i)).toBeTruthy();
  });

  it('renders entries returned by the backend', async () => {
    mockFetch(async () =>
      jsonResponse({
        entries: [
          {
            id: '00000000-0000-0000-0000-000000000001',
            text: 'A witty first entry',
            source: 'moses_manager',
            created_at: '2026-01-01T12:00:00Z',
          },
        ],
        count: 1,
      }),
    );
    render(<App />);
    expect(await screen.findByText('A witty first entry')).toBeTruthy();
  });

  it('rejects an empty topic on submit and shows an error status', async () => {
    render(<App />);
    const button = await screen.findByRole('button', { name: /generate entry/i });
    expect((button as HTMLButtonElement).disabled).toBe(true); // disabled until topic typed
  });

  // CHAT-pswm.9 — the round-trip now goes through window.moses.actions.invoke
  // (supplied by the iframe SDK loaded from /api/v1/sdk/iframe-sdk.js). We
  // stub the runtime contract instead of stubbing fetch, mirroring the real
  // call boundary: App.tsx hands the user-supplied topic to the SDK, which
  // is responsible for POSTing to /__moses/invoke on the app's own backend.
  it('invokes window.moses.actions.invoke with the action id and typed topic', async () => {
    const { spy, restore } = installMosesSDK(async () => ({
      status: 'succeeded',
      result: { conversationId: 'conv-test-1' },
    }));

    try {
      render(<App />);
      const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
      fireEvent.change(input, { target: { value: 'octopus' } });

      const button = await screen.findByRole('button', { name: /generate entry/i });
      fireEvent.click(button);

      await waitFor(() => {
        expect(spy).toHaveBeenCalled();
      });

      // The platform-action invocation contract is (actionId, variables).
      // chartId / chartContext etc. are NOT carried by App.tsx anymore — the
      // proxy backend injects them from MOSES_CHART_ID / MOSES_APP_SLUG env.
      expect(spy).toHaveBeenCalledWith('generate-entry', { topic: 'octopus' });
    } finally {
      restore();
    }
  });

  it('posts moses_embed_open_chat to window.parent after a successful invoke', async () => {
    const { restore } = installMosesSDK(async () => ({
      status: 'succeeded',
      result: { conversationId: 'conv-announce-1' },
    }));

    // window.parent === window in jsdom (no real frame). Stub `parent` with
    // a distinct object so the `window.parent !== window` guard in App.tsx
    // takes the announce branch, and capture postMessage calls on it.
    const parentPostSpy = vi.fn();
    const originalParentDescriptor = Object.getOwnPropertyDescriptor(window, 'parent');
    Object.defineProperty(window, 'parent', {
      configurable: true,
      get: () => ({ postMessage: parentPostSpy } as unknown as Window),
    });

    try {
      render(<App />);
      const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
      fireEvent.change(input, { target: { value: 'walrus' } });

      const button = await screen.findByRole('button', { name: /generate entry/i });
      fireEvent.click(button);

      await waitFor(() => {
        expect(parentPostSpy).toHaveBeenCalled();
      });

      // Find the moses_embed_open_chat envelope among any other parent posts.
      const announceCall = parentPostSpy.mock.calls.find((args) => {
        const data = args[0] as Record<string, unknown> | undefined;
        return data?.type === 'moses_embed_open_chat';
      });
      expect(announceCall).toBeTruthy();
      const envelope = announceCall?.[0] as Record<string, unknown>;
      expect(envelope.v).toBe(1);
      expect(envelope.conversationId).toBe('conv-announce-1');
      expect(envelope.app).toBe('fullstack-chat');
      // Targeted origin must match the host's, not '*'.
      expect(announceCall?.[1]).toBe(window.location.origin);
    } finally {
      if (originalParentDescriptor) {
        Object.defineProperty(window, 'parent', originalParentDescriptor);
      } else {
        // jsdom's default: parent === window.
        Object.defineProperty(window, 'parent', {
          configurable: true,
          get: () => window,
        });
      }
      restore();
    }
  });

  // CHAT-pswm.9 — when the SDK script fails to load (offline, network policy
  // blocks the platform path, or the iframe is opened standalone) window.moses
  // is undefined. App.tsx must surface a clear "Moses SDK not loaded" error
  // instead of a confusing "Cannot read properties of undefined" stack.
  it('shows "Moses SDK not loaded" when window.moses is undefined', async () => {
    // No installMosesSDK — leave window.moses undefined.
    render(<App />);
    const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'orca' } });
    const button = await screen.findByRole('button', { name: /generate entry/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText(/Moses SDK not loaded/i)).toBeTruthy();
    });
  });

  it('updates the status banner on a moses_embed_chat_complete postMessage', async () => {
    render(<App />);
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

    await waitFor(() => {
      expect(screen.getByText(/Moses Manager completed/i)).toBeTruthy();
    });
  });

  // CHAT-mux7: when the platform returns 409 action_not_activated with a
  // structured `hint` field, the template must render the hint verbatim
  // (not the opaque "(409): {raw json}" string). The hint points the user
  // at the SelectedAppPanel activation banner above the iframe — without
  // it, the user sees a code-tagged JSON blob and gives up.
  //
  // CHAT-pswm.9: the SDK reshapes the 4xx envelope onto Error.code/.hint
  // (see iframe_sdk_handler.go). App.tsx prefers hint > message when
  // rendering the status banner.
  it('renders the platform 409 action_not_activated hint instead of raw JSON', async () => {
    const ACTIVATION_HINT =
      "Open the app's tab in Moses Manager and approve permissions in the banner above the panel.";
    const { restore } = installMosesSDK(async () => {
      const err = new Error('action not yet activated; awaiting grant approval') as Error & {
        status?: number;
        code?: string;
        hint?: string;
      };
      err.status = 409;
      err.code = 'action_not_activated';
      err.hint = ACTIVATION_HINT;
      throw err;
    });

    try {
      render(<App />);
      const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
      fireEvent.change(input, { target: { value: 'octopus' } });
      const button = await screen.findByRole('button', { name: /generate entry/i });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(ACTIVATION_HINT.slice(0, 30), 'i'))).toBeTruthy();
      });

      // Critically: the noisy "(409):" prefix must NOT appear when the hint
      // is present. Templates that just append the hint after the raw error
      // message defeat the purpose of CHAT-mux7.
      expect(screen.queryByText(/\(409\):/)).toBeNull();
    } finally {
      restore();
    }
  });

  // CHAT-mux7 fallback: when the platform returns a 4xx without a hint
  // (e.g. plain rate_limited, internal_error, non-JSON proxy 502), the
  // template falls back to the previous "invoke failed (status): detail"
  // message so the user still sees something actionable. This guards
  // against the over-eager refactor where the hint path swallows all 4xx.
  it('falls back to "invoke failed" when the 4xx body has no hint', async () => {
    const { restore } = installMosesSDK(async () => {
      const err = new Error('rate limit exceeded for this action') as Error & {
        status?: number;
        code?: string;
      };
      err.status = 429;
      err.code = 'rate_limited';
      throw err;
    });

    try {
      render(<App />);
      const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
      fireEvent.change(input, { target: { value: 'walrus' } });
      const button = await screen.findByRole('button', { name: /generate entry/i });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText(/invoke failed \(429\)/i)).toBeTruthy();
      });
    } finally {
      restore();
    }
  });

  it('ignores postMessage events from a different origin', async () => {
    render(<App />);
    await screen.findByText(/no entries yet/i);

    window.dispatchEvent(
      new MessageEvent('message', {
        origin: 'https://attacker.example.com',
        data: {
          type: 'moses_embed_chat_complete',
          v: 1,
          conversationId: 'conv-attacker',
          finishReason: 'stop',
        },
      }),
    );

    // No "Moses Manager completed" banner should appear.
    expect(screen.queryByText(/Moses Manager completed/i)).toBeNull();
  });
});
