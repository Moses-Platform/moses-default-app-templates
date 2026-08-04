import { attemptSilentSSO } from './silentSSO';
import { resetBasePathCache } from '../utils/baseUrl';

// silentSSO's URL builders are private; we exercise them indirectly via
// attemptSilentSSO, which appends a hidden <iframe> whose `src` is the
// /auth/silent-check URL. silentCheckURL shares the exact base-prefix
// construction used by startInteractiveLogin / logout, so asserting the
// iframe src proves the prefix logic for all three navigations.
//
// In jsdom the iframe never fires `load`, so the probe resolves on its
// timeout — we use a short timeout and assert on the iframe `src`.

// clearNode removes all children of a node without touching innerHTML.
function clearNode(node: Node): void {
  while (node.firstChild) node.removeChild(node.firstChild);
}

function setBasePathMeta(path: string): void {
  const meta = document.createElement('meta');
  meta.setAttribute('name', 'moses-base-path');
  meta.setAttribute('content', path);
  document.head.appendChild(meta);
  resetBasePathCache();
}

describe('attemptSilentSSO', () => {
  beforeEach(() => {
    // getBasePath() only honours the mount when the live pathname is under
    // it, so tests must drive a real URL, not just the meta tag.
    window.history.pushState({}, '', '/');
    resetBasePathCache();
    clearNode(document.head);
    clearNode(document.body);
  });

  it('appends a hidden iframe pointing at an ABSOLUTE /auth/silent-check URL', async () => {
    // No moses-base-path meta -> getBasePath() falls back to "/".
    const promise = attemptSilentSSO({ timeoutMs: 50 });

    // The iframe is appended synchronously by attemptSilentSSO.
    const iframe = document.querySelector('iframe');
    expect(iframe).not.toBeNull();
    expect(iframe?.style.display).toBe('none');
    expect(iframe?.getAttribute('aria-hidden')).toBe('true');

    // The src must be ABSOLUTE (root-relative, leading slash) -> '/auth/...'.
    // A path-relative src would re-resolve against the current sub-route's
    // directory and double the base prefix (the prod /admin bug).
    const src = iframe?.getAttribute('src') ?? '';
    expect(src.startsWith('/auth/silent-check')).toBe(true);
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

  it('builds an ABSOLUTE, non-doubled silent-check URL from a sub-route under a base path', async () => {
    // Reproduce the prod scenario: app mounted at /apps/tenant/oidc, user on
    // the /admin sub-route when auth kicks off.
    window.history.pushState({}, '', '/apps/tenant/oidc/admin');
    setBasePathMeta('/apps/tenant/oidc');

    const promise = attemptSilentSSO({ timeoutMs: 50 });
    const iframe = document.querySelector('iframe');

    // Raw attribute must be absolute + single base prefix.
    const rawSrc = iframe?.getAttribute('src') ?? '';
    expect(rawSrc.startsWith('/apps/tenant/oidc/auth/silent-check')).toBe(true);

    // The RESOLVED url (what the browser actually navigates to) must NOT
    // double the mount: the old path-relative src produced
    // '/apps/tenant/oidc/apps/tenant/oidc/auth/silent-check'.
    const resolved = iframe?.src ?? '';
    expect(resolved).not.toContain('/apps/tenant/oidc/apps/tenant/oidc');
    expect(resolved).toContain('/apps/tenant/oidc/auth/silent-check');

    await promise;
  });
});
