/**
 * Moses platform-action invoke wrapper (CHAT-pswm.9) — INTEGRATION
 * REFERENCE code that survives clean_out_template.sh. Import from here
 * instead of touching window.moses directly in components.
 *
 * Transport: the iframe SDK (loaded by index.html from
 * /api/v1/sdk/iframe-sdk.js) installs window.moses.actions.invoke, which
 * POSTs to /__moses/invoke on this app's OWN backend (same-origin under the
 * iframe's nginx subpath). The backend's mosesproxy handler forwards
 * pod-to-pod to moses-backend with the user's JWT preserved — the iframe
 * never contacts moses-backend directly. Ambient typings: types/moses.d.ts.
 */

/**
 * Fire a platform action (chat_prompt or launch_agent) declared in
 * moses-app.config.json → platformActions[].
 *
 * Guards SDK availability: if the SDK script failed to load (offline,
 * gateway misconfig, or the proxy backend is down), `window.moses` is
 * undefined and we surface a clear error instead of a TypeError.
 */
export async function invokeAction(
  actionId: string,
  variables?: Record<string, unknown>,
): Promise<MosesInvokeResult> {
  const invoke = window.moses?.actions?.invoke;
  if (typeof invoke !== 'function') {
    throw new Error(
      'Moses SDK not loaded — /api/v1/sdk/iframe-sdk.js failed to fetch. ' +
        'Is the iframe served via the Moses platform?',
    );
  }
  return invoke(actionId, variables);
}

/**
 * Pull the conversationId out of a chat_prompt invoke result (the
 * dispatcher wraps it as { result: { conversationId } }).
 */
export function conversationIdOf(result: MosesInvokeResult | undefined): string | undefined {
  const id = result?.result?.conversationId;
  return typeof id === 'string' ? id : undefined;
}

/**
 * Shape an invoke rejection into a user-facing message.
 *
 * The SDK reshapes the platform's structured 4xx envelope so `.hint`
 * (e.g. CHAT-mux7 action_not_activated) lands on the error object
 * directly. Prefer hint > status-prefixed message > message so the user
 * sees the actionable remediation rather than a status-code-prefixed
 * JSON blob.
 */
export function shapeInvokeError(err: unknown): string {
  const e = err as { hint?: string; message?: string; status?: number };
  const fallback =
    typeof e?.status === 'number'
      ? `invoke failed (${e.status}): ${e.message ?? 'unknown error'}`
      : e?.message ?? 'invoke failed';
  return e?.hint ?? fallback;
}
