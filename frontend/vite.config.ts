import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

const apiProxyTarget = process.env.VITE_DEV_API_PROXY_TARGET || process.env.VITE_DEFAULT_BACKEND_URL || 'http://127.0.0.1:64217'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 63107,
    proxy: {
      '/api': {
        target: apiProxyTarget,
        changeOrigin: true,
      }
    }
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    css: true,
  }
})
