import { act, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';

// Canonical test convention for the Query data layer: wrap renders in a
// QueryClientProvider with retries OFF so query hooks resolve deterministically.
// Copy this wrapper into any test that renders components using ../api/hooks.
const testQueryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

// Stub the API client so OverviewPage's mount-time fetch doesn't run.
// In jsdom, relative URLs ('api/v1/moses-info') reject with a URL parse
// error because window.location is about:blank. The smoke test only
// verifies the component tree renders; real fetches belong in dedicated
// integration tests, not this one.
vi.mock('./api/client', () => ({
  getMosesInfo: vi.fn().mockResolvedValue(null),
  getCapabilities: vi.fn().mockResolvedValue([]),
  getCapability: vi.fn().mockResolvedValue(null),
  getHealth: vi.fn().mockResolvedValue({ status: 'ok', service: '', version: '' }),
  getUsers: vi.fn().mockResolvedValue({ users: [] }),
  listNotes: vi.fn().mockResolvedValue([]),
  createNote: vi.fn().mockResolvedValue(null),
  deleteNote: vi.fn().mockResolvedValue(undefined),
}));

describe('App', () => {
  it('renders without crashing', async () => {
    // act(async) flushes the post-mount setState that fires when the
    // mocked getMosesInfo promise resolves — without it React warns
    // about an un-acted state update.
    let container: HTMLElement;
    await act(async () => {
      ({ container } = render(
        <QueryClientProvider client={testQueryClient}>
          <MemoryRouter>
            <App />
          </MemoryRouter>
        </QueryClientProvider>
      ));
    });
    expect(container!.children.length).toBeGreaterThan(0);
  });
});
