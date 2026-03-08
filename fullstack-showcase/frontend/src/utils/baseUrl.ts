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
  const cleanPath = path.startsWith('/') ? path : `/${path}`;
  return `${base}${cleanPath}`;
}
