import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import { useAuth } from '../auth/useAuth';
import ThemeToggle from './ThemeToggle';
import {
  OverviewIcon,
  IdentityIcon,
  RolesIcon,
  EntriesIcon,
  SharedNotesIcon,
  SilentSSOIcon,
  HowItWorksIcon,
} from './NavIcons';
import './Layout.css';

/**
 * App shell: a sidebar nav plus a header that always shows the live
 * auth state, so the OIDC integration is visible on every page.
 */
export default function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const { phase, me, signIn, signOut } = useAuth();

  const navItems = [
    { path: '/', label: 'Overview', Icon: OverviewIcon },
    { path: '/identity', label: 'My Identity', Icon: IdentityIcon },
    { path: '/roles', label: 'Roles & Access', Icon: RolesIcon },
    { path: '/entries', label: 'My Entries', Icon: EntriesIcon },
    { path: '/shared-notes', label: 'Shared Notes', Icon: SharedNotesIcon },
    { path: '/silent-sso', label: 'Silent SSO', Icon: SilentSSOIcon },
    { path: '/how-it-works', label: 'How It Works', Icon: HowItWorksIcon },
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
            {sidebarOpen ? '▶' : '◀'}
          </button>
        </div>
        <nav className="sidebar-nav">
          {navItems.map(({ path, label, Icon }) => (
            <NavLink
              key={path}
              to={path}
              end={path === '/'}
              className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}
              onClick={() => {
                /* Tap-to-dismiss on mobile: closing on nav avoids leaving
                   the off-canvas sheet hovering over the destination page. */
                if (window.matchMedia('(max-width: 768px)').matches) {
                  setSidebarOpen(false);
                }
              }}
            >
              <span className="nav-icon">
                <Icon />
              </span>
              {sidebarOpen && <span className="nav-label">{label}</span>}
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
            <ThemeToggle />
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
