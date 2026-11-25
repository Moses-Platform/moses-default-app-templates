/**
 * Host-shell postMessage contract (CHAT-rg5t / CHAT-l7va) — INTEGRATION
 * REFERENCE code, platform plumbing.
 *
 * Two envelopes flow between the embedded iframe and the Moses host shell:
 *
 *   - `moses_embed_open_chat` (v1, iframe → host). The app posts it to the
 *     Moses window after a successful chat_prompt invoke so the host can
 *     (a) open the global chat sidebar pinned to the new conversation, and
 *     (b) register the conversationId so it can later forward
 *     `auto_response_ready` WS events back here.
 *   - `moses_embed_chat_complete` (v1, host → iframe). Fired when the AI
 *     turn finishes; consumed via the origin-checked hook below.
 *
 * SECURITY: the Moses window is addressed by its origin and only messages
 * from that origin are accepted. Apps may run on their own apps origin, so
 * that origin is NOT this page's: it comes from `window.MosesSDK.hostOrigin`
 * (set by /api/v1/sdk/iframe-sdk.js). On an older platform that serves apps
 * on the Moses origin, a parent we can read is same-origin and is used.
 * Otherwise (app opened in its own tab, SDK missing) the bridge is a no-op;
 * the query refetch backstop reconciles. Never '*'.
 */
import { useEffect } from 'react';

const BARE_ORIGIN = /^https?:\/\/[^/]+$/;

/** The Moses window's origin, or null when it is unknown. */
function hostOrigin(): string | null {
  const o = (window as { MosesSDK?: { hostOrigin?: unknown } }).MosesSDK?.hostOrigin;
  if (typeof o === 'string' && BARE_ORIGIN.test(o)) return o;
  // Older platform (same origin): the parent IS this origin.
  try {
    if (window.parent !== window && window.parent.location.origin === window.location.origin) {
      return window.location.origin;
    }
  } catch {
    /* cross-origin parent */
  }
  return null;
}

export type CompletionMessage = {
  type: 'moses_embed_chat_complete';
  v: 1;
  conversationId: string;
  preview?: string;
  finishReason?: 'stop' | 'length' | 'error' | 'credential_unset';
};

/**
 * Subscribe to `moses_embed_chat_complete` messages from the host shell.
 * This is a push channel, not a data fetch — it stays imperative on
 * purpose (FRONTEND_DATA_LAYER.md); invalidate your TanStack Query keys
 * inside onComplete to reconcile server state immediately.
 */
export function useMosesChatComplete(onComplete: (msg: CompletionMessage) => void): void {
  useEffect(() => {
    function onMessage(e: MessageEvent) {
      if (e.origin !== hostOrigin()) return;
      const data = e.data;
      if (typeof data !== 'object' || data === null) return;

      if (data.type === 'moses_embed_chat_complete') {
        const msg = data as CompletionMessage;
        if (msg.v !== 1 || typeof msg.conversationId !== 'string') return;
        onComplete(msg);
      }
      // moses_embed_open_chat is host-shell-bound (the host opens its own
      // sidebar). The app neither sends nor consumes it here; it is posted
      // UP via announceOpenChat below.
    }
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [onComplete]);
}

/**
 * Tell the host shell about a freshly created conversation so it (a) opens
 * the global chat sidebar pinned to this conversationId, and (b) forwards
 * the eventual `auto_response_ready` WS event back to us as a
 * `moses_embed_chat_complete` postMessage. Without this announce step the
 * host has no record of which conversations belong to this iframe.
 *
 * No-op outside an iframe or when the Moses origin is unknown. Never throws
 * (the caller's query refetch backstop still reconciles).
 */
export function announceOpenChat(conversationId: string, appSlug: string): void {
  if (!conversationId || !window.parent || window.parent === window) return;
  const target = hostOrigin();
  if (!target) return;
  try {
    window.parent.postMessage(
      {
        type: 'moses_embed_open_chat',
        v: 1,
        conversationId,
        app: appSlug,
      },
      target,
    );
  } catch (err) {
    console.warn('[moses] moses_embed_open_chat post failed', err);
  }
}
