/**
 * Moses BASE_PATH helper.
 *
 * CHAT-pbup contract: when the platform deploys this template, an
 * entrypoint.sh script renders the runtime BASE_PATH into a meta tag in
 * index.html (`<meta name="moses-base-path" content="/apps/<t>/<a>/">`).
 * This helper reads that meta tag at runtime so the React Router basename
 * + any link-href construction agree with the public URL the user sees.
 *
 * Standalone deploys (no platform) leave the placeholder unrendered, so
 * the helper falls back to "/" — same behavior as before this epic.
 *
 * Vite's `base: './'` is unaffected: hashed asset filenames in the built
 * HTML use relative URLs and resolve against the current page URL.
 */

const META_NAME = 'moses-base-path';
const PLACEHOLDER = '__MOSES_BASE_PATH__';

/**
 * getBasePath returns the runtime BASE_PATH the app is mounted under.
 * Returns "/" when standalone or when the placeholder hasn't been rendered.
 *
 * Cached after the first read because the meta tag is fixed at startup.
 */
let cached: string | null = null;
export function getBasePath(): string {
  if (cached !== null) return cached;
  if (typeof document !== 'undefined') {
    const meta = document.querySelector(`meta[name="${META_NAME}"]`);
    const content = meta?.getAttribute('content') ?? '';
    if (content && content !== PLACEHOLDER) {
      cached = ensureLeadingSlash(stripTrailingSlash(content));
      return cached;
    }
  }
  // Legacy fallback for tests + window.__MOSES_BASE_URL__ injection.
  if (typeof window !== 'undefined') {
    const w = window as unknown as { __MOSES_BASE_URL__?: string };
    if (w.__MOSES_BASE_URL__) {
      cached = ensureLeadingSlash(stripTrailingSlash(w.__MOSES_BASE_URL__));
      return cached;
    }
  }
  // Vite env var fallback for build-time injection (rare; runtime preferred).
  // Conversion through `unknown` because ImportMetaEnv doesn't list VITE_BASE_URL by default.
  const importMetaAny = import.meta as unknown as { env?: { VITE_BASE_URL?: string } };
  if (typeof import.meta !== 'undefined' && importMetaAny.env?.VITE_BASE_URL) {
    cached = ensureLeadingSlash(stripTrailingSlash(importMetaAny.env.VITE_BASE_URL));
    return cached;
  }
  cached = '/';
  return cached;
}

/**
 * getBaseUrl is the legacy alias for getBasePath. Kept for backwards
 * compatibility with templates / consumers that import the old name.
 * @deprecated Use getBasePath instead.
 */
export function getBaseUrl(): string {
  return getBasePath();
}

/**
 * resetBasePathCache is for tests that need to re-read the meta tag.
 */
export function resetBasePathCache(): void {
  cached = null;
}

function ensureLeadingSlash(s: string): string {
  return s.startsWith('/') ? s : `/${s}`;
}

function stripTrailingSlash(s: string): string {
  if (s.length > 1 && s.endsWith('/')) {
    return s.slice(0, -1);
  }
  return s;
}
