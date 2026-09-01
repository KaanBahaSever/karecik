import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// Karecik frontend configuration.
//
// - /api and /uploads requests are proxied to the Go backend (:8080), which
//   avoids CORS issues in the browser and lets subdomain testing
//   (demo-kafe.localhost:5173) work out of the box.
// - host: true makes *.localhost subdomains reach the dev server as well.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(process.cwd(), 'src'),
    },
  },
  server: {
    host: true,
    port: 5173,
    strictPort: false,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
})
