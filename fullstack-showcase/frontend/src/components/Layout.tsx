import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import ThemeToggle from './ThemeToggle';
import './Layout.css';

export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const navItems = [
    { path: '/', label: 'Overview', icon: '🏠' },
    { path: '/mcp-tools', label: 'MCP Tools', icon: '🔧' },
    { path: '/deployment', label: 'Deployment', icon: '🚀' },
    { path: '/auth-flow', label: 'Auth Flow', icon: '🔐' },
    { path: '/multi-tenancy', label: 'Multi-Tenancy', icon: '👥' },
    { path: '/api-examples', label: 'API Examples', icon: '💻' },
    { path: '/platform-users', label: 'Platform Users', icon: '🪪' },
  ];

  return (
    <div className="app-layout">
      <aside className={`sidebar ${sidebarOpen ? 'open' : 'closed'}`}>
        <div className="sidebar-header">
          <h2>Moses</h2>
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
          <h1>Moses Platform Showcase</h1>
          <p className="header-subtitle">Interactive demonstration of platform capabilities</p>
          <ThemeToggle />
        </header>

        <main className="content-area">
          <Outlet />
        </main>

        <footer className="app-footer">
          <p>Moses Platform © 2024 | Built with React + Go + Kubernetes</p>
        </footer>
      </div>
    </div>
  );
}
