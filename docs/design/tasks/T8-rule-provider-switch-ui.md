# T8 规则页：规则集（providers）行内启停开关（F9 前端半）

## WHY（为什么）
T7 已交付 PUT /api/rule-providers/{id}/enabled。本卡把规则页的规则集列表（现在只显示「· 启用/禁用」文字，RulesPage.vue:183）变成可切换的开关。

## 前置：T7 必须已完成（卡 blocked_by T7）
若 T7 的接口还没合并（GET /api/rule-providers 之外的 PUT 404/405），停下报告 lead。

## 心跳（4 时点 send_message 给 lead）
1. 任务书读完 + 锚点校对完
2. 改动落盘
3. 自测开始
4. 全部完成 + 全量命令输出原文

## 红线
- 只许动：web/src/api/endpoints.ts、web/src/pages/RulesPage.vue、web/src/pages/RulesPage.spec.ts
- 不许 git commit / 不许建卡

## 第 0 步：锚点校对
- web/src/pages/RulesPage.vue:183  <v-list-item v-for="p in providers" :key="p.id" :title="p.name" :subtitle="p.source + (p.enabled ? ' · 启用' : ' · 禁用')">
- web/src/pages/RulesPage.vue:176  providers 列表所在的 AsyncSection
- web/src/pages/RulesPage.vue:23-31  loadProviders 的实现（enabled 过滤参数）
- web/src/api/endpoints.ts:406-407  listRuleProviders 封装
- web/src/pages/RulesPage.spec.ts:15  listRuleProviders: vi.fn()（mock 注入方式）
对不上 → 停下报告 lead。

## 第 1 步：endpoints.ts 加封装
紧跟 :407 listRuleProviders 之后加：

```ts
  updateRuleProvider: (id: number, enabled: boolean): Promise<OkResponse> =>
    api.put<OkResponse>('/api/rule-providers/' + id + '/enabled', { enabled }),
```
（api.put 的签名与 OkResponse 的用法先看 :193-195 与相邻封装，照抄风格。）

## 第 2 步：RulesPage.vue 行内开关
:183 的 v-list-item 加 append 插槽（title/subtitle 保留，去掉 subtitle 里的启用文字可以保留——保留也行，开关本身就是状态）：

```vue
            <v-list-item v-for="p in providers" :key="p.id" :title="p.name" :subtitle="p.source + (p.enabled ? ' · 启用' : ' · 禁用')">
              <template #append>
                <v-switch :model-value="p.enabled" density="compact" :hide-details="true"
                  data-testid="provider-switch" :loading="togglingProvider === p.id"
                  @update:model-value="toggleProvider(p, $event as boolean)"></v-switch>
              </template>
            </v-list-item>
```
（注意 :183 原本是自闭合的单行写法还是带子内容的写法，以实际为准展开成带 #append 的形式。）
script 加：

```ts
const togglingProvider = ref<number | null>(null)

async function toggleProvider(p: RuleProvider, value: boolean): Promise<void> {
  togglingProvider.value = p.id
  try {
    await endpoints.updateRuleProvider(p.id, value)
    p.enabled = value
  } catch (err) {
    notify.error(errorMessage(err))
  } finally {
    togglingProvider.value = null
  }
}
```
- notify / errorMessage 的 import：看 RulesPage.vue 头部是否已有（其他页面有 from '../composables/useNotify'），没有就补。
- RuleProvider 类型 import 已在文件里（:19 用到），核对。

## 第 3 步：spec（RulesPage.spec.ts 追加，mock 注入照抄 :15 的方式）
- updateRuleProvider: vi.fn() 加进 endpoints mock。
- 用例 A「provider-switch 切换调 API」：mock updateRuleProvider resolve；触发第一个 provider-switch 的 update:modelValue(false)；断言 updateRuleProvider 被调 (1, false)。
- 用例 B「失败回滚」：mock updateRuleProvider reject；触发切换后，该 provider 的开关 model-value 仍是原值（p.enabled 不变）。

## 第 4 步：自测（原样贴输出给 lead）
```
pnpm -C web run typecheck
pnpm -C web run test -- --run web/src/pages/RulesPage.spec.ts
pnpm -C web run test -- --run
```
预期：全绿。
（pnpm 不在 PATH 时用 node C:\Users\橙先生\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\pnpm\bin\pnpm.mjs -C web run ...）

## 完成定义
- typecheck + 全量测试绿（输出原文贴 lead）
- git status 只含本卡三个文件
