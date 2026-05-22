import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './App.css';
import { installBrowserLogger } from './moses-browser-logger';

// Moses browser-log reporter (BLF-B). Fire-and-forget; silent no-op when
// the build-time chart_id / deployment_id env vars are absent.
void installBrowserLogger();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
