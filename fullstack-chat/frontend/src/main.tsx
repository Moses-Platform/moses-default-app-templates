import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import App from './App';
import './styles/theme.css';
import './App.css';
import { queryClient } from './api/queryClient';
import { installBrowserLogger } from './moses-browser-logger';

// Moses browser-log reporter. Fire-and-forget; silent no-op when the
// platform's build-time chart_id / deployment_id env vars are absent.
void installBrowserLogger();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </React.StrictMode>,
);
