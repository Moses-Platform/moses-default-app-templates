/**
 * MOSES ROUTING: All URLs must be relative (no leading slash).
 * Moses serves each app under a sub-path (e.g. /apps/<app-name>/).
 * A leading slash creates an absolute path that bypasses the sub-path,
 * breaking routing in production. Relative paths let the browser resolve
 * against the current <base href>, which Moses sets per app.
 */

/**
 * Get the base URL for API requests
 * In development: uses Vite proxy
 * In production: uses relative URLs (nginx proxy)
 */
export function getBaseURL(): string {
  return '';
}

export function getAPIURL(path: string): string {
  const base = getBaseURL();
  // Strip leading slashes so the result is always a relative path.
  // See MOSES ROUTING comment at top of file.
  const cleanPath = path.replace(/^\/+/, '');
  return `${base}${cleanPath}`;
}

/**
 * CHAT-pbup: getBasePath returns the BASE_PATH the app is mounted under right
 * now — used as the BrowserRouter basename so internal <Link to="/foo">
 * resolves correctly. The <meta name="moses-base-path"> tag records the
 * canonical Moses mount (/apps/<t>/<a>); the app is reachable both there AND —
 * when an admin assigns a custom hostname — at that hostname's ROOT, so this
 * checks the live URL: under the canonical mount -> the mount; otherwise -> "/".
 * Falls back to "/" for standalone deploys.
 */
let cachedBasePath: string | null = null;
export function getBasePath(): string {
  if (cachedBasePath !== null) return cachedBasePath;
  if (typeof document !== 'undefined') {
    const meta = document.querySelector('meta[name="moses-base-path"]');
    const content = meta?.getAttribute('content') ?? '';
    if (content && content !== '__MOSES_BASE_PATH__') {
      const ensured = content.startsWith('/') ? content : `/${content}`;
      const mount = ensured.length > 1 && ensured.endsWith('/') ? ensured.slice(0, -1) : ensured;
      if (typeof window !== 'undefined' && window.location) {
        const p = window.location.pathname;
        cachedBasePath = p === mount || p.startsWith(`${mount}/`) ? mount : '/';
      } else {
        cachedBasePath = mount;
      }
      return cachedBasePath;
    }
  }
  cachedBasePath = '/';
  return cachedBasePath;
}

/** resetBasePathCache is for tests that need to re-read the meta tag. */
export function resetBasePathCache(): void {
  cachedBasePath = null;
}
