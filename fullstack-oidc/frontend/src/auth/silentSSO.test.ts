import { attemptSilentSSO } from './silentSSO';
import { resetBasePathCache } from '../utils/baseUrl';

// silentSSO's URL builders are private; we exercise them indirectly via
// attemptSilentSSO, which appends a hidden <iframe> whose `src` is the
// /auth/silent-check URL. In jsdom the iframe never fires `load`, so the
// probe resolves on its timeout — we use a short timeout and assert on
// the iframe `src` that was written.

// clearNode removes all children of a node without touching innerHTML.
function clearNode(node: Node): void {
  while (node.firstChild) node.removeChild(node.firstChild);
}

describe('attemptSilentSSO', () => {
  beforeEach(() => {
    resetBasePathCache();
    clearNode(document.head);
    clearNode(document.body);
  });

  it('appends a hidden iframe pointing at a relative /auth/silent-check URL', async () => {
    // No moses-base-path meta -> getBasePath() falls back to "/".
    const promise = attemptSilentSSO({ timeoutMs: 50 });

    // The iframe is appended synchronously by attemptSilentSSO.
    const iframe = document.querySelector('iframe');
    expect(iframe).not.toBeNull();
    expect(iframe?.style.display).toBe('none');
    expect(iframe?.getAttribute('aria-hidden')).toBe('true');

    // The src must be RELATIVE (no leading slash) so it resolves
    // against the app's prefixed page URL, and must hit silent-check.
    const src = iframe?.getAttribute('src') ?? '';
    expect(src.startsWith('/')).toBe(false);
    expect(src).toContain('auth/silent-check');
    expect(src).toContain('return_to=');

    // jsdom never fires the iframe load event; the probe times out and
    // resolves as not-authenticated. It must NEVER reject.
    const result = await promise;
    expect(result.authenticated).toBe(false);
    if (result.authenticated === false) {
      expect(result.reason).toBe('timeout');
    }

    // The iframe is cleaned up after the probe settles.
    expect(document.querySelector('iframe')).toBeNull();
  });

  it('builds the silent-check URL with a moses-base-path meta present', async () => {
    // A moses-base-path meta tag is the canonical mount record.
    const meta = document.createElement('meta');
    meta.setAttribute('name', 'moses-base-path');
    meta.setAttribute('content', '/apps/tenant/oidc');
    document.head.appendChild(meta);
    resetBasePathCache();

    const promise = attemptSilentSSO({ timeoutMs: 50 });
    const src = document.querySelector('iframe')?.getAttribute('src') ?? '';
    expect(src).toContain('auth/silent-check');
    expect(src.startsWith('/')).toBe(false);
    await promise;
  });
});
