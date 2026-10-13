/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'prompt',
      injectRegister: false,
      manifest: {
        name: 'AI Novel',
        short_name: 'AI Novel',
        description: 'Viết truyện cùng AI, xem từng chương hiện ra.',
        lang: 'vi',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        theme_color: '#0e6e62',
        background_color: '#f4f5f3',
        icons: [
          { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: '/icons/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // Chỉ precache asset build; API và SSE là dữ liệu riêng tư nên không bao giờ qua cache.
        globPatterns: ['**/*.{js,css,html,woff2,png,svg}'],
        navigateFallback: 'index.html',
        navigateFallbackDenylist: [/^\/api\//, /^\/healthz/],
        cleanupOutdatedCaches: true,
      },
    }),
  ],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    proxy: {
      // SSE giữ kết nối lâu: tắt mọi timeout của proxy.
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: false,
        proxyTimeout: 0,
        timeout: 0,
      },
    },
  },
  build: { outDir: 'dist', emptyOutDir: true, sourcemap: false },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    css: false,
  },
})
