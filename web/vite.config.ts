import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api/v1': 'http://127.0.0.1:8080',
    },
  },
	test: {
		environment: 'jsdom',
		setupFiles: './src/test/setup.ts',
		include: ['src/**/*.test.{ts,tsx}'],
		css: true,
  },
})
