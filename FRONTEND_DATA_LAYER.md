# Moses frontend data-layer pattern (canonical)

The reference implementation lives in **`fullstack-showcase/frontend`**. Every
Moses template frontend follows this exact shape, and the Moses platform
frontend adopts it too. The goal: **server state lives in TanStack Query, not
in `useEffect` + `useState`.**

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
// hooks.ts
export function useCapabilities() {
  return useQuery({ queryKey: queryKeys.capabilities.all, queryFn: getCapabilities });
}
// component
const { data, isPending, isError, error } = useCapabilities();
```
No `useEffect`, no loading/error `useState`. (Imperative on-click calls — e.g. an
"API playground" — may still call the client directly; that's an explicit user
action, not data-load-on-mount.)

## Writes (mutation + invalidation)
```ts
export function useCreateNote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: NoteInput) => createNote(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.notes }),
  });
}
// component
const createNote = useCreateNote();
createNote.mutate(input, { onSuccess: () => resetForm() });
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
