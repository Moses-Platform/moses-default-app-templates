/**
 * Shared authentication state for the fullstack-oidc reference app.
 *
 * PURE AUTH PLUMBING — no demo/app data lives here. Identity is SERVER
 * STATE, so it lives in TanStack Query (`useMe` in ../api/hooks) — not in
 * hand-rolled useState + useEffect-fetch. This provider layers the OIDC
 * BFF bootstrap lifecycle on top of that query:
 *   1. The `me` query reads the existing session cookie (`GET /api/v1/me`).
 *   2. If it resolves anonymous, a silent (`prompt=none`) SSO probe runs
 *      ONCE in a hidden iframe — invisible if the user is already logged
 *      into Moses — then the `me` query is invalidated to re-read.
 *   3. Otherwise the user is anonymous and clicks "Sign in".
 *
 * Every page reads the derived phase via `useAuth()` so the demo pages all
 * reflect the same identity without each re-fetching.
 */
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useMe } from '../api/hooks';
import { queryKeys } from '../api/queryKeys';
import type { MeResponse } from '../api/client';
import { attemptSilentSSO, startInteractiveLogin, logout } from './silentSSO';

/** Where the bootstrap currently stands. */
export type AuthPhase = 'loading' | 'authenticated' | 'anonymous';

/** How the current session was established (for the demo narration). */
export type AuthVia = 'session' | 'silent-sso' | 'none';

export interface AuthState {
  phase: AuthPhase;
  /** The authenticated principal, or null when anonymous. */
  me: MeResponse | null;
  /** How the session was obtained — drives the "silent SSO" narration. */
  via: AuthVia;
  /** Human-readable note about the last bootstrap outcome. */
  note: string;
  /** Re-read the identity (used after sign-in returns / a silent probe). */
  refresh: () => Promise<void>;
  /** Navigate to the interactive Keycloak login. */
  signIn: () => void;
  /** RP-initiated logout. */
  signOut: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const meQuery = useMe();

  // `via` is the only piece of genuinely local lifecycle state: it records
  // HOW the (server-owned) identity was obtained, for the demo narration.
  const [via, setVia] = useState<AuthVia>('none');
  // Guard so the silent-SSO probe runs at most once per anonymous result.
  const silentTried = useRef(false);

  const me = meQuery.data ?? null;

  // Bootstrap orchestration (an imperative side-effect, NOT data-into-useState:
  // the data lives in Query). When the `me` query settles to anonymous and a
  // silent probe has not yet been tried, run it once; if it establishes a
  // session, invalidate `me` so Query re-reads the now-authenticated identity.
  useEffect(() => {
    if (meQuery.isPending) return;
    if (me?.authenticated) {
      setVia((prev) => (prev === 'none' ? 'session' : prev));
      return;
    }
    if (silentTried.current) return;
    silentTried.current = true;
    void (async () => {
      const silent = await attemptSilentSSO();
      if (silent.authenticated) {
        setVia('silent-sso');
        await qc.invalidateQueries({ queryKey: queryKeys.me });
      }
    })();
  }, [meQuery.isPending, me?.authenticated, qc]);

  const phase: AuthPhase = meQuery.isPending
    ? 'loading'
    : me?.authenticated
      ? 'authenticated'
      : 'anonymous';

  const note = me?.authenticated
    ? via === 'silent-sso'
      ? 'Signed in silently via prompt=none — no login page was shown.'
      : 'Signed in from an existing session cookie.'
    : silentTried.current
      ? 'Silent SSO did not establish a session. Click "Sign in".'
      : 'Not signed in.';

  const value = useMemo<AuthState>(
    () => ({
      phase,
      me,
      via,
      note,
      refresh: async () => {
        silentTried.current = false;
        await qc.invalidateQueries({ queryKey: queryKeys.me });
      },
      signIn: () => startInteractiveLogin(),
      signOut: () => logout(),
    }),
    [phase, me, via, note, qc],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/** Read the shared auth state. Must be used under an `AuthProvider`. */
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an <AuthProvider>');
  }
  return ctx;
}
