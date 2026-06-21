import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    // React Compiler — auto-memoizes; no manual useMemo/useCallback/React.memo needed.
    react({ babel: { plugins: [['babel-plugin-react-compiler', {}]] } }),
  ],
  base: './',
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
})
