# T9 机场页：withRules 语义提示（F6）+ 启用列行内开关（F15）

## WHY（为什么）
- F6：工具行「刷新时同时导入规则」开关（AirportsPage.vue:252）没解释它和行内「导入规则」按钮的关系，新手不知道勾了会发生什么。
- F15：机场「启用」列只是圆点+文字（AirportsPage.vue:264-267），要启停必须开编辑对话框。改成行内开关直达。

## 心跳（4 时点 send_message 给 lead）
1. 任务书读完 + 锚点校对完
2. 骨架落盘（help 文案 + toggleEnabled 空壳）
3. 自测开始（跑 pnpm test 前）
4. 全部完成 + 全量命令输出原文

## 红线
- 只许动：web/src/content/help.ts、web/src/pages/AirportsPage.vue、web/src/pages/AirportsPage.spec.ts
- 不许 git commit / 不许建卡 / 不许动其他页面
- 颜色只许 var(--sf-*)（本项目红线，本卡其实不需要新颜色）

## 第 0 步：锚点校对
- web/src/pages/AirportsPage.vue:252  <v-switch v-model="withRules" data-testid="with-rules" label="刷新时同时导入规则" density="compact" :hide-details="true"></v-switch>
- web/src/pages/AirportsPage.vue:264-267  enabled 列 slot：sf-status__dot 圆点 + 启用/停用文字
- web/src/pages/AirportsPage.vue:66  const withRules = ref(false)
- web/src/pages/AirportsPage.vue:12  import { pageHelp, fieldHelp } from '../content/help'
- web/src/pages/AirportsPage.vue:61  const draft = ref<AirportInput>({ name: '', subscription_url: '', remarks: '', enabled: true })
- web/src/api/endpoints.ts  搜 updateAirport——核对它的签名（参数是 (id, body: AirportInput) 形态；AirportInput 见 endpoints.ts 的 export interface AirportInput）
- web/src/content/help.ts  搜 'airport.name' 看 fieldHelp 的条目形态
对不上下任何一条 → 停下报告 lead。

## 第 1 步：help.ts 加文案
找到机场页 fieldHelp 条目聚集处（搜 'airport.name'），在旁边加：

```ts
  'airport.with_rules': {
    hint: '勾选后，「全部刷新」会顺带把每个启用机场的规则与规则集导入「规则」页；不勾则只刷新节点与流量。想单独导入某个机场，用表格行内的「导入规则」按钮。',
    example: '',
  },
```
（条目字段形态照抄相邻条目——如果相邻条目没有 example 字段就也不要加。）

## 第 2 步：AirportsPage.vue 工具行开关加 hint
:252 改为（去掉 hide-details，加 hint）：

```vue
        <v-switch v-model="withRules" data-testid="with-rules" label="刷新时同时导入规则" density="compact"
          :hint="fieldHelp('airport.with_rules')?.hint ?? ''" persistent-hint class="mb-2"></v-switch>
```

## 第 3 步：启用列改行内开关
script 部分（withRules 定义 :66 附近）加：

```ts
const toggling = ref<number | null>(null)

async function toggleEnabled(row: AirportRow, value: boolean): Promise<void> {
  toggling.value = row.id
  try {
    await endpoints.updateAirport(row.id, {
      name: row.name,
      subscription_url: row.subscription_url,
      remarks: row.remarks,
      enabled: value,
    })
    row.enabled = value
    notify.success(value ? '已启用 ' + row.name : '已停用 ' + row.name)
  } catch (err) {
    notify.error(errorMessage(err))
  } finally {
    toggling.value = null
  }
}
```
注意：
- updateAirport 的真名与参数顺序以第 0 步在 endpoints.ts 核对的为准（可能是 updateAirport(id, body) 也可能叫别的）。
- notify / errorMessage 已在 :10-11 import，直接用。
- AirportRow.id 字段名以 :17-25 的类型定义为准；若 updateAirport 需要的字段更多（比如 enabled 之外还有必填），按 endpoints.ts 的 AirportInput 定义补齐。

template :264-267 的 enabled slot 整段替换为：

```vue
          <template #item.enabled="{ item }">
            <v-switch :model-value="item.enabled" density="compact" :hide-details="true"
              data-testid="airport-enabled" :loading="toggling === item.id"
              @update:model-value="toggleEnabled(item, $event as boolean)"></v-switch>
          </template>
```

## 第 4 步：spec（web/src/pages/AirportsPage.spec.ts 追加）
先读现有用例的 mock 方式（endpoints 是怎么被 vi.mock 的，照抄）。
- 用例 A「with-rules 开关有 hint 文案」：渲染后 wrapper.find('[data-testid="with-rules"]') 存在，且页面文本包含「全部刷新」或「导入规则」字样（hint 文案渲染）。
- 用例 B「行内开关调 updateAirport」：mock updateAirport resolve；找到 airport-enabled 开关，触发 update:modelValue(false)；断言 updateAirport 被调且 payload.enabled === false。

## 第 5 步：自测（原样贴输出给 lead）
```
pnpm -C web run typecheck
pnpm -C web run test -- --run web/src/pages/AirportsPage.spec.ts
pnpm -C web run test -- --run
```
预期：typecheck 无输出；单文件用例全过；全量 Test Files 35+ 全过。
（若队友会话里 pnpm 不在 PATH，用 node C:\Users\橙先生\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\pnpm\bin\pnpm.mjs -C web run ... 代替。）

## 完成定义
- typecheck + 全量测试绿（输出原文贴 lead）
- git status 只含本卡三个文件
