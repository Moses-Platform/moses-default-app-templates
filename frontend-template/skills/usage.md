# Frontend Template - Agent Customization Guide

## What This Template Provides

A production-ready React + TypeScript + Vite frontend application with:

- Correct subpath deployment configuration (`base: './'`)
- SPA routing with React Router
- nginx production server with health checks
- Moses UI design tokens and 4px spacing grid
- Multi-stage Docker build with optimization
- Helm chart for Kubernetes deployment

## File Structure

```
frontend-template/
├── src/
│   ├── components/     # Reusable UI components
│   │   └── Layout.tsx  # Page shell (header, footer)
│   ├── pages/          # Route pages
│   │   ├── HomePage.tsx
│   │   ├── AboutPage.tsx
│   │   └── NotFoundPage.tsx
│   ├── utils/          # Helper functions
│   │   └── baseUrl.ts  # Moses subpath handling
│   ├── App.tsx         # Route configuration
│   ├── App.css         # Global styles + design tokens
│   └── main.tsx        # React entry point
├── public/
│   └── favicon.svg     # Moses branding icon
├── helm/               # Kubernetes deployment
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── Dockerfile          # Multi-stage build
├── nginx.conf          # Production web server
├── vite.config.ts      # Build configuration (CRITICAL: base: './')
├── package.json        # Dependencies
└── moses-app.config.json  # Moses platform metadata

## How to Customize

### 1. Update App Metadata

Edit `moses-app.config.json`:

```json
{
  "name": "my-app",
  "displayName": "My Application",
  "description": "Brief description of what your app does"
}
```

### 2. Add New Pages

Create a new page in `src/pages/`:

```typescript
// src/pages/MyPage.tsx
import './MyPage.css'

function MyPage() {
  return (
    <div className="my-page">
      <h1>My New Page</h1>
      <p>Content goes here</p>
    </div>
  )
}

export default MyPage
```

Add route in `src/App.tsx`:

```typescript
import MyPage from './pages/MyPage'

// Add to Routes:
<Route path="/my-page" element={<MyPage />} />
```

### 3. Customize Design Tokens

Moses design tokens are in `src/App.css`:

```css
:root {
  --color-primary: #4F46E5;      /* Change primary color */
  --color-bg: #F9FAFB;           /* Background color */
  --color-surface: #FFFFFF;      /* Card/panel backgrounds */
  --color-text: #111827;         /* Main text */
  --spacing-4: 16px;             /* 4px spacing grid */
}
```

**IMPORTANT**: Keep spacing in multiples of 4px to match Moses UI standards.

### 4. Add Components

Create reusable components in `src/components/`:

```typescript
// src/components/Button.tsx
import './Button.css'

interface ButtonProps {
  label: string
  onClick: () => void
}

function Button({ label, onClick }: ButtonProps) {
  return (
    <button className="moses-button" onClick={onClick}>
      {label}
    </button>
  )
}

export default Button
```

### 5. Subpath Deployment

The app is pre-configured for Moses subpath deployment (`/apps/{tenant}/{slug}/`).

Vite config already has `base: './'` - DO NOT change this.

For absolute URLs (sharing, etc.), use the baseUrl utility:

```typescript
import { getBaseUrl } from './utils/baseUrl'

const shareUrl = `${getBaseUrl()}my-page`
```

### 6. Health Checks

The nginx config provides `/health` endpoint for Kubernetes probes.

If you add an API backend, implement health checks:

```typescript
// Example Express.js
app.get('/health', (req, res) => {
  res.status(200).send('healthy')
})
```

### 7. Building

Local development:

```bash
npm install
npm run dev
# Visit http://localhost:5173
```

Production build:

```bash
npm run build
# Creates optimized dist/ directory
```

Docker build:

```bash
docker build -t my-app:latest .
```

### 8. Deployment to Moses

Once customized:

1. Commit changes to Git repository
2. Register with Moses via the Apps page
3. Agents will build and deploy automatically
4. Access at `https://{domain}/apps/{tenant}/{slug}/`

## Moses Integration Points

### Design System

Use Moses CSS variables for consistency:

- Typography: Follow `MOSES_UI_UX_STANDARDS.md` font scale
- Colors: Semantic color palette for primary, success, warning, error
- Spacing: 4px grid system (`var(--spacing-4)`, etc.)
- Accessibility: WCAG 2.1 AA contrast ratios

### Security Headers

nginx.conf includes:

- `X-Frame-Options: SAMEORIGIN` - Allows iframe in Moses Apps page
- `X-Content-Type-Options: nosniff` - Prevents MIME type sniffing
- `X-XSS-Protection: 1; mode=block` - XSS protection

### Caching Strategy

Static assets cached for 1 year (`expires 1y`).

HTML files served with no-cache to ensure updates.

### Resource Limits

Helm chart sets resource requests/limits:

- CPU: 100m request, 200m limit
- Memory: 128Mi request, 256Mi limit

Adjust in `helm/values.yaml` if needed (stay within namespace quota).

## Common Patterns

### API Integration

Add API calls in services:

```typescript
// src/services/api.ts
const API_BASE = import.meta.env.VITE_API_URL || '/api'

export async function fetchData() {
  const response = await fetch(`${API_BASE}/data`)
  return response.json()
}
```

### State Management

For complex state, add Zustand:

```bash
npm install zustand
```

```typescript
// src/stores/appStore.ts
import { create } from 'zustand'

interface AppState {
  count: number
  increment: () => void
}

export const useAppStore = create<AppState>((set) => ({
  count: 0,
  increment: () => set((state) => ({ count: state.count + 1 })),
}))
```

### Forms

Use controlled components:

```typescript
const [value, setValue] = useState('')

<input
  value={value}
  onChange={(e) => setValue(e.target.value)}
  className="form-input"
/>
```

## Troubleshooting

**Issue**: Routes return 404 in production

**Fix**: nginx.conf has `try_files $uri $uri/ /index.html` - all routes fall back to index.html for SPA routing.

---

**Issue**: Assets fail to load with 404

**Fix**: Ensure `vite.config.ts` has `base: './'` (relative paths).

---

**Issue**: App not accessible after deployment

**Fix**: Check health endpoint responds at `/health`. Kubernetes probes will fail if missing.

---

**Issue**: Styles not applying

**Fix**: Import CSS files in component: `import './Component.css'`

## Next Steps

1. Customize `moses-app.config.json` with your app details
2. Update design tokens in `src/App.css`
3. Add your pages to `src/pages/`
4. Build and test locally
5. Deploy via Moses platform

This template is production-ready. Customize it to fit your application's needs while maintaining Moses platform integration.
