/**
 * Shared authentication state for the fullstack-oidc reference app.
 *
 * `AuthProvider` runs the BFF bootstrap ONCE at app mount:
 *   1. Try the existing session cookie (`GET /api/v1/me`).
 *   2. If none, run a silent (`prompt=none`) SSO probe in a hidden
 *      iframe — invisible if the user is already logged into Moses.
 *   3. Fall back to anonymous; the user clicks "Sign in".
 *
 * Every page reads the result via `useAuth()` so the demo pages all
 * reflect the same identity without each re-fetching.
 */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { fetchMe, fetchPublicInfo, type MeResponse, type PublicInfo } from './api';
import { attemptSilentSSO, startInteractiveLogin, logout } from './silentSSO';

/** Where the bootstrap currently stands. */
export type AuthPhase = 'loading' | 'authenticated' | 'anonymous';

/** How the current session was established (for the demo narration). */
export type AuthVia = 'session' | 'silent-sso' | 'none';

export interface AuthState {
  phase: AuthPhase;
  /** The authenticated principal, or null when anonymous. */
  me: MeResponse | null;
  /** Always-public backend metadata (best-effort). */
  info: PublicInfo | null;
  /** How the session was obtained — drives the "silent SSO" narration. */
  via: AuthVia;
  /** Human-readable note about the last bootstrap outcome. */
  note: string;
  /** Re-run the full bootstrap (used after sign-in returns). */
  refresh: () => Promise<void>;
  /** Navigate to the interactive Keycloak login. */
  signIn: () => void;
  /** RP-initiated logout. */
  signOut: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [phase, setPhase] = useState<AuthPhase>('loading');
  const [me, setMe] = useState<MeResponse | null>(null);
  const [info, setInfo] = useState<PublicInfo | null>(null);
  const [via, setVia] = useState<AuthVia>('none');
  const [note, setNote] = useState('');

  const refresh = useCallback(async () => {
    setPhase('loading');
    try {
      setInfo(await fetchPublicInfo());
    } catch {
      /* public-info is best-effort — the app still works without it. */
    }

    // 1. Existing session cookie?
    const existing = await fetchMe();
    if (existing?.authenticated) {
      setMe(existing);
      setVia('session');
      setNote('Signed in from an existing session cookie.');
      setPhase('authenticated');
      return;
    }

    // 2. Silent SSO — invisible if the user is already logged into Moses.
    const silent = await attemptSilentSSO();
    if (silent.authenticated) {
      const after = await fetchMe();
      if (after?.authenticated) {
        setMe(after);
        setVia('silent-sso');
        setNote('Signed in silently via prompt=none — no login page was shown.');
        setPhase('authenticated');
        return;
      }
    }

    // 3. Anonymous — interactive login required.
    setMe(null);
    setVia('none');
    setNote(
      silent.authenticated === false
        ? `Silent SSO did not establish a session (${silent.reason}). Click "Sign in".`
        : 'Not signed in.',
    );
    setPhase('anonymous');
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const value = useMemo<AuthState>(
    () => ({
      phase,
      me,
      info,
      via,
      note,
      refresh,
      signIn: () => startInteractiveLogin(),
      signOut: () => logout(),
    }),
    [phase, me, info, via, note, refresh],
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
