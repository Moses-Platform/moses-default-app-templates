/**
 * Integration-reference tests for src/moses/invoke.ts (the SDK wrapper; the
 * host-shell postMessage bridge is tested in hostMessages.test.ts). These test
 * the modules DIRECTLY — no UI involved —
 * so they keep guarding the contract whatever UI you build on top. Add your
 * own component tests alongside your components.
 */
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
import { conversationIdOf, invokeAction, shapeInvokeError } from './invoke';

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
