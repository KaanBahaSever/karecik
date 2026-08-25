import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// Karecik frontend yapılandırması.
//
// - /api ve /uploads istekleri Go backend'ine (8080) yönlendirilir.
//   Böylece tarayıcı tarafında CORS derdi olmaz ve subdomain testleri
//   (demo-kafe.localhost:5173) sorunsuz çalışır.
// - host: true => *.localhost alt alan adları da sunucuya ulaşır.
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
