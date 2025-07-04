import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    host: true,
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
  // Ensure WASM files are served with correct MIME type
  assetsInclude: ['**/*.wasm'],
})