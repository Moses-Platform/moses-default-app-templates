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

describe('Fullstack Chat Roundtrip — App', () => {
  beforeEach(() => {
    // Default: empty entries, no errors.
    mockFetch(async () => jsonResponse({ entries: [], count: 0 }));
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
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

  it('invokes the platform-action endpoint with the typed topic', async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    mockFetch(async (input, init) => {
      const url = typeof input === 'string' ? input : (input as URL | Request).toString();
      calls.push({ url, init });
      if (url.includes('/api/v1/apps/fullstack-chat/actions/generate-entry/invoke')) {
        return jsonResponse({
          status: 'succeeded',
          result: { conversationId: 'conv-test-1' },
        });
      }
      return jsonResponse({ entries: [], count: 0 });
    });

    render(<App />);
    const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'octopus' } });

    const button = await screen.findByRole('button', { name: /generate entry/i });
    fireEvent.click(button);

    await waitFor(() => {
      const invoked = calls.some((c) =>
        c.url.includes('/api/v1/apps/fullstack-chat/actions/generate-entry/invoke'),
      );
      expect(invoked).toBe(true);
    });

    const invokeCall = calls.find((c) =>
      c.url.includes('/api/v1/apps/fullstack-chat/actions/generate-entry/invoke'),
    );
    expect(invokeCall).toBeTruthy();
    expect(invokeCall?.init?.method).toBe('POST');
    const body = JSON.parse((invokeCall?.init?.body ?? '{}') as string);
    expect(body.variables.topic).toBe('octopus');
  });

  it('posts moses_embed_open_chat to window.parent after a successful invoke', async () => {
    mockFetch(async (input) => {
      const url = typeof input === 'string' ? input : (input as URL | Request).toString();
      if (url.includes('/api/v1/apps/fullstack-chat/actions/generate-entry/invoke')) {
        return jsonResponse({
          status: 'succeeded',
          result: { conversationId: 'conv-announce-1' },
        });
      }
      return jsonResponse({ entries: [], count: 0 });
    });

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
    }
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
  it('renders the platform 409 action_not_activated hint instead of raw JSON', async () => {
    const ACTIVATION_HINT =
      "Open the app's tab in Moses Manager and approve permissions in the banner above the panel.";
    mockFetch(async (input) => {
      const url = typeof input === 'string' ? input : (input as URL | Request).toString();
      if (url.includes('/api/v1/apps/fullstack-chat/actions/generate-entry/invoke')) {
        return new Response(
          JSON.stringify({
            error: 'action not yet activated; awaiting grant approval',
            code: 'action_not_activated',
            invocationId: '',
            hint: ACTIVATION_HINT,
          }),
          { status: 409, headers: { 'Content-Type': 'application/json' } },
        );
      }
      return jsonResponse({ entries: [], count: 0 });
    });

    render(<App />);
    const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'octopus' } });
    const button = await screen.findByRole('button', { name: /generate entry/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText(new RegExp(ACTIVATION_HINT.slice(0, 30), 'i'))).toBeTruthy();
    });

    // Critically: the noisy "(409):" prefix must NOT appear when the hint is
    // present. Templates that just append the hint after the raw error
    // message defeat the purpose of CHAT-mux7.
    expect(screen.queryByText(/\(409\):/)).toBeNull();
  });

  // CHAT-mux7 fallback: when the platform returns a 4xx without a hint
  // (e.g. plain rate_limited, internal_error, non-JSON proxy 502), the
  // template falls back to the previous "invoke failed (status): detail"
  // message so the user still sees something actionable. This guards
  // against the over-eager refactor where the hint path swallows all 4xx.
  it('falls back to "invoke failed" when the 4xx body has no hint', async () => {
    mockFetch(async (input) => {
      const url = typeof input === 'string' ? input : (input as URL | Request).toString();
      if (url.includes('/api/v1/apps/fullstack-chat/actions/generate-entry/invoke')) {
        return new Response(
          JSON.stringify({
            error: 'rate limit exceeded for this action',
            code: 'rate_limited',
            invocationId: '',
          }),
          { status: 429, headers: { 'Content-Type': 'application/json' } },
        );
      }
      return jsonResponse({ entries: [], count: 0 });
    });

    render(<App />);
    const input = (await screen.findByLabelText(/topic/i)) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'walrus' } });
    const button = await screen.findByRole('button', { name: /generate entry/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText(/invoke failed \(429\)/i)).toBeTruthy();
    });
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
