import { act, render } from '@testing-library/react';
import App from './App';

// Stub the API + silent-SSO modules so the mount-time bootstrap does not
// hit the network or open a real iframe. jsdom rejects relative-URL
// fetches (window.location is about:blank) and never fires iframe
// `load`, so the unmocked path would hang. The smoke test only verifies
// the component tree renders for each phase.
vi.mock('./auth/api', () => ({
  fetchMe: vi.fn().mockResolvedValue(null),
  fetchPublicInfo: vi.fn().mockResolvedValue({ app: 'fullstack-oidc', oidc_enabled: true }),
}));
vi.mock('./auth/silentSSO', () => ({
  attemptSilentSSO: vi.fn().mockResolvedValue({ authenticated: false, reason: 'login_required' }),
  startInteractiveLogin: vi.fn(),
  logout: vi.fn(),
}));

describe('App', () => {
  it('renders the anonymous state without crashing', async () => {
    let container: HTMLElement;
    await act(async () => {
      ({ container } = render(<App />));
    });
    expect(container!.textContent).toContain('Moses Fullstack OIDC Template');
    // After the mocked bootstrap settles, the anonymous CTA is shown.
    expect(container!.textContent).toContain('Sign in with Moses');
  });
});
