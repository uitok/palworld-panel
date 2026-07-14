import { defineConfig } from 'vitest/config'
import { loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const defaultBackendPort = env.VITE_DEFAULT_BACKEND_PORT || '64217'
  const apiProxyTarget = env.VITE_DEV_API_PROXY_TARGET || env.VITE_DEFAULT_BACKEND_URL || `http://127.0.0.1:${defaultBackendPort}`
  const devPort = Number(env.VITE_DEV_PORT || '63107') || 63107

  return {
    plugins: [react()],
    server: {
      port: devPort,
      proxy: {
        '/api': {
          target: apiProxyTarget,
          changeOrigin: false,
        }
      }
    },
    test: {
      environment: 'jsdom',
      setupFiles: './src/test/setup.ts',
      css: true,
	  fileParallelism: false,
	  maxWorkers: 1,
	  exclude: ['e2e/**', 'node_modules/**', 'dist/**'],
    }
  }
})
