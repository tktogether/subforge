import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// 构建产物直接落到 Go 的 embed 目录，避免多一步拷贝。
// 目录名固定为 internal/api/web/dist（根目录的 /dist/ 是历史测试产物，已在 .gitignore）。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    outDir: fileURLToPath(new URL('../internal/api/web/dist', import.meta.url)),
    // emptyOutDir=true 会连 internal/api/web/dist/.gitkeep 一起删，导致 go:embed 失败；
    // 构建脚本尾部的 scripts/ensure-dist-placeholder.mjs 负责把它补回来。不要改成 false。
    emptyOutDir: true,
    // 单二进制内嵌：不生成 sourcemap，减少体积。
    sourcemap: false,
    chunkSizeWarningLimit: 1500,
  },
  server: {
    port: 5173,
    proxy: {
      // 开发态把后端接口转给 go run ./cmd/subforge serve
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/sub': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/subscribe': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/version': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/healthz': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/getruleset': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/inspect': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
});
