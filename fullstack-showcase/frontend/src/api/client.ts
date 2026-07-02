// Typed transport layer for the Moses showcase.
//   - Reads: fetchAPI (GET, JSON in).
//   - Writes: mutateAPI (body in, JSON entity back) / mutateAPINoContent (204/empty).
// RELATIVE paths only ('api/v1/...', never '/api/...') so the nginx subpath proxy
// works under /apps/<tenant>/<slug>/. See FRONTEND_DATA_LAYER.md.
//
// AbortSignal is threaded so TanStack Query can cancel in-flight requests on
// unmount / key change / refetch supersession.

export interface MosesInfo {
  // CHAT-pxeo.12: the backend distinguishes the deploy-pinned storage
  // tenant (self_tenant_id, from the MOSES_TENANT_ID env) from the
  // caller-context header value (caller_tenant_id, X-Moses-Tenant-ID).
  self_tenant_id: string;
  caller_tenant_id: string;
  user_id: string;
  chart_id: string;
  tool_id: string;
  request_id: string;
  mcp_source: string;
  api_key_id: string;
  headers_present: boolean;
  deployment_mode: 'standalone' | 'mcp-proxy';
}

export interface Capability {
  id: string;
  title: string;
  description: string;
  category: string;
  icon: string;
  details: string[];
}

export interface PlatformUser {
  id: string;
  email: string;
  displayName: string;
  role: string;
}

export interface UsersResponse {
  users: PlatformUser[];
  message?: string;
}

export interface Note {
  id: string;
  title: string;
  content: string;
  created_at: string;
  updated_at: string;
}

export interface NoteInput {
  title: string;
  content: string;
}

// Normalize an unknown thrown value to a message. Use this in components/hooks
// instead of `error as Error` — a queryFn can reject with anything.
export function getErrorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === 'string') return e;
  return 'Unexpected error';
}

// Build a rich error from a non-2xx response, surfacing the server's body
// (the Go backend uses http.Error with meaningful plaintext, e.g. "Title is
// required") instead of the generic statusText.
async function failure(response: Response): Promise<never> {
  let detail = '';
  try {
    detail = (await response.text()).trim();
  } catch {
    /* body already consumed / unavailable */
  }
  const base = `API error ${response.status}`;
  throw new Error(detail ? `${base}: ${detail.slice(0, 300)}` : `${base}: ${response.statusText || 'request failed'}`);
}

async function fetchAPI<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    signal,
  });
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

// Write that expects a JSON entity back (200/201).
async function mutateAPI<T>(path: string, method: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  if (!response.ok) return failure(response);
  return response.json() as Promise<T>;
}

// Write that expects no entity back (204 / empty body).
async function mutateAPINoContent(path: string, method: string, signal?: AbortSignal): Promise<void> {
  const response = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    signal,
  });
  if (!response.ok) await failure(response);
}

export const getMosesInfo = (signal?: AbortSignal) => fetchAPI<MosesInfo>('api/v1/moses-info', signal);
export const getCapabilities = (signal?: AbortSignal) => fetchAPI<Capability[]>('api/v1/capabilities', signal);
export const getCapability = (id: string, signal?: AbortSignal) => fetchAPI<Capability>(`api/v1/capabilities/${id}`, signal);
export const getHealth = (signal?: AbortSignal) =>
  fetchAPI<{ status: string; service: string; version: string }>('health', signal);
export const getUsers = (signal?: AbortSignal) => fetchAPI<UsersResponse>('api/v1/users', signal);
export const listNotes = (signal?: AbortSignal) => fetchAPI<Note[]>('api/v1/notes', signal);
export const createNote = (input: NoteInput) => mutateAPI<Note>('api/v1/notes', 'POST', input);
export const deleteNote = (id: string) => mutateAPINoContent(`api/v1/notes/${id}`, 'DELETE');
