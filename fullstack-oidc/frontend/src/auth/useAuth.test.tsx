/**
 * AuthProvider bootstrap-phase tests (relogin-on-refresh fix, UX half).
 *
 * On every page load whose session cookie has lapsed, the SPA runs a
 * silent (prompt=none) SSO probe. The UI must stay in the `loading`
 * phase until that probe SETTLES — flashing the anonymous "Sign in"
 * state for the probe's duration made every refresh look like a logout
 * even when the probe silently re-authenticated half a second later.
 */
import { render, screen, act, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider, useAuth } from './useAuth';
import type { MeResponse } from '../api/client';

const me: MeResponse = {
  authenticated: true,
  source: 'session',
  subject: 'u-1',
  email: 'u@x.io',
  name: 'U',
  username: 'u',
  roles: [],
};

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    getMe: vi.fn(),
    getPublicInfo: vi
      .fn()
      .mockResolvedValue({ app: 'fullstack-oidc', oidc_enabled: true, known_roles: [] }),
  };
});
vi.mock('./silentSSO', () => ({
  attemptSilentSSO: vi.fn(),
  startInteractiveLogin: vi.fn(),
  logout: vi.fn(),
}));

import { getMe } from '../api/client';
import { attemptSilentSSO } from './silentSSO';

/** Renders the live phase and records every phase transition. */
function PhaseProbe({ phases }: { phases: string[] }) {
  const { phase } = useAuth();
  if (phases[phases.length - 1] !== phase) phases.push(phase);
  return <div data-testid="phase">{phase}</div>;
}

function renderProbe(phases: string[]) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <PhaseProbe phases={phases} />
      </AuthProvider>
    </QueryClientProvider>,
  );
}

describe('AuthProvider bootstrap phases', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('stays in loading (not anonymous) while the silent-SSO probe is in flight', async () => {
    vi.mocked(getMe).mockResolvedValue(null); // anonymous session
    let resolveProbe!: (r: { authenticated: boolean; reason?: string }) => void;
    vi.mocked(attemptSilentSSO).mockImplementation(
      () => new Promise((res) => (resolveProbe = res as typeof resolveProbe)),
    );

    const phases: string[] = [];
    renderProbe(phases);

    // Let the `me` query settle to anonymous; the probe is now pending.
    await waitFor(() => expect(vi.mocked(attemptSilentSSO)).toHaveBeenCalledTimes(1));
    expect(screen.getByTestId('phase').textContent).toBe('loading');
    expect(phases).not.toContain('anonymous');

    // Probe fails -> NOW the app is genuinely anonymous.
    await act(async () => {
      resolveProbe({ authenticated: false, reason: 'login_required' });
    });
    expect(screen.getByTestId('phase').textContent).toBe('anonymous');
  });

  it('lands directly in authenticated when the probe silently signs in', async () => {
    // First read: anonymous. After the probe: authenticated.
    vi.mocked(getMe).mockResolvedValueOnce(null).mockResolvedValue(me);
    vi.mocked(attemptSilentSSO).mockResolvedValue({ authenticated: true });

    const phases: string[] = [];
    renderProbe(phases);

    expect(await screen.findByText('authenticated')).toBeTruthy();
    // The whole point: no anonymous flash on a silently-recovered session.
    expect(phases).not.toContain('anonymous');
  });

  it('skips the probe entirely when the session cookie is still valid', async () => {
    vi.mocked(getMe).mockResolvedValue(me);
    vi.mocked(attemptSilentSSO).mockResolvedValue({ authenticated: false, reason: 'error' });

    const phases: string[] = [];
    renderProbe(phases);

    expect(await screen.findByText('authenticated')).toBeTruthy();
    expect(vi.mocked(attemptSilentSSO)).not.toHaveBeenCalled();
    expect(phases).not.toContain('anonymous');
  });
});
