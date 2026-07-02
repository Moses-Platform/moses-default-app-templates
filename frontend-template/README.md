# Moses Frontend Template

A production-ready React + TypeScript + Vite frontend template for building Moses platform applications with correct subpath deployment, SPA routing, and health checks.

## Features

- **Subpath Deployment** - Works seamlessly under `/apps/{tenant}/{slug}/` with relative paths
- **SPA Routing** - React Router with nginx fallback for all routes
- **Health Checks** - `/health` endpoint for Kubernetes liveness/readiness probes
- **Moses UI Standards** - Design tokens, 4px grid, accessible colors
- **Iframe Ready** - CSP `frame-ancestors` policy (from `MOSES_EMBEDDING_*` env) for embedding in Moses Apps page
- **Production Build** - Multi-stage Docker, gzip compression, optimized caching

## Technology Stack

- **React 19** - Modern UI framework, with the **React Compiler** enabled in
  `vite.config.ts` (auto-memoization — do NOT add manual
  `useMemo`/`useCallback`/`React.memo` for plain render values)
- **TypeScript** - Type safety and better developer experience
- **Vite** - Fast development server and optimized production builds
- **React Router** - Client-side routing for single-page applications
- **TanStack Query** - Server-state layer (`src/api/`): typed client + hooks + central query-key factory
- **nginx** - Production web server with health checks and caching

## Quick Start

### First step: clean out the demo

The Home/About/404 pages are demo content. Starting a real app? Run
`./clean_out_template.sh` once — it removes the demo pages, keeps all Moses
plumbing (base-path meta, router basename, data layer, nginx/entrypoint,
helm), and leaves a building, tested skeleton. Details (including a manual
KEEP/REPLACE/DELETE table) in [skills/usage.md](skills/usage.md).

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

# Run container (nginx listens on 8080 inside the container)
docker run -p 8080:8080 frontend-template:latest

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
   - Build Docker image in-cluster
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

Declaring runtime secrets — see [skills/secrets-tutorial.md](skills/secrets-tutorial.md) (frontends cannot read raw secrets — fork a fullstack template).

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

Moses design tokens live in `src/styles/theme.css` (global element styles and
utilities are in `src/App.css`):

```css
:root {
  --color-brand-primary: #34D399;     /* Brand (emerald) */
  --color-page-background: #0F0F0F;   /* Page background (dark default) */
  --spacing-4: 16px;                  /* 4px spacing grid */
}
```

Keep spacing in multiples of 4px to match Moses UI standards. Light mode is
the `[data-theme="light"]` override in the same file.

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

- `vite.config.ts`: `base: './'` (relative asset paths)
- `nginx.conf` + `entrypoint.sh`: sub-path location block rewrites
  `/apps/<t>/<a>/<rest>` → `/<rest>`, plus the SPA `try_files` fallback
- `main.tsx`: `<BrowserRouter basename={getBasePath()}>` — the basename IS
  required. The ingress does NOT strip the prefix; `getBasePath()` reads the
  `moses-base-path` meta tag that `entrypoint.sh` renders at container start
  (CHAT-pbup). Use `<Link to="/about">` for internal navigation — never raw
  absolute `<a href="/...">`, which escapes the mount.

For building app-absolute paths (sharing, etc.), join against the base path —
it comes back as `/` (root) or `/apps/<t>/<a>` (no trailing slash):

```typescript
import { getBasePath } from './utils/baseUrl'

const base = getBasePath()
const shareUrl = `${base === '/' ? '' : base}/my-page`
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

- `Content-Security-Policy: frame-ancestors ...` - Framing policy rendered at
  container start by `entrypoint.sh` from `MOSES_EMBEDDING_FRAMING` /
  `MOSES_EMBEDDING_ALLOWED_ANCESTORS` (default `moses-only`); `denied` also
  emits `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff` - Prevents MIME sniffing

`X-XSS-Protection` is deliberately NOT sent — it is deprecated/no-op in modern
browsers; CSP is the source of truth.

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
│   ├── api/                 # TanStack Query data layer
│   │   ├── client.ts        # Typed transport (relative paths only)
│   │   ├── hooks.ts         # useQuery/useMutation hooks
│   │   ├── queryClient.ts   # Shared QueryClient singleton
│   │   └── queryKeys.ts     # Central query-key factory
│   ├── components/          # Reusable UI components
│   │   ├── Layout.tsx       # Page shell (header, footer)
│   │   ├── Layout.css
│   │   └── ThemeToggle.tsx  # data-theme switcher
│   ├── pages/               # DEMO route pages (cleanout target)
│   │   ├── HomePage.tsx
│   │   ├── AboutPage.tsx
│   │   └── NotFoundPage.tsx
│   ├── styles/
│   │   └── theme.css        # Design tokens (dark default + light override)
│   ├── utils/
│   │   └── baseUrl.ts       # Moses subpath handling (getBasePath)
│   ├── App.tsx              # Route configuration
│   ├── App.css              # Global element styles + utilities
│   ├── main.tsx             # React entry point (router basename, logger)
│   ├── moses-browser-logger.ts  # Vendored browser-log reporter (do not edit)
│   └── vite-env.d.ts        # Vite type definitions
├── public/
│   └── favicon.svg          # Moses branding icon
├── helm/                    # Kubernetes deployment
├── skills/                  # Agent documentation
│   ├── usage.md             # Customization guide
│   └── secrets-tutorial.md  # Declaring runtime secrets
├── .template-clean/         # Clean twins used by clean_out_template.sh
├── clean_out_template.sh    # One-shot demo cleanout (self-deleting)
├── Dockerfile               # Multi-stage build
├── nginx.conf               # Production web server (template, rendered by entrypoint.sh)
├── entrypoint.sh            # Renders nginx.conf + base-path meta at start
├── vite.config.ts           # Build configuration (React Compiler enabled)
├── tsconfig.json            # TypeScript configuration
├── package.json             # Dependencies
├── moses-app.config.json    # Moses platform metadata
└── README.md                # This file
```

## Common Patterns

### API Integration

Server state goes through the TanStack Query layer that ships in `src/api/`
(typed client + hooks + query-key factory) — never `fetch()` from a component.
Paths MUST be relative (`'api/v1/...'`, never `'/api/...'`): the app lives at
a subpath and an absolute path escapes its mount:

```typescript
// src/api/client.ts — add a typed read
export interface Thing { id: string; name: string }
export const getThings = (signal?: AbortSignal) =>
  fetchAPI<Thing[]>('api/v1/things', signal) // relative path, no leading '/'

// src/api/hooks.ts — expose it as a hook
export function useThings() {
  return useQuery({ queryKey: queryKeys.things, queryFn: ({ signal }) => getThings(signal) })
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
