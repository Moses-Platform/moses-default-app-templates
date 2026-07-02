import { ReactNode } from 'react'
import ThemeToggle from './ThemeToggle'
import './Layout.css'

interface LayoutProps {
  children: ReactNode
}

function Layout({ children }: LayoutProps) {
  return (
    <div className="layout">
      <header className="layout-header">
        <div className="header-content">
          {/* UPDATE THIS — your app's name */}
          <h1 className="header-title">My App</h1>
          <ThemeToggle />
        </div>
      </header>

      {/* `content-area` scopes the App.css bento/button/form/table styles. */}
      <main className="layout-main content-area">
        {children}
      </main>

      <footer className="layout-footer">
        <p>Part of Moses Platform</p>
      </footer>
    </div>
  )
}

export default Layout
