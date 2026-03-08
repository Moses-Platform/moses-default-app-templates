import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
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
            >
              <span className="nav-icon">{item.icon}</span>
              {sidebarOpen && <span className="nav-label">{item.label}</span>}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className="main-content">
        <header className="app-header">
          <h1>Moses Platform Showcase</h1>
          <p className="header-subtitle">Interactive demonstration of platform capabilities</p>
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
