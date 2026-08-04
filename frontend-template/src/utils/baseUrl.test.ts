// CHAT-pbup / CHAT-t5d1u.23: tests for getBasePath() — the BASE_PATH helper.
//
// getBasePath() reads the <meta name="moses-base-path"> tag (the canonical
// Moses mount /apps/<t>/<a>) and then checks the live URL: if the app is being
// served under that mount it returns the mount; if it is served elsewhere (a
// custom-hostname root) it returns "/". BrowserRouter consumes the result as
// basename so internal links stay correct at either location.
//
// Vitest + jsdom (configured in vite.config.ts).

import { describe, it, expect, beforeEach } from 'vitest'
import { getBasePath, getBaseUrl, resetBasePathCache } from './baseUrl'

beforeEach(() => {
  resetBasePathCache()
  document.head.replaceChildren()
  // Clear any window-injected legacy value.
  delete (window as unknown as { __MOSES_BASE_URL__?: string }).__MOSES_BASE_URL__
  // Reset the jsdom URL to root between tests.
  window.history.replaceState({}, '', '/')
})

function setMeta(content: string) {
  const m = document.createElement('meta')
  m.setAttribute('name', 'moses-base-path')
  m.setAttribute('content', content)
  document.head.appendChild(m)
}

function navigateTo(path: string) {
  window.history.replaceState({}, '', path)
}

describe('getBasePath', () => {
  it('returns "/" when no meta tag is set', () => {
    expect(getBasePath()).toBe('/')
  })

  it('returns "/" when the meta tag still has the placeholder (standalone deploy)', () => {
    setMeta('__MOSES_BASE_PATH__')
    expect(getBasePath()).toBe('/')
  })

  it('returns the canonical mount when served under it', () => {
    setMeta('/apps/acme/frontend/')
    navigateTo('/apps/acme/frontend/some/route')
    expect(getBasePath()).toBe('/apps/acme/frontend')
  })

  it('returns the canonical mount when the URL is exactly the mount', () => {
    setMeta('/apps/acme/frontend/')
    navigateTo('/apps/acme/frontend')
    expect(getBasePath()).toBe('/apps/acme/frontend')
  })

  it('preserves a leading slash when the meta content omits it', () => {
    setMeta('apps/acme/frontend/')
    navigateTo('/apps/acme/frontend/')
    expect(getBasePath()).toBe('/apps/acme/frontend')
  })

  it('returns "/" when served at a custom-hostname root (URL not under the mount)', () => {
    // The meta tag still records the canonical mount, but the app is being
    // served at the root of a custom hostname — basename must be "/".
    setMeta('/apps/acme/frontend/')
    navigateTo('/some/app/route')
    expect(getBasePath()).toBe('/')
  })

  it('caches the result after the first read', () => {
    setMeta('/apps/acme/frontend/')
    navigateTo('/apps/acme/frontend/')
    expect(getBasePath()).toBe('/apps/acme/frontend')
    // Mutating the meta tag / URL after the first read must not change it.
    document.head.replaceChildren()
    navigateTo('/')
    expect(getBasePath()).toBe('/apps/acme/frontend')
  })

  it('falls back to the legacy window.__MOSES_BASE_URL__ when no meta tag is set', () => {
    ;(window as unknown as { __MOSES_BASE_URL__?: string }).__MOSES_BASE_URL__ = '/legacy/path/'
    expect(getBasePath()).toBe('/legacy/path')
  })

  it('exports getBaseUrl as a deprecated alias', () => {
    setMeta('/apps/acme/frontend/')
    navigateTo('/apps/acme/frontend/')
    expect(getBaseUrl()).toBe(getBasePath())
  })
})
