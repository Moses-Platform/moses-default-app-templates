// Moses iframe SDK ambient typings.
//
// The SDK is fetched at boot from /api/v1/sdk/iframe-sdk.js (served by
// moses-backend, CHAT-pswm.2). It installs window.moses.actions.invoke
// which POSTs to /__moses/invoke on this app's own backend. The proxy
// (mosesproxy-go, CHAT-pswm.8) forwards pod-to-pod to moses-backend
// with the user's JWT preserved.
//
// These types reflect the SDK's runtime surface so App.tsx can call
// window.moses.actions.invoke without an "any" cast. Keep in sync with
// iframe_sdk_handler.go on the platform side.

export {};

declare global {
  interface MosesInvokeResult {
    /** Wrapped result envelope from the dispatcher (e.g. { conversationId }). */
    result?: Record<string, unknown>;
    /** Per-call invocation UUID for cross-system tracing. */
    invocationId?: string;
    /** Dispatcher status string (e.g. "succeeded"). */
    status?: string;
  }

  /**
   * Error thrown by window.moses.actions.invoke on a non-2xx response.
   * The SDK reshapes the structured 4xx envelope into Error.code / hint
   * so the UI can render an actionable message instead of raw JSON.
   */
  interface MosesInvokeError extends Error {
    /** HTTP status from the proxy (or upstream moses-backend) hop. */
    status?: number;
    /** Stable error code, e.g. "rate_limited", "action_not_activated". */
    code?: string;
    /** Human-readable remediation hint when the platform provides one. */
    hint?: string;
    /** Server-suggested retry delay in seconds (rate-limit responses). */
    retryAfterSeconds?: number;
  }

  interface Window {
    moses?: {
      /** Bumps on every SDK API-surface change. Feature-detect against this. */
      __iframeSDKVersion?: string;
      actions?: {
        /**
         * Invoke a platform action (chat_prompt or launch_agent). Returns
         * the dispatcher's envelope. Rejects with a MosesInvokeError on
         * any non-2xx, with .code / .hint reshaped from the JSON body.
         */
        invoke?: (
          actionId: string,
          variables?: Record<string, unknown>,
        ) => Promise<MosesInvokeResult>;
      };
    };
  }
}
