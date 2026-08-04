/**
 * WORKED EXAMPLE — the component half of the `things` slice.
 *
 * WHY THIS IS A REAL FILE AND NOT A COMMENT: `npm run lint` / `npm run build`
 * run `tsc -p tsconfig.build.json`, whose `include` is `src`, so this file is
 * type-checked — a defect here fails CI. Nothing imports it, so Vite
 * tree-shakes it out of the bundle: compiled, never shipped. Keep it that way
 * — do not import it from app code.
 *
 * HOW TO USE IT: move the component into your own page under src/pages/,
 * register a route for it in src/App.tsx, rename Thing/things to your real
 * resource, then delete this file (and src/api/example.ts).
 */
import { getErrorMessage } from './api/client';
import { useCreateThing, useThings } from './api/example';

export function Things() {
  // Data comes from the TanStack Query hook — never a useEffect fetch, never a
  // raw fetch() in a component.
  const { data, isPending, isError, error } = useThings();
  const createThing = useCreateThing();

  if (isPending) return <p>Loading…</p>;
  // Normalize the thrown value — a queryFn can reject with anything, so never
  // cast it as `error as Error`.
  if (isError) return <p>{getErrorMessage(error)}</p>;

  return (
    <>
      <ul>
        {data.things.map((t) => (
          <li key={t.id}>{t.name}</li>
        ))}
      </ul>
      {/* The list re-fetches via the mutation's invalidation — no manual refetch. */}
      <button onClick={() => createThing.mutate({ name: 'New thing' })}>Add</button>
    </>
  );
}
