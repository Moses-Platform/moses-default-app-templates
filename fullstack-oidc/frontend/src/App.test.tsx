import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';

// Stub the API client + silent-SSO modules so the mount-time bootstrap does
// not hit the network or open a real iframe. jsdom rejects relative-URL
// fetches (window.location is about:blank) and never fires iframe `load`,
// so the unmocked path would hang. The smoke test only verifies the
// component tree renders.
vi.mock('./api/client', async () => {
  const actual = await vi.importActual<typeof import('./api/client')>('./api/client');
  return {
    ...actual,
    getMe: vi.fn().mockResolvedValue(null),
  };
});
vi.mock('./auth/silentSSO', () => ({
  attemptSilentSSO: vi.fn().mockResolvedValue({ authenticated: false, reason: 'login_required' }),
  startInteractiveLogin: vi.fn(),
  logout: vi.fn(),
}));

describe('App', () => {
  it('renders the home page in the anonymous state without crashing', async () => {
    // A fresh QueryClient per test (no retries) — App's AuthProvider reads
    // server state via TanStack Query, so it must mount under a provider.
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>,
    );
    // The Layout header is always present.
    expect(screen.getByText('My Moses App')).toBeTruthy();
    // After the mocked `me` query settles to anonymous, the CTA is shown
    // (findBy* waits for the Query-driven re-render).
    expect((await screen.findAllByText('Sign in with Moses')).length).toBeGreaterThan(0);
    // The placeholder home page renders.
    expect(screen.getByText('Your app starts here')).toBeTruthy();
  });
});
