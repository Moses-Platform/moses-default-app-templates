// CHAT-il3y6: tests for the browser-logger's pre-bootstrap crash-capture.
//
// The bug this guards against: the window 'error' / 'unhandledrejection'
// listeners used to be attached only AFTER `await fetch(bootstrapUrl)`
// resolved, so a render-time crash that fired window.onerror in the same tick
// as installBrowserLogger() was lost. The fix attaches the listeners
// synchronously and buffers events (bounded) until bootstrap resolves, then
// either drains the buffer (enabled) or drops it (disabled / error).
//
// Vitest + jsdom (configured in vite.config.ts). This file imports the
// VENDORED copy (./moses-browser-logger), which the parity gate keeps
// byte-identical to shared/browser-logger/index.ts.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { installBrowserLogger } from './moses-browser-logger'

// import.meta.env values that make installBrowserLogger proceed past the
// env-guard. vite.config.ts's define / import.meta.env is read via readViteEnv;
// we stub it through vi.stubGlobal-style env injection below.
const CHART = 'chart-uuid-1'
const DEPLOY = 'deploy-uuid-1'

// Helper: build a fetch mock where the FIRST call (bootstrap) resolves with the
// given bootstrap body, and every subsequent call (ingest POST) is recorded.
function makeFetchMock(bootstrap: {
  ok?: boolean
  body?: unknown
  reject?: boolean
}) {
  const ingestCalls: Array<{ url: string; events: unknown[] }> = []
  let bootstrapResolve: (() => void) | null = null
  const bootstrapGate = new Promise<void>((r) => {
    bootstrapResolve = r
  })

  const fetchMock = vi.fn((url: string, _opts?: RequestInit) => {
    if (url.indexOf('/api/v1/browser-logs/bootstrap') !== -1) {
      // Bootstrap call — gate it so the test can fire a crash BEFORE it
      // resolves, proving the listeners are attached synchronously.
      return bootstrapGate.then(() => {
        if (bootstrap.reject) {
          return Promise.reject(new Error('network'))
        }
        return {
          ok: bootstrap.ok ?? true,
          json: () => Promise.resolve(bootstrap.body),
        } as unknown as Response
      })
    }
    // Ingest POST — record it.
    try {
      const parsed = JSON.parse(String((_opts as RequestInit).body))
      ingestCalls.push({ url: String(url), events: parsed.events })
    } catch {
      ingestCalls.push({ url: String(url), events: [] })
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve({}) } as unknown as Response)
  })

  return { fetchMock, ingestCalls, releaseBootstrap: () => bootstrapResolve!() }
}

// Fire a window 'error' event the way a render-time crash would. We build a
// plain Event and decorate it with the ErrorEvent-shaped fields the logger
// reads, rather than a real ErrorEvent — jsdom re-throws ErrorEvents that
// carry a live `error` object as an uncaught exception, which would crash the
// test runner instead of exercising the listener.
function fireWindowError(message: string) {
  const evt = new Event('error') as Event & {
    message: string
    error: { stack: string }
    filename: string
    lineno: number
    colno: number
  }
  evt.message = message
  evt.error = { stack: message + '\n  at app.js:1:1' }
  evt.filename = 'app.js'
  evt.lineno = 1
  evt.colno = 1
  window.dispatchEvent(evt)
}

// jsdom re-throws an unhandled window 'error' event (one carrying a truthy
// `error`) as an uncaught exception that would fail the test runner. A
// top-level listener that calls preventDefault marks it handled. The logger's
// own listener still runs alongside this one.
const swallowError = (e: Event) => e.preventDefault()

// vitest isolates modules per FILE, not per test — every installBrowserLogger()
// call permanently attaches window listeners to the shared jsdom window, so
// without cleanup a later test's dispatched crash would also fire EARLIER
// tests' still-live listeners and inflate the send count. We wrap
// window.add/removeEventListener to record every listener the logger registers
// during a test and tear them all down in afterEach, giving real per-test
// isolation. (In production there is exactly one install per page, so this is
// purely a test concern.)
const realAdd = window.addEventListener.bind(window)
const realRemove = window.removeEventListener.bind(window)
let trackedListeners: Array<[string, EventListenerOrEventListenerObject]> = []

// The logger reads import.meta.env via readViteEnv(). vi.stubEnv is vitest's
// supported way to set those keys so all modules (incl. the logger) see them.
// The enabled-path install patches console.error / console.warn. Save and
// restore them so the patch never leaks into other tests or test files.
let savedConsoleError: typeof console.error
let savedConsoleWarn: typeof console.warn

