// CHAT-pbup: shared Moses-aware BASE_PATH helper.
//
// The platform's deploy pipeline renders the runtime BASE_PATH into a meta
// tag in index.html (`<meta name="moses-base-path" content="/apps/<t>/<a>/">`)
// at container start via the template's entrypoint.sh. Frontend templates
// import this helper so React Router's BrowserRouter `basename` agrees
// with the public URL the user sees.
//
// Standalone deploys leave the placeholder unrendered, so the helper
// falls back to "/" — preserving pre-CHAT-pbup behavior for legacy
// templates that don't ship the entrypoint substitution.
//
// Usage (preferred):
//   import { getBasePath } from '@moses/shared/base-path/getBasePath'
//   <BrowserRouter basename={getBasePath()}>
//
// This file is the canonical implementation; per-template src/utils/baseUrl.ts
// should re-export from here once the template's tooling can resolve the
// relative path. The first wave of templates inlines an equivalent
// implementation to keep the diff minimal — see frontend-template/src/utils/baseUrl.ts.

const META_NAME = 'moses-base-path';
const PLACEHOLDER = '__MOSES_BASE_PATH__';

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
  cached = '/';
  return cached;
}

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
