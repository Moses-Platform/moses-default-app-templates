import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [
    // React Compiler — auto-memoizes; no manual useMemo/useCallback/React.memo needed.
    react({ babel: { plugins: [['babel-plugin-react-compiler', {}]] } }),
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
  },
});
