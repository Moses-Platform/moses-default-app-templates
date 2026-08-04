# Moses frontend data-layer pattern (canonical)

The scaffolding lives in **`fullstack-showcase/frontend/src/api/`** (and the
same four files in every other React template). Each ships the transport,
the shared client, and an empty key factory / hooks module for you to fill in.
The Moses platform frontend adopts the same shape. The goal: **server state
lives in TanStack Query, not in `useEffect` + `useState`.**

## Why
The anti-pattern (`useEffect(() => { fetch(...).then(setState) }, [])`) hand-rolls
loading/error/cache/refetch in every component, scatters data access, and
produces stale-closure bugs (the reason `eslint-disable react-hooks/exhaustive-deps`
proliferates). TanStack Query owns caching, dedup, loading/error, and
invalidation once, centrally.

## The four files
| File | Role |
|------|------|
| `src/api/client.ts` | Typed, transport-only functions. `fetchAPI` (reads) + `mutateAPI` (writes). **Relative paths only** (`api/v1/...`, never `/api/...`) so the nginx subpath proxy works. |
| `src/api/queryKeys.ts` | Query-key factory — single source of truth for cache keys. Never inline a key array in a component. |
| `src/api/queryClient.ts` | The shared `QueryClient` (staleTime 30s, retry 1, no refetch-on-focus). |
| `src/api/hooks.ts` | `useXxx()` query hooks + `useXxx()` mutation hooks. **Components import only these.** |

`main.tsx` wraps the app in `<QueryClientProvider client={queryClient}>`.

## Reads
```ts
// client.ts
export interface Thing { id: string; name: string }
export const getThings = (signal?: AbortSignal) =>
  fetchAPI<{ things: Thing[] }>('api/v1/things', signal);
// queryKeys.ts
export const queryKeys = { things: ['things'] as const };
// hooks.ts
export function useThings() {
  return useQuery({ queryKey: queryKeys.things, queryFn: ({ signal }) => getThings(signal) });
}
// component
const { data, isPending, isError, error } = useThings();
```
No `useEffect`, no loading/error `useState`. (Imperative on-click calls — e.g. an
"API playground" — may still call the client directly; that's an explicit user
action, not data-load-on-mount.)

## Writes (mutation + invalidation)
```ts
// client.ts
export const createThing = (input: { name: string }, signal?: AbortSignal) =>
  mutateAPI<Thing>('api/v1/things', 'POST', input, signal);
// hooks.ts
export function useCreateThing() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string }) => createThing(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.things }),
  });
}
// component
const createThing = useCreateThing();
createThing.mutate(input, { onSuccess: () => resetForm() });
```
The list re-fetches automatically via invalidation — no manual refetch.

## CSRF — important difference between templates and the platform
- **Templates** (this repo): the backend CSRF guard is **`Sec-Fetch-Site`-based**
  (`backend/internal/middleware/csrf.go`, vendored from `shared/csrf-go`). Browsers
  set that header and forbid scripts from spoofing it, so a **same-origin
  `fetch()` mutation passes with no token**. `mutateAPI` therefore sends no CSRF
  header. Do not add one.
- **Moses platform frontend**: uses a **CSRF token** scheme. When porting this
  pattern there, attach the existing token in the single `mutateAPI` helper —
  every mutation flows through it, so it's one edit.

## Real-time updates
Drive freshness with **explicit invalidation**, not polling. Mutations invalidate
their keys (above). In the platform, WebSocket events call
`queryClient.invalidateQueries({ queryKey })` — the WS→cache bridge replaces the
per-component refetch debouncers.

## Rules (enforced by lint in the platform; conventions here)
1. No data-loading `useEffect` — use a query hook.
2. No `fetch`/client calls inside components (except explicit on-click playground actions) — go through `hooks.ts`.
3. Query keys come from `queryKeys.ts`.
4. Relative API paths only.
5. Client/UI state (form inputs, open/closed, selection) stays in `useState`/Zustand; server state stays in Query.
6. No manual `useMemo`/`useCallback`/`React.memo` — the **React Compiler** is enabled in every template's `vite.config.ts` (`babel-plugin-react-compiler`) and auto-memoizes. Write the plain value/function. (Only keep a memo whose dependency array is *intentionally partial* — a deliberate stale snapshot the compiler would otherwise change.)

---

## Design system (visual layer)

Every template ships the same Moses-aligned design system. Build the app's identity by *extending* it — never rip it out or hardcode around it.

**Files:**
- `src/styles/theme.css` — the token system: emerald brand + coral accent, three-tier background hierarchy, **dark-default + `[data-theme="light"]`** (+ a `prefers-color-scheme` no-JS fallback), display/mono/body font stacks with system fallbacks, plus spacing/radius/shadow/motion tokens. Values mirror the platform `MOSES_COLOR_SYSTEM`. Templates load **no webfonts** (the Google-Fonts CDN `<link>`s were removed); stacks that name `Sora` / `JetBrains Mono` resolve to their system fallbacks unless an app self-hosts those fonts.
- `src/App.css` — the global layer built on those tokens: typography, terminal-style `code`/`pre`, the dot-grid + glow atmosphere, the staggered page-load reveal, and the **unified button system** (`.btn`, `.btn-secondary`, `.btn-cta`, `.btn-ghost`, `.btn-danger`, `.btn-sm`, `.btn-icon`) plus form-field, badge, and table styles.
- `src/components/ThemeToggle.tsx` — the dark/light toggle (writes `data-theme`, persists to `localStorage('moses-theme')`, `matchMedia`-guarded for jsdom/SSR). Paired with the **no-flash init script in `index.html`** (sets `data-theme` before first paint).

(Vanilla, non-React templates ship the equivalent in `static/style.css` + `static/index.html` with a vanilla toggle — same tokens, same `.btn` classes, same dark/light.)

## Design rules
1. **Consume the tokens** (`var(--color-*)`, `var(--spacing-*)`, `var(--font-*)`) — no hardcoded hex / `px` / font names outside `theme.css`.
2. **Keep dark + light working** — never drop the toggle or the `[data-theme]` overrides; verify both.
3. **Use the `.btn` classes** and the shared form/badge/table styles — don't re-style buttons per component.
4. **Extend, don't replace** — add tokens/components on top of the system; don't remove it.
