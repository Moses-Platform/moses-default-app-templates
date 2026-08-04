import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import './styles/theme.css'
import './App.css'
import { installBrowserLogger } from './moses-browser-logger'
import { queryClient } from './api/queryClient'

// Moses browser-log reporter (BLF-B). Fire-and-forget; when the build-time
// chart_id / deployment_id env vars are absent it falls back to a
// location-derived `loc` param (the platform resolves identity server-side)
// and only no-ops if the bootstrap endpoint reports not-enabled.
void installBrowserLogger()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </React.StrictMode>,
)
