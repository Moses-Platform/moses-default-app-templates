/**
 * Integration-reference tests for src/moses/ (invoke wrapper + host-shell
 * postMessage contract). These test the modules DIRECTLY — no UI involved —
 * so they keep guarding the contract whatever UI you build on top. Add your
 * own component tests alongside your components.
 */
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { conversationIdOf, invokeAction, shapeInvokeError } from './invoke';
import { announceOpenChat, useMosesChatComplete } from './hostMessages';

/**
 * CHAT-pswm.9 — the in-iframe SDK is supplied via <script src="..."> on the
 * real platform. In tests we stub the runtime shape directly on window.moses.
 */
function installMosesSDK(
  invoke?: (actionId: string, variables?: Record<string, unknown>) => Promise<MosesInvokeResult>,
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

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  delete (window as unknown as { moses?: unknown }).moses;
});

describe('moses/invoke — SDK wrapper', () => {
  it('throws a clear "Moses SDK not loaded" error when window.moses is undefined', async () => {
    await expect(invokeAction('generate-entry', { topic: 'orca' })).rejects.toThrow(
      /Moses SDK not loaded/i,
    );
  });

  it('delegates to window.moses.actions.invoke with (actionId, variables)', async () => {
    const spy = installMosesSDK(async () => ({
      status: 'succeeded',
      result: { conversationId: 'conv-test-1' },
    }));
    const result = await invokeAction('generate-entry', { topic: 'octopus' });
    expect(spy).toHaveBeenCalledWith('generate-entry', { topic: 'octopus' });
    expect(conversationIdOf(result)).toBe('conv-test-1');
  });

  it('conversationIdOf returns undefined for missing/non-string ids', () => {
    expect(conversationIdOf(undefined)).toBeUndefined();
    expect(conversationIdOf({})).toBeUndefined();
    expect(conversationIdOf({ result: { conversationId: 42 } })).toBeUndefined();
  });
});

describe('moses/invoke — error shaping (CHAT-mux7)', () => {
  it('prefers the structured hint over the raw status/JSON message', () => {
    const HINT =
      "Open the app's tab in Moses Manager and approve permissions in the banner above the panel.";
    const err = new Error('action not yet activated; awaiting grant approval') as Error & {
      status?: number;
      hint?: string;
    };
    err.status = 409;
    err.hint = HINT;
    expect(shapeInvokeError(err)).toBe(HINT);
  });

  it('falls back to "invoke failed (status): detail" when the 4xx has no hint', () => {
    const err = new Error('rate limit exceeded for this action') as Error & { status?: number };
    err.status = 429;
    expect(shapeInvokeError(err)).toBe('invoke failed (429): rate limit exceeded for this action');
  });

  it('uses the plain message when there is no status', () => {
    expect(shapeInvokeError(new Error('boom'))).toBe('boom');
    expect(shapeInvokeError(undefined)).toBe('invoke failed');
  });
});

describe('moses/hostMessages — useMosesChatComplete', () => {
  it('delivers a valid moses_embed_chat_complete message', async () => {
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

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

    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onComplete.mock.calls[0][0].conversationId).toBe('conv-finished-1');
  });

  it('ignores messages from a different origin', async () => {
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

    await act(async () => {
      window.dispatchEvent(
        new MessageEvent('message', {
          origin: 'https://attacker.example.com',
          data: { type: 'moses_embed_chat_complete', v: 1, conversationId: 'conv-attacker' },
        }),
      );
    });

    expect(onComplete).not.toHaveBeenCalled();
  });

  it('ignores unrelated message types and non-v1 envelopes', async () => {
    const onComplete = vi.fn();
    renderHook(() => useMosesChatComplete(onComplete));

    await act(async () => {
      window.dispatchEvent(
        new MessageEvent('message', {
          origin: window.location.origin,
          data: { type: 'something_else', v: 1, conversationId: 'x' },
        }),
      );
      window.dispatchEvent(
        new MessageEvent('message', {
          origin: window.location.origin,
          data: { type: 'moses_embed_chat_complete', v: 2, conversationId: 'x' },
        }),
      );
    });

    expect(onComplete).not.toHaveBeenCalled();
  });
});

describe('moses/hostMessages — announceOpenChat', () => {
  function stubParent(postMessage: ReturnType<typeof vi.fn>): () => void {
    const original = Object.getOwnPropertyDescriptor(window, 'parent');
    Object.defineProperty(window, 'parent', {
      configurable: true,
      get: () => ({ postMessage } as unknown as Window),
    });
    return () => {
      if (original) Object.defineProperty(window, 'parent', original);
      else Object.defineProperty(window, 'parent', { configurable: true, get: () => window });
    };
  }

  it('posts the v1 moses_embed_open_chat envelope to the targeted origin (never "*")', () => {
    const postSpy = vi.fn();
    const restore = stubParent(postSpy);
    try {
      announceOpenChat('conv-announce-1', 'fullstack-chat');
      expect(postSpy).toHaveBeenCalledTimes(1);
      const [envelope, targetOrigin] = postSpy.mock.calls[0];
      expect(envelope).toEqual({
        type: 'moses_embed_open_chat',
        v: 1,
        conversationId: 'conv-announce-1',
        app: 'fullstack-chat',
      });
      expect(targetOrigin).toBe(window.location.origin);
    } finally {
      restore();
    }
  });

  it('swallows cross-origin postMessage throws (non-fatal by contract)', () => {
    const postSpy = vi.fn(() => {
      throw new DOMException('blocked');
    });
    const restore = stubParent(postSpy);
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    try {
      expect(() => announceOpenChat('conv-1', 'fullstack-chat')).not.toThrow();
      expect(warn).toHaveBeenCalled();
    } finally {
      restore();
    }
  });

  it('is a no-op when not embedded (window.parent === window)', () => {
    // jsdom default: window.parent === window. Nothing should throw.
    expect(() => announceOpenChat('conv-1', 'fullstack-chat')).not.toThrow();
  });
});
