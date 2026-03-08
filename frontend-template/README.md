# Moses Frontend Template

A production-ready React + TypeScript + Vite frontend template for building Moses platform applications with correct subpath deployment, SPA routing, and health checks.

## Features

- **Subpath Deployment** - Works seamlessly under `/apps/{tenant}/{slug}/` with relative paths
- **SPA Routing** - React Router with nginx fallback for all routes
- **Health Checks** - `/health` endpoint for Kubernetes liveness/readiness probes
- **Moses UI Standards** - Design tokens, 4px grid, accessible colors
- **Iframe Ready** - SAMEORIGIN headers for embedding in Moses Apps page
- **Production Build** - Multi-stage Docker, gzip compression, optimized caching

## Technology Stack

- **React 18** - Modern UI framework with hooks and concurrent features
- **TypeScript** - Type safety and better developer experience
- **Vite** - Fast development server and optimized production builds
- **React Router** - Client-side routing for single-page applications
- **nginx** - Production web server with health checks and caching

## Quick Start

### Local Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Visit http://localhost:5173
```

### Build

```bash
# TypeScript + Vite build
npm run build

# Preview production build locally
npm run preview
```

### Docker Build

```bash
# Build Docker image
docker build -t frontend-template:latest .

# Run container
docker run -p 8080:80 frontend-template:latest

# Visit http://localhost:8080
```

## Deploy to Moses

### Option 1: Via Moses Platform UI

1. Push your code to a Git repository
2. Navigate to Moses Apps page
3. Click "Register New App"
4. Provide Git URL
5. Moses will automatically:
   - Clone repository
   - Build Docker image with Kaniko
   - Deploy with Helm to Kubernetes
   - Make app available at `/apps/{tenant}/{slug}/`

### Option 2: Manual Helm Deployment

```bash
# Update helm/values.yaml with your image
image:
  repository: your-registry/frontend-template
  tag: v1.0.0

# Deploy with Helm
helm install my-app ./helm \
  --namespace your-namespace \
  --set moses.namespace=your-namespace \
  --set moses.tenantId=your-tenant-id \
  --set moses.executionId=your-execution-id
```

## Customization Guide

### Update App Metadata

Edit `moses-app.config.json`:

```json
{
  "name": "my-app",
  "displayName": "My Application",
  "description": "Brief description of what your app does",
  "quickActions": [
    {
      "id": "launch",
      "label": "Launch App",
      "icon": "rocket",
      "url": "/",
      "description": "Open the app"
    }
  ]
}
```

### Add New Pages

1. Create page in `src/pages/MyPage.tsx`
2. Add route in `src/App.tsx`:

```typescript
import MyPage from './pages/MyPage'

// In <Routes>:
<Route path="/my-page" element={<MyPage />} />
```

### Customize Design

Moses design tokens in `src/App.css`:

```css
:root {
  --color-primary: #4F46E5;      /* Change primary color */
  --color-bg: #F9FAFB;           /* Background color */
  --spacing-4: 16px;             /* 4px spacing grid */
}
```

Keep spacing in multiples of 4px to match Moses UI standards.

### Add Components

Create reusable components in `src/components/`:

```typescript
// src/components/Button.tsx
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
```

## Moses Integration Points

### Subpath Deployment

The app is configured for subpath deployment via:

- `vite.config.ts`: `base: './'` (relative paths)
- `nginx.conf`: `try_files $uri $uri/ /index.html` (SPA fallback)
- No `basename` needed in React Router (Moses strips prefix)

For absolute URLs (sharing, etc.), use the baseUrl utility:

```typescript
import { getBaseUrl } from './utils/baseUrl'

const shareUrl = `${getBaseUrl()}my-page`
```

### Health Checks

The `/health` endpoint is required for Kubernetes probes:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: http
  initialDelaySeconds: 15
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: http
  initialDelaySeconds: 5
  periodSeconds: 5
```

nginx serves this automatically. No changes needed unless adding API backend.

### Security Headers

nginx includes:

- `X-Frame-Options: SAMEORIGIN` - Allows embedding in Moses
- `X-Content-Type-Options: nosniff` - Prevents MIME sniffing
- `X-XSS-Protection: 1; mode=block` - XSS protection

### Resource Limits

Helm chart sets Kubernetes resource constraints:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 200m
    memory: 256Mi
```

Adjust in `helm/values.yaml` if needed (stay within namespace quota).

## File Structure

```
frontend-template/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── Layout.tsx       # Page shell (header, footer)
│   │   └── Layout.css
│   ├── pages/               # Route pages
│   │   ├── HomePage.tsx
│   │   ├── AboutPage.tsx
│   │   └── NotFoundPage.tsx
│   ├── utils/               # Helper functions
│   │   └── baseUrl.ts       # Moses subpath handling
│   ├── App.tsx              # Route configuration
│   ├── App.css              # Global styles + design tokens
│   ├── main.tsx             # React entry point
│   └── vite-env.d.ts        # Vite type definitions
├── public/
│   └── favicon.svg          # Moses branding icon
├── helm/                    # Kubernetes deployment
│   ├── Chart.yaml           # Helm chart metadata
│   ├── values.yaml          # Deployment configuration
│   └── templates/           # Kubernetes resources
│       ├── deployment.yaml
│       ├── service.yaml
│       └── _helpers.tpl
├── skills/                  # Agent documentation
│   └── usage.md             # Customization guide
├── Dockerfile               # Multi-stage build
├── nginx.conf               # Production web server
├── vite.config.ts           # Build configuration
├── tsconfig.json            # TypeScript configuration
├── package.json             # Dependencies
├── moses-app.config.json    # Moses platform metadata
└── README.md                # This file
```

## Common Patterns

### API Integration

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

## Troubleshooting

**Routes return 404 in production**

nginx.conf has `try_files $uri $uri/ /index.html` for SPA routing. All routes fall back to index.html.

**Assets fail to load with 404**

Ensure `vite.config.ts` has `base: './'` for relative paths.

**App not accessible after deployment**

Check `/health` endpoint responds. Kubernetes probes will fail if missing.

**Styles not applying**

Import CSS files in components: `import './Component.css'`

## Documentation

- **Moses Architecture**: See `/arch.md` in main repository
- **UI/UX Standards**: See `/coding-standards/MOSES_UI_UX_STANDARDS.md`
- **Backend Standards**: See `/coding-standards/MOSES_BACKEND_STANDARDS.md`
- **Agent Customization**: See `/skills/usage.md` in this template

## License

Part of the Moses platform. See main repository for license details.

## Support

For issues or questions:

1. Check Moses platform documentation
2. Review existing apps in `default-apps/` directory
3. Consult Moses coding standards
4. Contact Moses platform team

---

This template is production-ready. Customize it to fit your application's needs while maintaining Moses platform integration.
