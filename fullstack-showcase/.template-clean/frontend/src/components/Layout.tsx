import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import ThemeToggle from './ThemeToggle';
import './Layout.css';

// Minimal app shell: collapsible sidebar (mobile off-canvas + scrim), header
// with theme toggle, routed content area. Add nav entries alongside your
// routes in App.tsx — every route needs a nav entry or a deep link, or it is
// unreachable.
export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const navItems = [
    { path: '/', label: 'Home', icon: '🏠' },
  ];

  return (
    <div className="app-layout">
      <aside className={`sidebar ${sidebarOpen ? 'open' : 'closed'}`}>
        <div className="sidebar-header">
          <h2>My App</h2>
          <button
            className="sidebar-toggle"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label="Toggle sidebar"
          >
            {sidebarOpen ? '◀' : '▶'}
          </button>
        </div>
        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              end={item.path === '/'}
              className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
              onClick={() => {
                /* Tap-to-dismiss on mobile: closing on nav avoids leaving
                   the off-canvas sheet hovering over the destination page. */
                if (window.matchMedia('(max-width: 768px)').matches) {
                  setSidebarOpen(false);
                }
              }}
            >
              <span className="nav-icon">{item.icon}</span>
              {sidebarOpen && <span className="nav-label">{item.label}</span>}
            </NavLink>
          ))}
        </nav>
      </aside>

      {/* Mobile-only scrim — dismiss the off-canvas sheet on outside tap. */}
      {sidebarOpen && (
        <div
          className="sidebar-overlay"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      <div className="main-content">
        <header className="app-header">
          <button
            className="mobile-menu-toggle"
            type="button"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label={sidebarOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={sidebarOpen}
          >
            {sidebarOpen ? '✕' : '☰'}
          </button>
          <h1>My App</h1>
          <ThemeToggle />
        </header>

        <main className="content-area">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
