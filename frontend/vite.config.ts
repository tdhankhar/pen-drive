import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig(({ mode }) => {
  const rootDir = new URL('.', import.meta.url).pathname
  const env = loadEnv(mode, rootDir, '')
  const apiTarget = env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8080'
  const appBaseUrl = env.VITE_BASE_URL ?? 'http://127.0.0.1:5173'
  const appUrl = new URL(appBaseUrl)

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': new URL('./src', import.meta.url).pathname,
      },
    },
    server: {
      host: appUrl.hostname,
      port: Number(appUrl.port || 5173),
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
  }
})
