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
