/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The build output in web/dist is embedded into recon-server with go:embed (SPEC-026).
// The UI runs fully offline on the office LAN, so nothing may load from the internet.
export default defineConfig({
  plugins: [react()],
  // In development, the API calls go to recon-server (SPEC-004 part 6).
  server: { proxy: { '/api': 'http://127.0.0.1:8080' } },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    // Whole-app tests render the Client file with the sample engine output; under a full,
    // parallel run on an office PC they pass but can exceed the 5 s default.
    testTimeout: 20000,
  },
})
