/**
 * Host-shell postMessage contract (CHAT-rg5t / CHAT-l7va) — INTEGRATION
 * REFERENCE code that survives clean_out_template.sh.
 *
 * Two envelopes flow between the embedded iframe and the Moses host shell:
 *
 *   - `moses_embed_open_chat` (v1, iframe → host). The app posts it to
 *     window.parent after a successful chat_prompt invoke so the host can
 *     (a) open the global chat sidebar pinned to the new conversation, and
 *     (b) register the conversationId so it can later forward
 *     `auto_response_ready` WS events back here.
 *   - `moses_embed_chat_complete` (v1, host → iframe). Fired when the AI
 *     turn finishes; consumed via the origin-checked hook below.
 *
 * SECURITY: always origin-check incoming messages and post with an explicit
 * targetOrigin. The host iframe shell deploys apps under a Moses subpath,
 * so iframe and host are same-origin; cross-origin custom-domain apps need
 * an origin allowlist (future work).
 */
import { useEffect } from 'react';

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
      if (e.origin !== window.location.origin) return;
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
 * No-op outside an iframe. Never throws (cross-origin posts throw — the
 * caller's query refetch backstop still reconciles).
 */
export function announceOpenChat(conversationId: string, appSlug: string): void {
  if (!conversationId || !window.parent || window.parent === window) return;
  try {
    window.parent.postMessage(
      {
        type: 'moses_embed_open_chat',
        v: 1,
        conversationId,
        app: appSlug,
      },
      window.location.origin, // targeted origin, never '*'
    );
  } catch (err) {
    console.warn('[moses] moses_embed_open_chat post failed', err);
  }
}
