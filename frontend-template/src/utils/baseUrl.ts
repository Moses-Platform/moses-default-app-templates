/**
 * Get the base URL for the application.
 *
 * Moses sets BASE_URL env var for subpath deployment.
 * After ingress strip-prefix, app sees '/' as root.
 * This utility exists for constructing absolute/sharing URLs.
 */
export function getBaseUrl(): string {
  // Check for Moses-injected base URL
  if (typeof window !== 'undefined' && (window as any).__MOSES_BASE_URL__) {
    return (window as any).__MOSES_BASE_URL__;
  }

  // Fallback to Vite env var or root
  return import.meta.env.VITE_BASE_URL || '/';
}
