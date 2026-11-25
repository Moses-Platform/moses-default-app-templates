/**
 * Integration-reference tests for the host-shell postMessage bridge
 * (src/moses/hostMessages.ts). The Moses window may be another origin than
 * this app (apps origin): the bridge talks to `window.MosesSDK.hostOrigin`
 * (set by /api/v1/sdk/iframe-sdk.js once the platform serves apps on their own
 * origin), and to its own origin only when the
 * parent is provably same-origin (an older platform serving apps on the
 * Moses origin).
 */
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { announceOpenChat, useMosesChatComplete } from './hostMessages';

const MOSES = 'https://moses.example.com';
const OPEN_CHAT = { type: 'moses_embed_open_chat', v: 1, conversationId: 'conv-1', app: 'fullstack-chat' };
const COMPLETE = { type: 'moses_embed_chat_complete', v: 1, conversationId: 'conv-done', finishReason: 'stop' };

type Win = Window & { MosesSDK?: unknown };

function setHostOrigin(hostOrigin: unknown): void {
  (window as Win).MosesSDK = { hostOrigin };
}

let restoreParent: (() => void) | null = null;

/** Embed the app: `sameOrigin` false makes parent.location throw, as a browser does cross-origin. */
function stubParent(postMessage: ReturnType<typeof vi.fn>, sameOrigin: boolean): void {
  const original = Object.getOwnPropertyDescriptor(window, 'parent');
  const parent = {
    postMessage,
    get location() {
      if (!sameOrigin) throw new DOMException('Blocked a frame with origin', 'SecurityError');
      return { origin: window.location.origin };
    },
  };
  Object.defineProperty(window, 'parent', { configurable: true, get: () => parent as unknown as Window });
  restoreParent = () => {
    if (original) Object.defineProperty(window, 'parent', original);
  };
}

async function deliver(origin: string, data: unknown): Promise<void> {
  await act(async () => {
    window.dispatchEvent(new MessageEvent('message', { origin, data }));
  });
}

afterEach(() => {
  cleanup();
  restoreParent?.();
  restoreParent = null;
  delete (window as Win).MosesSDK;
  vi.restoreAllMocks();
});

describe('announceOpenChat', () => {
  it('posts to MosesSDK.hostOrigin', () => {
    const post = vi.fn();
    stubParent(post, false);
    setHostOrigin(MOSES);

    announceOpenChat('conv-1', 'fullstack-chat');

    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith(OPEN_CHAT, MOSES);
  });

  it('no-op without hostOrigin and a cross-origin parent (no throw)', () => {
    const post = vi.fn();
    stubParent(post, false);

    expect(() => announceOpenChat('conv-1', 'fullstack-chat')).not.toThrow();
    expect(post).not.toHaveBeenCalled();
  });

  it('same-origin parent (old platform) still works', () => {
    const post = vi.fn();
    stubParent(post, true);

    announceOpenChat('conv-1', 'fullstack-chat');

    expect(post).toHaveBeenCalledWith(OPEN_CHAT, window.location.origin);
  });

  it.each(['http://localhost/', 'http://localhost/x', 'javascript:alert(1)', '*', 42])(
    'rejects hostOrigin %s (not a bare origin)',
    (bad) => {
      const post = vi.fn();
      stubParent(post, false);
      setHostOrigin(bad);

      announceOpenChat('conv-1', 'fullstack-chat');

      expect(post).not.toHaveBeenCalled();
    },
  );

  it('is a no-op when not embedded (window.parent === window)', () => {
    setHostOrigin(MOSES);
    expect(() => announceOpenChat('conv-1', 'fullstack-chat')).not.toThrow();
  });

  it('swallows a throwing postMessage (non-fatal by contract)', () => {
    const post = vi.fn(() => {
      throw new DOMException('blocked');
    });
    stubParent(post, false);
    setHostOrigin(MOSES);
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    expect(() => announceOpenChat('conv-1', 'fullstack-chat')).not.toThrow();
    expect(warn).toHaveBeenCalled();
  });
});

describe('useMosesChatComplete', () => {
  it('accepts completion from hostOrigin', async () => {
    stubParent(vi.fn(), false);
    setHostOrigin(MOSES);
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

    await deliver(MOSES, COMPLETE);

    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onComplete.mock.calls[0][0].conversationId).toBe('conv-done');
  });

  it('ignores messages from its own origin when hostOrigin differs', async () => {
    stubParent(vi.fn(), false);
    setHostOrigin(MOSES);
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

    await deliver(window.location.origin, COMPLETE);
    await deliver('https://attacker.example.com', COMPLETE);

    expect(onComplete).not.toHaveBeenCalled();
  });

  it('ignores everything without hostOrigin and a cross-origin parent', async () => {
    stubParent(vi.fn(), false);
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

    await deliver(window.location.origin, COMPLETE);
    await deliver(MOSES, COMPLETE);

    expect(onComplete).not.toHaveBeenCalled();
  });

  it('same-origin parent (old platform) still delivers', async () => {
    stubParent(vi.fn(), true);
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

    await deliver(window.location.origin, COMPLETE);

    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it('ignores unrelated message types and non-v1 envelopes', async () => {
    stubParent(vi.fn(), false);
    setHostOrigin(MOSES);
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

    await deliver(MOSES, { type: 'something_else', v: 1, conversationId: 'x' });
    await deliver(MOSES, { ...COMPLETE, v: 2 });

    expect(onComplete).not.toHaveBeenCalled();
  });
});

describe('no wildcard postMessage targets', () => {
  it("no postMessage( in src uses '*'", () => {
    const sources = import.meta.glob(['../**/*.{ts,tsx}', '!../**/*.test.{ts,tsx}'], {
      query: '?raw',
      import: 'default',
      eager: true,
    }) as Record<string, string>;
    expect(Object.keys(sources).length).toBeGreaterThan(0);
    for (const [file, raw] of Object.entries(sources)) {
      const code = raw.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '');
      const calls = code.match(/postMessage\([\s\S]*?\);/g) ?? [];
      for (const call of calls) {
        expect(call, file).not.toMatch(/['"`]\*['"`]/);
      }
    }
  });
});
