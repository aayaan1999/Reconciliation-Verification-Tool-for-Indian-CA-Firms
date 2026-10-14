/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The build output in web/dist is embedded into recon-server with go:embed (SPEC-026).
// The UI runs fully offline on the office LAN, so nothing may load from the internet.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
  },
})
