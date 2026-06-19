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
          <h1 className="header-title">Moses Frontend Template</h1>
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
