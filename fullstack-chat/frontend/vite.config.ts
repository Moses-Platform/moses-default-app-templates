import { defineConfig } from 'vitest/config';
import babel from '@rolldown/plugin-babel';
import react, { reactCompilerPreset } from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [
    // React Compiler — auto-memoizes; no manual useMemo/useCallback/React.memo needed.
    react(),
    babel({ presets: [reactCompilerPreset()] }),
  ],
  base: './',
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    maxWorkers: 2,
  },
});
