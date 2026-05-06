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
