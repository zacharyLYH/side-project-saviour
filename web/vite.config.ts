import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// SPS_SERVER_URL points the dev proxy at the Go server: set to
// http://server:8080 inside docker-compose.dev.yml, default localhost.
const serverUrl = process.env.SPS_SERVER_URL ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': { target: serverUrl, changeOrigin: true },
      '/ws': { target: serverUrl, ws: true },
    },
  },
})