beforeEach(() => {
  savedConsoleError = console.error
  savedConsoleWarn = console.warn
  trackedListeners = []
  window.addEventListener = ((type: string, l: EventListenerOrEventListenerObject, opts?: unknown) => {
    trackedListeners.push([type, l])
    return realAdd(type, l, opts as AddEventListenerOptions)
  }) as typeof window.addEventListener

  window.addEventListener('error', swallowError)
  vi.stubEnv('VITE_MOSES_CHART_ID', CHART)
  vi.stubEnv('VITE_MOSES_DEPLOYMENT_ID', DEPLOY)
  vi.stubEnv('VITE_MOSES_API_BASE', '')
})

afterEach(() => {
  for (const [type, l] of trackedListeners) {
    realRemove(type, l)
  }
  window.addEventListener = realAdd as typeof window.addEventListener
  trackedListeners = []
  console.error = savedConsoleError
  console.warn = savedConsoleWarn
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
})

describe('installBrowserLogger pre-bootstrap crash capture', () => {
  it('captures a crash fired BEFORE bootstrap resolves and flushes it once after', async () => {
    const { fetchMock, ingestCalls, releaseBootstrap } = makeFetchMock({
      ok: true,
      body: { enabled: true, token: 'tok', endpoint: '/api/v1/browser-logs/ingest' },
    })
    vi.stubGlobal('fetch', fetchMock)

    // Kick off install but DO NOT await — bootstrap is gated open below.
    const installed = installBrowserLogger()

    // Simulate a render-time crash in the SAME tick, before bootstrap resolves.
    fireWindowError('R.map is not a function')

    // No ingest yet — bootstrap hasn't resolved.
    expect(ingestCalls.length).toBe(0)

    // Now let bootstrap resolve and the install finish.
    releaseBootstrap()
    await installed
    // Allow the drain flush microtask to run.
    await Promise.resolve()

    // Exactly one ingest call, carrying the buffered crash, sent ONCE.
    expect(ingestCalls.length).toBe(1)
    expect(ingestCalls[0].events.length).toBe(1)
    const ev = ingestCalls[0].events[0] as { message: string; level: string }
    expect(ev.level).toBe('error')
    expect(ev.message).toContain('R.map is not a function')
  })

  it('drops the buffer and sends nothing when bootstrap reports disabled', async () => {
    const { fetchMock, ingestCalls, releaseBootstrap } = makeFetchMock({
      ok: true,
      body: { enabled: false },
    })
    vi.stubGlobal('fetch', fetchMock)

    const installed = installBrowserLogger()
    fireWindowError('crash before disabled bootstrap')
    releaseBootstrap()
    await installed
    await Promise.resolve()

    // Disabled bootstrap → buffer dropped, zero ingest sends.
    expect(ingestCalls.length).toBe(0)

    // A post-bootstrap crash must also be a no-op (dropped flag latched).
    fireWindowError('crash after disabled bootstrap')
    await Promise.resolve()
    expect(ingestCalls.length).toBe(0)
  })

  it('drops the buffer and sends nothing when bootstrap errors', async () => {
    const { fetchMock, ingestCalls, releaseBootstrap } = makeFetchMock({
      reject: true,
    })
    vi.stubGlobal('fetch', fetchMock)

    const installed = installBrowserLogger()
    fireWindowError('crash before failing bootstrap')
    releaseBootstrap()
    await installed
    await Promise.resolve()

    expect(ingestCalls.length).toBe(0)
  })

  it('bounds the pre-bootstrap buffer to the cap (drop-oldest)', async () => {
    const { fetchMock, ingestCalls, releaseBootstrap } = makeFetchMock({
      ok: true,
      body: { enabled: true, token: 'tok', endpoint: '/api/v1/browser-logs/ingest' },
    })
    vi.stubGlobal('fetch', fetchMock)

    const installed = installBrowserLogger()

    // Fire many more than the 50-event cap before bootstrap resolves.
    const TOTAL = 130
    for (let i = 0; i < TOTAL; i++) {
      fireWindowError('crash #' + i)
    }

    releaseBootstrap()
    await installed
    await Promise.resolve()

    // Buffer was capped at 50, drop-oldest, so the drained flush carries
    // exactly 50 events and the OLDEST were dropped (most-recent retained).
    expect(ingestCalls.length).toBe(1)
    const events = ingestCalls[0].events as Array<{ message: string }>
    expect(events.length).toBe(50)
    // Newest event retained, oldest dropped.
    expect(events[events.length - 1].message).toContain('crash #' + (TOTAL - 1))
    expect(events[0].message).toContain('crash #' + (TOTAL - 50))
  })

  it('silently no-ops when chart/deployment env vars are absent (no listeners, no fetch)', async () => {
    vi.stubEnv('VITE_MOSES_CHART_ID', '')
    vi.stubEnv('VITE_MOSES_DEPLOYMENT_ID', '')
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    await installBrowserLogger()
    fireWindowError('crash with no env')
    await Promise.resolve()

    expect(fetchMock).not.toHaveBeenCalled()
  })
})
