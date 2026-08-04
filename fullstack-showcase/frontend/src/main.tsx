import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import './styles/theme.css';
import './App.css';
import { QueryClientProvider } from '@tanstack/react-query';
import { installBrowserLogger } from './moses-browser-logger';
import { getBasePath } from './utils/baseUrl';
import { queryClient } from './api/queryClient';

// Moses browser-log reporter (BLF-B). Fire-and-forget; silent no-op when
// the platform's build-time chart_id / deployment_id env vars are absent.
void installBrowserLogger();

// CHAT-pbup: BrowserRouter basename reads from the moses-base-path meta tag
// so internal links stay under /apps/<t>/<a>/ instead of escaping to the
// platform router.
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter basename={getBasePath()}>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
);
