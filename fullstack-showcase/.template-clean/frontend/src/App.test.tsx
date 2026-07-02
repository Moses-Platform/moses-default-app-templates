import { act, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';

// Canonical test convention for the Query data layer: wrap renders in a
// QueryClientProvider with retries OFF so query hooks resolve deterministically.
// Copy this wrapper into any test that renders components using ../api/hooks.
//
// When your components fetch on mount, stub the API client so jsdom's
// relative-URL limitation doesn't bite:
//   vi.mock('./api/client', () => ({
//     getErrorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
//     getHealth: vi.fn().mockResolvedValue({ status: 'ok', service: '', version: '' }),
//   }));
const testQueryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

describe('App', () => {
  it('renders without crashing', async () => {
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
