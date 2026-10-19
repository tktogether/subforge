# T4 · 订阅令牌：pipeline / config 改下拉（接 T5 的 GET /api/configs）

## 0. 你是谁、红线是什么
你是 SubForge 前端实施队员。三条红线：
1. 只允许修改这两个文件：web/src/api/endpoints.ts、web/src/pages/SubscriptionsPage.vue（如 spec 需要补 mock，则加第三个：web/src/pages/SubscriptionsPage.spec.ts）。
2. 不碰 CSS；不碰其他页面。
3. 不许 git commit / git add；不许新建任务卡。

## 1. 进度心跳（必须执行）
send_message 发给 lead，共 4 次：心跳1「T4 book read, starting」/ 心跳2「T4 skeleton written」/ 心跳3「T4 self-test starting」/ 心跳4「T4 done」+ 输出原文。

## 2. WHY
令牌对话框里 pipeline 和 config 是手填文本框（用户原话：per-token 操作就不能走下拉吗，非得人手填）。target 已经是下拉（loadTargets 模式），照着它补两个下拉。

## 3. 前置（已就绪）
- GET /api/pipelines → Pipeline[]（endpoints.ts:357 已有 endpoints.pipelines(config)；Pipeline 接口在 endpoints.ts 里，打开核对字段名，列表项名称字段应为 name）。
- GET /api/configs → {configs: string[], default: string}（T5 已交付；本任务在 endpoints.ts 补封装）。

## 4. 逐步实施

### 步骤 1：endpoints.ts 追加（configs）——找到 pipelines 声明（:357）旁边，同风格：
```ts
  configs: (): Promise<{ configs: string[]; default: string }> =>
    api.get<{ configs: string[]; default: string }>('/api/configs'),
```

### 步骤 2：SubscriptionsPage.vue script
(a) 现有 targetOptions/loadingTargets 旁边追加两组 state：
```ts
const pipelineOptions = ref<string[]>([])
const loadingPipelines = ref(false)
const configOptions = ref<string[]>([])
const loadingConfigs = ref(false)
```
(b) 照抄 loadTargets（:62-73 附近）写两个加载器（失败退化空数组，不阻塞表单）：
```ts
async function loadPipelineOptions() {
  loadingPipelines.value = true
  try {
    const list = await endpoints.pipelines('')
    pipelineOptions.value = list.map((p: { name: string }) => p.name)
  } catch {
    pipelineOptions.value = []
  } finally {
    loadingPipelines.value = false
  }
}

async function loadConfigOptions() {
  loadingConfigs.value = true
  try {
    const r = await endpoints.configs()
    configOptions.value = r.configs ?? []
  } catch {
    configOptions.value = []
  } finally {
    loadingConfigs.value = false
  }
}
```
(c) openCreateToken 与 openEditToken 两个函数末尾各加：
```ts
  void loadPipelineOptions()
  void loadConfigOptions()
```

### 步骤 3：SubscriptionsPage.vue template（:361-362 两个 v-text-field）
把 pipeline 与 config 两个 v-text-field 换成 v-combobox（可选可填：config 无封闭集合、也允许远程 URL，combobox 是唯一兼顾小白的形态；clearable 支持清空=用默认）：
```html
<v-combobox
  data-testid="token-pipeline"
  v-model="tokenForm.pipeline"
  :items="pipelineOptions"
  :loading="loadingPipelines"
  label="处理管线"
  clearable
  persistent-hint
  hint="留空用默认；可从下拉选模板里定义的命名管线"
/>
<v-combobox
  data-testid="token-config"
  v-model="tokenForm.config"
  :items="configOptions"
  :loading="loadingConfigs"
  label="配置模板"
  clearable
  persistent-hint
  hint="留空用默认；可下拉选模板推荐的，也可手填远程 URL"
/>
```

### 步骤 4：清空语义防 null
v-combobox 点清除会把 v-model 置 null，而 saveToken 会把值直接发给 PUT（textOrEmpty 语义：空串=清空，缺失=不改）。找到 saveToken（:99 附近）里组装 payload 的位置，把 pipeline/config 两个字段包一层兜底：
```ts
pipeline: textOrEmpty((tokenForm.pipeline as string) ?? ''),
config: textOrEmpty((tokenForm.config as string) ?? ''),
```
校对：先看 saveToken 现有 payload 写法（textOrEmpty 的真实调用形态），保持一致，只加 ?? '' 兜底。

### 步骤 5：SubscriptionsPage.spec.ts 补 mock
(a) vi.mock 工厂（:8-24）的 endpoints 对象里追加：
```ts
      pipelines: vi.fn(),
      configs: vi.fn(),
```
(b) 新增一个测试（describe 内）：
```ts
import { endpoints as ep } from '../api/endpoints'  // 若文件头已 import endpoints 就复用

it('打开令牌对话框后加载 pipeline/config 候选', async () => {
  ;(ep.pipelines as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([{ name: 'clean' }])
  ;(ep.configs as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ configs: ['http://x/a.ini'], default: 'http://x/a.ini' })
  const w = mount(SubscriptionsPage, { global: { plugins: [vuetify] } })
  await flushPromises()
  ;(w.vm as any).openCreateToken()
  await flushPromises()
  expect((ep.pipelines as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalled()
  expect((ep.configs as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalled()
  expect((w.vm as any).pipelineOptions).toEqual(['clean'])
  expect((w.vm as any).configOptions).toEqual(['http://x/a.ini'])
  w.unmount()
})
```
校对：该 spec 的挂载方式/vuetify 引入照抄文件头现有写法（:30 import { vuetify } from '../plugins/vuetify'）；若 beforeEach 里已有统一 mock 默认值就跟着补默认值，别重复。

## 5. 自测命令（repo 根逐条跑，全部贴原文）
```
pnpm -C web run typecheck
pnpm -C web run test -- --run
pnpm -C web run build
```

## 6. 完成报告格式（心跳4 一起发）
- 改动文件列表 + 三条命令输出原文 + Pipeline 字段名核对结果 + saveToken 兜底写法说明