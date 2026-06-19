import { act, render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';

// Canonical test convention for the Query data layer: wrap renders in a
// QueryClientProvider with retries OFF so query hooks resolve deterministically.
// Copy this wrapper into any test that renders components using ../api/hooks.
const testQueryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

// Stub the API client so the mount-time queries don't run. In jsdom, relative
// URLs ('api/v1/status') reject with a URL parse error because window.location
// is about:blank. The smoke test only verifies the component tree renders; real
// fetches belong in dedicated integration tests, not this one.
vi.mock('./api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api/client')>();
  return {
    ...actual,
    getStatus: vi.fn().mockResolvedValue(null),
    listItems: vi.fn().mockResolvedValue([]),
    createItem: vi.fn().mockResolvedValue(null),
    deleteItem: vi.fn().mockResolvedValue(undefined),
  };
});

describe('App', () => {
  it('renders without crashing', async () => {
    // act(async) flushes the post-mount setState that fires when the mocked
    // query promises resolve — without it React warns about an un-acted update.
    let container: HTMLElement;
    await act(async () => {
      ({ container } = render(
        <QueryClientProvider client={testQueryClient}>
          <App />
        </QueryClientProvider>
      ));
    });
    expect(container!.children.length).toBeGreaterThan(0);
  });
});
