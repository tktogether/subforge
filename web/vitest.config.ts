import { fileURLToPath } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./tests/setup.ts', './vitest.setup.ts'],
    include: ['src/**/*.spec.ts'],
    server: {
      deps: {
        inline: ['vuetify'],
      },
    },
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary'],
      include: ['src/**/*.ts', 'src/**/*.vue'],
      exclude: ['src/**/*.spec.ts', 'src/env.d.ts'],
      thresholds: {
        lines: 100,
        functions: 100,
        branches: 80,
        statements: 100,
      },
    },
  },
})