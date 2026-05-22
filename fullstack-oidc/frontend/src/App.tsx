/**
 * fullstack-oidc reference app (CHAT-t5d1u.28.8 skeleton).
 *
 * This is the working skeleton: it demonstrates the silent-SSO flow and
 * the protected /api/v1/me round-trip against the vendored oidcauth
 * backend middleware. The polished demo pages land in ticket 28.14.
 *
 * MOSES ROUTING: all fetch() calls use relative paths — see
 * ./auth/api.ts and ./utils/baseUrl.ts.
 */
import { useCallback, useEffect, useState } from 'react';
import { fetchMe, fetchPublicInfo, type MeResponse, type PublicInfo } from './auth/api';
import { attemptSilentSSO, startInteractiveLogin, logout } from './auth/silentSSO';

type Phase = 'loading' | 'authenticated' | 'anonymous';

function App() {
  const [phase, setPhase] = useState<Phase>('loading');
  const [me, setMe] = useState<MeResponse | null>(null);
  const [info, setInfo] = useState<PublicInfo | null>(null);
  const [note, setNote] = useState<string>('');

  // On mount: try the existing session, then a silent SSO probe, then
  // fall back to anonymous (the user clicks "Sign in").
  const bootstrap = useCallback(async () => {
    setPhase('loading');
    try {
      setInfo(await fetchPublicInfo());
    } catch {
      /* public-info is best-effort */
    }

    // 1. Existing session cookie?
    const existing = await fetchMe();
    if (existing?.authenticated) {
      setMe(existing);
      setPhase('authenticated');
      return;
    }

    // 2. Silent SSO — invisible if the user is already logged into Moses.
    const silent = await attemptSilentSSO();
    if (silent.authenticated) {
      const after = await fetchMe();
      if (after?.authenticated) {
        setMe(after);
        setPhase('authenticated');
        setNote('Signed in silently (no login page shown).');
        return;
      }
    }

    // 3. Anonymous — interactive login required.
    setPhase('anonymous');
    if (silent.authenticated === false) {
      setNote(`Silent SSO did not establish a session (${silent.reason}).`);
    }
  }, []);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  return (
    <main className="oidc-app">
      <h1>Moses Fullstack OIDC Template</h1>
      <p className="subtitle">
        Reference app for the vendored <code>oidcauth</code> BFF middleware.
      </p>

      {info && (
        <p className="info-line">
          Backend: <strong>{info.app}</strong> — OIDC enforcement{' '}
          {info.oidc_enabled ? 'enabled' : 'disabled (pass-through)'}
        </p>
      )}

      {note && <p className="note">{note}</p>}

      {phase === 'loading' && <p>Checking your session…</p>}

      {phase === 'anonymous' && (
        <section>
          <p>You are not signed in.</p>
          <button onClick={() => startInteractiveLogin()}>Sign in with Moses</button>
        </section>
      )}

      {phase === 'authenticated' && me && (
        <section className="identity">
          <h2>Signed in</h2>
          <dl>
            <dt>Subject</dt>
            <dd>{me.subject}</dd>
            <dt>Name</dt>
            <dd>{me.name || '—'}</dd>
            <dt>Email</dt>
            <dd>{me.email || '—'}</dd>
            <dt>Auth source</dt>
            <dd>{me.source}</dd>
            <dt>Roles</dt>
            <dd>{me.roles.length ? me.roles.join(', ') : '—'}</dd>
          </dl>
          <button onClick={() => logout()}>Sign out</button>
        </section>
      )}
    </main>
  );
}

export default App;
