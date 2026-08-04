import { act, render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';

// Canonical test convention for the Query data layer: wrap renders in a
// QueryClientProvider with retries OFF so query hooks resolve deterministically.
// Copy this wrapper into any test that renders components using ../api/hooks.
// When your components start fetching, stub the client module (vi.mock of
// './api/client') — in jsdom, relative URLs reject because window.location is
// about:blank; real fetches belong in integration tests.
const testQueryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

describe('App', () => {
  it('renders without crashing', async () => {
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
