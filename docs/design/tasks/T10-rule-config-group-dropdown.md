# T10 规则页：rule_config 改分组下拉（F10）

## WHY（为什么）
新增/编辑规则时 rule_config 要手填分组 id（RulesPage.vue:168 type:'number'），帮助文案还得教用户「去分组页看 id」（:160）。改成下拉直接选分组名，id 由 items 的 value 承载。0 = 无分组（DIRECT）保留。

## 心跳（4 时点 send_message 给 lead）
1. 任务书读完 + 锚点校对完
2. 骨架落盘（ResourcePage select 分支）
3. 自测开始
4. 全部完成 + 全量命令输出原文

## 红线
- 只许动：web/src/components/ResourcePage.vue、web/src/components/ResourcePage.spec.ts、web/src/pages/RulesPage.vue、web/src/pages/RulesPage.spec.ts
- 不许 git commit / 不许建卡
- ResourcePage 是共享组件：改动必须是**向后兼容的增量**（不传 options 的字段行为零变化）。改完必须跑全量测试，任何别的页面挂了都算本卡失败。

## 第 0 步：锚点校对
- web/src/pages/RulesPage.vue:158-170  帮助文案 + :fields="[...]" 数组（rule_config 是 { key: 'rule_config', label: '配置', type: 'number' }）
- web/src/pages/RulesPage.vue:6-9  ResourcePage 组件 import
- web/src/components/ResourcePage.vue:233-241  字段类型分支：field.type === 'switch' → v-switch；'number' → 数字输入；else 文本
- web/src/components/ResourcePage.vue:115  空表单字段默认值
- web/src/api/endpoints.ts:20  export interface ProxyGroup {...} —— 核对它有没有 id 和 name 字段（下拉要用）
- web/src/api/endpoints.ts:375  listGroups: (): Promise<ProxyGroup[]> => ...
对不上 → 停下报告 lead。

## 第 1 步：ResourcePage.vue 支持 select 字段（向后兼容增量）
1. 字段类型定义（先读 ResourcePage.vue 里 fields prop 的类型声明，形如 { key: string; label: string; type: string; ... }）加可选字段：options?: { title: string; value: number | string }[]
2. :115 默认值行不用动（select 字段的值也是 number/string）。
3. :233-241 分支链里，在 number 分支之后加：

```vue
        <v-select v-else-if="field.type === 'select'" v-model="row[field.key]"
          :items="field.options ?? []" item-title="title" item-value="value"
          :label="field.label" density="compact" :hide-details="'auto'" />
```
（label/hide-details 等属性照抄相邻分支的写法，保持一致；不要引入新的视觉风格。）

## 第 2 步：RulesPage.vue 接分组数据
1. script 加：
```ts
const groups = ref<ProxyGroup[]>([])
const groupOptions = computed(() => [
  { title: '无分组（DIRECT）', value: 0 },
  ...groups.value.map((g) => ({ title: `#${g.id} ${g.name}`, value: g.id })),
])
```
（ProxyGroup 类型从 '../api/endpoints' import——核对现有 import 行。）
2. 现有 onMounted/加载函数里并行拉一份：groups.value = await endpoints.listGroups()（失败 catch 住留空数组，下拉只剩「无分组」——页面主功能不能被它拖死；参考 loadProviders 的容错写法）。
3. :168 fields 数组里 rule_config 改为：
```ts
{ key: 'rule_config', label: '配置', type: 'select' }
```
options 的传递方式以第 1 步确定的 fields prop 类型为准（:fields 数组的对象直接带 options: groupOptions.value —— 若 :fields 是模板里的静态数组，绑定为 :fields="[{...}, {..., options: groupOptions}]"，注意 computed 解包；以模板现有写法风格为准）。
4. :160 的帮助文案改一句：把「rule_config 填的是「目标代理分组的 id」（去「分组」页看 id）」改成「rule_config 选目标代理分组；选「无分组（DIRECT）」表示该行直连」。

## 第 3 步：spec
- ResourcePage.spec.ts：追加一个用例——type:'select' + options 的字段渲染成 v-select 且含给定 options；不带 options 的 text/number/switch 字段行为不变（现有用例就是回归网，全量绿即证）。
- RulesPage.spec.ts：mock listGroups 返回 [{id: 3, name: 'HK', ...}]（字段照 ProxyGroup 接口补全）；断言 rule_config 编辑控件出现且含「#3 HK」选项文案；提交保存 payload 里 rule_config === 3。
（mock 注入方式照抄 RulesPage.spec.ts 现有写法。）

## 第 4 步：自测（原样贴输出给 lead）
```
pnpm -C web run typecheck
pnpm -C web run test -- --run web/src/components/ResourcePage.spec.ts web/src/pages/RulesPage.spec.ts
pnpm -C web run test -- --run
```
预期：全绿（**全量**——ResourcePage 是共享组件，别的页面用到它的用例都必须过）。
（pnpm 不在 PATH 时用 node C:\Users\橙先生\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\pnpm\bin\pnpm.mjs -C web run ...）

## 完成定义
- typecheck + 全量测试绿（输出原文贴 lead）
- git status 只含本卡四个文件
