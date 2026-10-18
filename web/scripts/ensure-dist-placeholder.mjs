// 为什么需要这个脚本：vite 的 build.emptyOutDir=true 会把 outDir（../internal/api/web/dist）
// 整个删掉再重建，连占位文件 .gitkeep 一起删。而 Go 侧 //go:embed all:web/dist 要求该模式
// 至少匹配到一个文件，否则干净 clone 上 `go build ./...` 会直接报
//   pattern all:web/dist: no matching files found
// 所以每次构建后必须把占位文件补回来。详见 docs/design/vite-vuetify-coverage-plan.md §2.4。
import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const target = resolve(here, '..', '..', 'internal', 'api', 'web', 'dist', '.gitkeep')
mkdirSync(dirname(target), { recursive: true })
writeFileSync(target, '')
console.log('[ensure-dist-placeholder] 已确保 ' + target)
