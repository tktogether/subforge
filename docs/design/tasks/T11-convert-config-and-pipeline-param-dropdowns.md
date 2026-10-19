# T11 转换页 config 下拉（F11）+ 管线步骤参数下拉（F12）

## WHY（为什么）
- F11：手动转换页 config 参数被硬编码为空（ConvertPage.vue:117 config: ''），后端 GET /api/configs 明明能列出可选配置。
- F12：管线步骤参数全是手填文本框（PipelinesPage.vue:263-287），但 op.sort 的 by 只有 5 个合法值，op.script/filter.script 的 ref 是既有脚本名——都该下拉。

## 心跳（4 时点 send_message 给 lead）
1. 任务书读完 + 锚点校对完
2. 骨架落盘
3. 自测开始
4. 全部完成 + 全量命令输出原文

## 红线
- 只许动：web/src/pages/ConvertPage.vue、web/src/pages/ConvertPage.spec.ts、web/src/pages/PipelinesPage.vue、web/src/pages/PipelinesPage.spec.ts
- 不许 git commit / 不许建卡
- 值域不许编造：op.sort.by 的 5 值来自 web/src/content/help.ts:534（by= 排序键：original / name / type / server / port）

## 第 0 步：锚点校对
- web/src/pages/ConvertPage.vue:92  附近已有 const [t, p] = await Promise.all([endpoints.targets(), endpoints.pipelines('')]) —— pipeline 下拉就是现成模式
- web/src/pages/ConvertPage.vue:117  config: '',
- web/src/api/endpoints.ts  搜 configs: —— 应有 configs: (): Promise<{configs: string[]; default: string}> => ...（T5 已交付）
- web/src/pages/PipelinesPage.vue:46-65  paramFields(op) 返回字段名数组
- web/src/pages/PipelinesPage.vue:283  附近是参数 v-text-field（v-for f in paramFields(step.op) 内）
- web/src/content/help.ts:534  kind: 'op.sort', ... by= 排序键：original / name / type / server / port
- web/src/api/endpoints.ts  搜 listScripts: —— (): Promise<Script[]> => ...（ref 下拉数据源）
对不上下任何一条 → 停下报告 lead。

## 第 1 步：ConvertPage config 下拉
1. script 加：const configs = ref<string[]>([])、const configDefault = ref('')、const configValue = ref<string | null>(null)
2. 第 0 步 :92 的 Promise.all 处（或它所在的加载函数）里并行再拉一份：endpoints.configs()，填充 configs 与 configDefault。拉取失败不阻塞页面（catch 后 configs 留空数组，并在 UI 上退化为可手填）。
3. 模板里 pipeline 选择器旁边加：

```vue
      <v-select v-model="configValue" :items="configs" label="配置模板（可选）" clearable
        data-testid="convert-config" :hint="configDefault ? ('默认：' + configDefault) : ''" persistent-hint></v-select>
```
4. submit 里 :117 改为：config: configValue.value ?? '',

## 第 2 步：PipelinesPage 参数下拉
找到 :263-290 的 v-for f in paramFields(step.op) 循环体里渲染 v-text-field 的那段，改成三分支：

```vue
              <template v-for="f in paramFields(step.op)" :key="f">
                <!-- 原有字段定义/label/hint 逻辑保留，仅替换控件本体 -->
                <v-select v-if="step.op === 'op.sort' && f === 'by'"
                  v-model="step.args[f]" :items="['original', 'name', 'type', 'server', 'port']"
                  :label="f" data-testid="param-by" :hint="paramHint(step.op, f)" persistent-hint />
                <v-combobox v-else-if="(step.op === 'op.script' || step.op === 'filter.script') && f === 'ref'"
                  v-model="step.args[f]" :items="scriptNames" :label="f" data-testid="param-ref"
                  :hint="paramHint(step.op, f)" persistent-hint />
                <v-text-field v-else
                  v-model="step.args[f]" :label="f" data-testid="param-field"
                  :hint="paramHint(step.op, f)" persistent-hint />
              </template>
```
注意：上面只是控件替换示意——先读原模板 :263-290，保留原有 label/hint/data-testid 绑定方式，不要把原有属性弄丢。原 v-text-field 上的所有绑定，按分支原样迁移。
script 加：const scriptNames = ref<string[]>([])；onMounted/加载函数里 try { scriptNames.value = (await endpoints.listScripts()).map(s => s.name) } catch { scriptNames.value = [] }（Script 的名字字段以 endpoints.ts 的 export interface Script 为准，先核对）。

## 第 3 步：spec
- ConvertPage.spec.ts：mock endpoints.configs 返回 {configs:['a.ini','b.ini'],default:'a.ini'}；断言 convert-config 下拉出现且含两个选项；提交后断言 convert 调用参数 config 为所选项（clearable 清空时为 ''）。
- PipelinesPage.spec.ts：mock listScripts 返回 [{name:'hk',...}]（字段照 Script 接口补全）；选 op.sort 步骤时断言 param-by 是 select 且含 'name' 选项；op.script 步骤时 param-ref 含 'hk'。
（mock 方式照抄各自 spec 现有写法——先读再写。）

## 第 4 步：自测（原样贴输出给 lead）
```
pnpm -C web run typecheck
pnpm -C web run test -- --run web/src/pages/ConvertPage.spec.ts web/src/pages/PipelinesPage.spec.ts
pnpm -C web run test -- --run
```
预期：全绿。
（pnpm 不在 PATH 时用 node C:\Users\橙先生\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\pnpm\bin\pnpm.mjs -C web run ...）

## 完成定义
- typecheck + 全量测试绿（输出原文贴 lead）
- git status 只含本卡四个文件
