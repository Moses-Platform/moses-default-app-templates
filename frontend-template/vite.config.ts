import { defineConfig } from 'vitest/config'
import babel from '@rolldown/plugin-babel'
import react, { reactCompilerPreset } from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    // React Compiler — auto-memoizes; no manual useMemo/useCallback/React.memo needed.
    react(),
    babel({ presets: [reactCompilerPreset()] }),
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
    maxWorkers: 2,
  },
})
