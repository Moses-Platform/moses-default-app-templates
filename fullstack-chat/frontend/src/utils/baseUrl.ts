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
 * CHAT-pbup: getBasePath returns the runtime BASE_PATH the app is mounted
 * under, read from <meta name="moses-base-path">. Used as the BrowserRouter
 * basename so internal <Link to="/foo"> resolves to /apps/<t>/<a>/foo when
 * deployed through Moses, instead of escaping the app and hitting the
 * platform router. Falls back to "/" for standalone deploys.
 */
let cachedBasePath: string | null = null;
export function getBasePath(): string {
  if (cachedBasePath !== null) return cachedBasePath;
  if (typeof document !== 'undefined') {
    const meta = document.querySelector('meta[name="moses-base-path"]');
    const content = meta?.getAttribute('content') ?? '';
    if (content && content !== '__MOSES_BASE_PATH__') {
      const ensured = content.startsWith('/') ? content : `/${content}`;
      const stripped = ensured.length > 1 && ensured.endsWith('/') ? ensured.slice(0, -1) : ensured;
      cachedBasePath = stripped;
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
