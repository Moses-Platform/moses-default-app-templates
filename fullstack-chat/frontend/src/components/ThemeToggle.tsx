import { useEffect, useState } from 'react';

type Theme = 'dark' | 'light';

function currentTheme(): Theme {
  if (typeof document !== 'undefined') {
    const attr = document.documentElement.getAttribute('data-theme');
    if (attr === 'light' || attr === 'dark') return attr;
  }
  // matchMedia is absent in jsdom/SSR — guard it.
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  }
  return 'dark';
}

// Dark/light toggle. Persists to localStorage and reflects onto <html data-theme>;
// the no-flash init script in index.html sets the initial attribute before paint.
// (This useEffect is a DOM side-effect, not data fetching — the canonical no-
// useEffect-for-data rule does not apply.)
export default function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(currentTheme);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    try {
      localStorage.setItem('moses-theme', theme);
    } catch {
      /* private mode / disabled storage — ignore */
    }
  }, [theme]);

  const next: Theme = theme === 'dark' ? 'light' : 'dark';
  return (
    <button
      type="button"
      className="theme-toggle"
      onClick={() => setTheme(next)}
      aria-label={`Switch to ${next} mode`}
      title={`Switch to ${next} mode`}
    >
      <span aria-hidden="true">{theme === 'dark' ? '☀' : '☾'}</span>
    </button>
  );
}
