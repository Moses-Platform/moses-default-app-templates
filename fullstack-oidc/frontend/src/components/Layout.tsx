import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import { useAuth } from '../auth/useAuth';
import './Layout.css';

/**
 * App shell: a sidebar nav plus a header that always shows the live
 * auth state, so the OIDC integration is visible on every page.
 */
export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const { phase, me, signIn, signOut } = useAuth();

  const navItems = [
    { path: '/', label: 'Overview', icon: '🔐' },
    { path: '/identity', label: 'My Identity', icon: '🧑' },
    { path: '/roles', label: 'Roles & Access', icon: '🛡️' },
    { path: '/entries', label: 'My Entries', icon: '📝' },
    { path: '/silent-sso', label: 'Silent SSO', icon: '👻' },
    { path: '/how-it-works', label: 'How It Works', icon: '📚' },
  ];

  return (
    <div className="app-layout">
      <aside className={`sidebar ${sidebarOpen ? 'open' : 'closed'}`}>
        <div className="sidebar-header">
          <h2>OIDC</h2>
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
              <span className="nav-icon" aria-hidden="true">
                {item.icon}
              </span>
              {sidebarOpen && <span className="nav-label">{item.label}</span>}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className="main-content">
        <header className="app-header">
          <div>
            <h1>Moses OIDC Relying-Party Template</h1>
            <p className="header-subtitle">
              App-owned OIDC via the vendored <code>oidcauth</code> BFF middleware
            </p>
          </div>
          <div className="auth-pill" role="status" aria-live="polite">
            {phase === 'loading' && <span className="pill pill-muted">Checking session…</span>}
            {phase === 'anonymous' && (
              <>
                <span className="pill pill-anon">Not signed in</span>
                <button className="btn btn-primary" onClick={signIn}>
                  Sign in with Moses
                </button>
              </>
            )}
            {phase === 'authenticated' && me && (
              <>
                <span className="pill pill-auth">
                  {me.name || me.username || me.email || me.subject}
                </span>
                <button className="btn btn-ghost" onClick={signOut}>
                  Sign out
                </button>
              </>
            )}
          </div>
        </header>

        <main className="content-area">
          <Outlet />
        </main>

        <footer className="app-footer">
          <p>Moses fullstack-oidc reference template · React + Go + PostgreSQL + Keycloak</p>
        </footer>
      </div>
    </div>
  );
}
