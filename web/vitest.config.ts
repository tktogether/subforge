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
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary'],
      include: ['src/**/*.ts', 'src/**/*.vue'],
      exclude: [
        'src/**/*.spec.ts',
        'src/env.d.ts',
        // 视图层（App、页面、外壳、Sf 组件）由浏览器验收保障，不做行级覆盖率门禁；
        // 逻辑层（api/composables/format/router/入口装配）维持 100% 阈值。
        'src/**/*.vue',
      ],
      thresholds: {
        lines: 100,
        functions: 100,
        branches: 80,
        statements: 100,
      },
    },
  },
})