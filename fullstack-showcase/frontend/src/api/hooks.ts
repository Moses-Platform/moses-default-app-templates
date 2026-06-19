// Canonical Moses data-layer hooks.
//
// Components consume these — NEVER call fetch()/the client directly from a
// component (except an explicit on-click "playground" action) and NEVER load
// data in a useEffect. Reads are useQuery; writes are useMutation with explicit
// cache invalidation. The queryFn signal is threaded so Query can cancel
// in-flight requests on unmount / key change / refetch supersession.
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import {
  getMosesInfo,
  getHealth,
  getCapabilities,
  getCapability,
  getUsers,
  listNotes,
  createNote,
  deleteNote,
  type NoteInput,
} from './client';

// ---- Reads -----------------------------------------------------------------

export function useMosesInfo() {
  return useQuery({ queryKey: queryKeys.mosesInfo, queryFn: ({ signal }) => getMosesInfo(signal) });
}

export function useHealth() {
  return useQuery({ queryKey: queryKeys.health, queryFn: ({ signal }) => getHealth(signal) });
}

export function useCapabilities() {
  return useQuery({ queryKey: queryKeys.capabilities.all, queryFn: ({ signal }) => getCapabilities(signal) });
}

export function useCapability(id: string) {
  return useQuery({
    queryKey: queryKeys.capabilities.detail(id),
    queryFn: ({ signal }) => getCapability(id, signal),
    enabled: !!id,
  });
}

export function useUsers() {
  return useQuery({ queryKey: queryKeys.users, queryFn: ({ signal }) => getUsers(signal) });
}

export function useNotes() {
  return useQuery({ queryKey: queryKeys.notes, queryFn: ({ signal }) => listNotes(signal) });
}

// ---- Writes (mutation + invalidation) --------------------------------------
// No optimistic updates by design (teaching template): invalidate on success so
// the list reconciles with server truth. onSettled also reconciles after error.

export function useCreateNote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: NoteInput) => createNote(input),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.notes }),
  });
}

export function useDeleteNote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteNote(id),
    onSettled: () => qc.invalidateQueries({ queryKey: queryKeys.notes }),
  });
}
