import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@wailsio/runtime': path.resolve(__dirname, 'node_modules/@wailsio/runtime'),
    },
  },
  build: {
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      input: {
        main: path.resolve(__dirname, 'index.html'),
      },
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom'],
          'highlight': ['highlight.js'],
          'mermaid': ['mermaid'],
        },
      },
    },
  },
})
