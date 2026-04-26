// CHAT-pbup: tests for getBasePath() — the meta-tag-based BASE_PATH helper.
//
// The contract: getBasePath() reads <meta name="moses-base-path"
// content="..."> at runtime, with sane fallbacks. BrowserRouter consumes
// the result as basename so internal links stay under the public prefix.
//
// Vitest + jsdom (configured in vite.config.ts).

import { describe, it, expect, beforeEach } from 'vitest'
import { getBasePath, getBaseUrl, resetBasePathCache } from './baseUrl'

beforeEach(() => {
  resetBasePathCache()
  document.head.innerHTML = ''
  // Clear any window-injected legacy value.
  delete (window as unknown as { __MOSES_BASE_URL__?: string }).__MOSES_BASE_URL__
})

function setMeta(content: string) {
  const m = document.createElement('meta')
  m.setAttribute('name', 'moses-base-path')
  m.setAttribute('content', content)
  document.head.appendChild(m)
}

describe('getBasePath', () => {
  it('returns "/" when no meta tag is set', () => {
    expect(getBasePath()).toBe('/')
  })

  it('returns "/" when meta tag still has the placeholder (standalone deploy)', () => {
    setMeta('__MOSES_BASE_PATH__')
    expect(getBasePath()).toBe('/')
  })

  it('reads the meta tag content and strips trailing slash', () => {
    setMeta('/apps/acme/frontend/')
    expect(getBasePath()).toBe('/apps/acme/frontend')
  })

  it('preserves a leading slash when the content omits it', () => {
    setMeta('apps/acme/frontend/')
    expect(getBasePath()).toBe('/apps/acme/frontend')
  })

  it('caches the result after the first read', () => {
    setMeta('/apps/acme/frontend/')
    expect(getBasePath()).toBe('/apps/acme/frontend')
    // Mutating the meta tag after the first read should not change the cached value.
    document.head.innerHTML = ''
    expect(getBasePath()).toBe('/apps/acme/frontend')
  })

  it('falls back to the legacy window.__MOSES_BASE_URL__ when no meta tag is set', () => {
    ;(window as unknown as { __MOSES_BASE_URL__?: string }).__MOSES_BASE_URL__ = '/legacy/path/'
    expect(getBasePath()).toBe('/legacy/path')
  })

  it('exports getBaseUrl as a deprecated alias', () => {
    setMeta('/apps/acme/frontend/')
    expect(getBaseUrl()).toBe(getBasePath())
  })
})
