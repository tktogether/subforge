# T3 · 规则源页：新增 / 删除 / 行内单源更新（接 T1 的三个新 API）

## 0. 你是谁、红线是什么
你是 SubForge 前端实施队员。三条红线：
1. 只允许修改这三个文件：web/src/api/endpoints.ts、web/src/pages/RuleSourcesPage.vue、web/src/pages/RuleSourcesPage.spec.ts。
2. UI 文案对齐 .impeccable.md：动词直接、错误消息 actionable、不堆装饰。颜色/CSS 一律不碰（另一队员在做全局 CSS）。
3. 不许 git commit / git add；不许新建任务卡。

## 1. 进度心跳（必须执行）
send_message 发给 lead，共 4 次：
- 心跳1「T3 book read, starting」
- 心跳2「T3 skeleton written」
- 心跳3「T3 self-test starting」
- 心跳4「T3 done」+ 自测输出原文

## 2. WHY
规则源页现在没有「新增/删除」入口（加 INI 源还得手编 manifest.yaml——用户痛点原话）；且行内「更新」按钮接的是全量接口（updateAll），点 src1 的更新会拉所有源。后端 T1 已提供三个新 API。本任务把页面接上。

## 3. 后端契约（T1 已定稿，前端照此接）
- POST /api/rule-sources  body {name?, url, input_format?} → 200 {path, created:true}；409 重名；400 url 空；404 清单未生成
- POST /api/rule-sources/remove  body {name} → 200 {ok:true, path}；404 不存在；400 最后一个
- POST /api/rule-sources/update-one  body {name} → 200 {results:[{source,version,path,from_cache,err}]}；404 不存在
（响应结构与现有 updateRuleSources 的 results 元素一致，可复用现有类型写法——打开 endpoints.ts 看 :411-418 现有四个函数怎么声明返回类型，照抄风格。）

## 4. 现状锚点（写代码前先打开文件核对行号附近内容）
- web/src/pages/RuleSourcesPage.vue：script setup 里现有 state（initDialog/rollbackDialog/mrsDialog/updateResults/showUpdateResults 等）与 load 函数（调用 endpoints.listRuleSources 的那个，记下它的名字，下文叫 loadList）。
- 行内更新按钮 bug 在 template 约 :199：data-testid="source-update" @click="updateAll" —— 改成 updateOne(item.name)。
- 工具行（update-all 按钮所在处）加「新增规则源」按钮。
- 表格行操作区（source-rollback/source-mrs 旁）加「删除」按钮。
- web/src/pages/RuleSourcesPage.spec.ts：vi.mock 里现有 6 个函数（:10-19），新增 mock 要补进这个清单，否则 undefined。

## 5. 逐步实施

### 步骤 1：web/src/api/endpoints.ts 追加三个函数
找到现有 rollbackRuleSource 的声明（:411-418 区域），在它旁边按同样风格追加（类型名如果 endpoints.ts 里已有 Result 类型就复用，没有就用下面的内联类型）：
```ts
  addRuleSource: (body: { name?: string; url: string; input_format?: string }): Promise<{ path: string; created: boolean }> =>
    api.post<{ path: string; created: boolean }>('/api/rule-sources', body),
  removeRuleSource: (name: string): Promise<OkResponse> =>
    api.post<OkResponse>('/api/rule-sources/remove', { name }),
  updateRuleSourceOne: (name: string): Promise<{ results: { source: string; version: string; path: string; from_cache: boolean; err: string }[] }> =>
    api.post<{ results: { source: string; version: string; path: string; from_cache: boolean; err: string }[] }>('/api/rule-sources/update-one', { name }),
```
校对：grep "OkResponse" web/src/api/endpoints.ts 确认该类型存在；不存在就换成内联 { ok: boolean }。

### 步骤 2：RuleSourcesPage.vue script 追加（放在现有 dialog state 旁边）
```ts
const addDialog = ref(false)
const addName = ref('')
const addUrl = ref('')
const addFormat = ref('')
const addSaving = ref(false)
const removeDialog = ref(false)
const removeName = ref('')
const removeSaving = ref(false)
const updatingOne = ref('')

function openAdd() {
  addName.value = ''
  addUrl.value = ''
  addFormat.value = ''
  addDialog.value = true
}

async function confirmAdd() {
  if (!addUrl.value.trim() || addSaving.value) return
  addSaving.value = true
  try {
    await endpoints.addRuleSource({
      name: addName.value.trim() || undefined,
      url: addUrl.value.trim(),
      input_format: addFormat.value.trim() || undefined,
    })
    addDialog.value = false
    // 成功提示照抄本页现有成功提示写法（如 notify/snackbar），没有就不加
    await loadList()   // ← loadList 换成第 4 节记下的真实函数名
  } catch (e) {
    // 错误提示照抄本页现有 catch 写法（rollback 的 catch 怎么写这里就怎么写）
  } finally {
    addSaving.value = false
  }
}

function openRemove(name: string) {
  removeName.value = name
  removeDialog.value = true
}

async function confirmRemove() {
  if (removeSaving.value) return
  removeSaving.value = true
  try {
    await endpoints.removeRuleSource(removeName.value)
    removeDialog.value = false
    await loadList()
  } catch (e) {
  } finally {
    removeSaving.value = false
  }
}

async function updateOne(name: string) {
  if (updatingOne.value) return
  updatingOne.value = name
  try {
    const r = await endpoints.updateRuleSourceOne(name)
    showUpdateResults(r.results)   // ← 复用现有 results 展示机制；函数名以源码为准
    await loadList()
  } catch (e) {
  } finally {
    updatingOne.value = ''
  }
}
```
校对（三个都必须做）：
1. grep "listRuleSources(" web/src/pages/RuleSourcesPage.vue 找到真实加载函数名替换 loadList。
2. grep "showUpdateResults\|updateResults" 确认展示函数/状态名，updateOne 里以真实名字调用。
3. 本页错误提示的真实写法（搜 rollback 的 catch / notify / snackbar）复制过来，不许自己发明。

### 步骤 3：RuleSourcesPage.vue template 改三处
(a) 工具行：在 data-testid="update-all" 按钮旁加：
```html
<v-btn data-testid="source-add" variant="tonal" @click="openAdd">新增规则源</v-btn>
```
(b) 行内更新按钮（:199 附近）：
- @click="updateAll" 改成 @click="updateOne(item.name)"（item 变量名以该 v-for 源码为准）
- :loading 加 updatingOne === item.name 的话注意保留原 disabled 逻辑（无版本 disabled 的行为不变）
(c) 行操作区加删除按钮（sources.length<=1 时禁用）：
```html
<v-btn
  data-testid="source-remove"
  variant="text"
  :disabled="sources.length <= 1"
  @click="openRemove(item.name)"
>删除</v-btn>
```
（sources 是本页渲染行用的数组名，以 v-for 源码为准；item.name 同理。）

### 步骤 4：两个对话框（放在现有 rollback/mrs 对话框旁，结构照抄它们）
```html
<v-dialog v-model="addDialog" max-width="520">
  <v-card data-testid="add-dialog">
    <v-card-title>新增规则源</v-card-title>
    <v-card-text>
      <v-text-field data-testid="add-name" v-model="addName" label="名称（留空自动取 URL 末段）" />
      <v-text-field data-testid="add-url" v-model="addUrl" label="订阅地址" placeholder="https://… 支持 http/https" />
      <v-text-field data-testid="add-format" v-model="addFormat" label="输入格式（可选：ini / clash / auto）" />
    </v-card-text>
    <v-card-actions>
      <v-spacer />
      <v-btn data-testid="add-cancel" variant="text" @click="addDialog = false">取消</v-btn>
      <v-btn data-testid="add-confirm" :loading="addSaving" @click="confirmAdd">添加</v-btn>
    </v-card-actions>
  </v-card>
</v-dialog>

<v-dialog v-model="removeDialog" max-width="420">
  <v-card data-testid="remove-dialog">
    <v-card-title>删除规则源</v-card-title>
    <v-card-text>确定删除「{{ removeName }}」？该源的本地版本文件不会被删除。</v-card-text>
    <v-card-actions>
      <v-spacer />
      <v-btn data-testid="remove-cancel" variant="text" @click="removeDialog = false">取消</v-btn>
      <v-btn data-testid="remove-confirm" :loading="removeSaving" @click="confirmRemove">删除</v-btn>
    </v-card-actions>
  </v-card>
</v-dialog>
```

### 步骤 5：RuleSourcesPage.spec.ts 增补
(a) vi.mock（:10-19）清单里追加三个：
```ts
    addRuleSource: vi.fn(),
    removeRuleSource: vi.fn(),
    updateRuleSourceOne: vi.fn(),
```
(b) 新增测试（加在 describe 内末尾；beforeEach 里给三个 mock 默认 resolve）：
```ts
const addMock = endpoints.addRuleSource as unknown as ReturnType<typeof vi.fn>
const removeMock = endpoints.removeRuleSource as unknown as ReturnType<typeof vi.fn>
const updateOneMock = endpoints.updateRuleSourceOne as unknown as ReturnType<typeof vi.fn>
// beforeEach 里补：
addMock.mockResolvedValue({ path: '/m/manifest.yaml', created: true })
removeMock.mockResolvedValue({ ok: true })
updateOneMock.mockResolvedValue({ results: [{ source: 'src1', version: 'v3', path: '/p/v3', from_cache: false, err: '' }] })

it('新增对话框：填表确认调用 addRuleSource 并重载列表', async () => {
  const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
  await flushPromises()
  await w.find('[data-testid="source-add"]').trigger('click')
  await flushPromises()
  expect((w.vm as any).addDialog).toBe(true)
  ;(w.vm as any).addUrl = 'http://example.com/new.yaml'
  ;(w.vm as any).addName = 'new'
  await (w.vm as any).confirmAdd()
  await flushPromises()
  expect(addMock).toHaveBeenCalledWith({ name: 'new', url: 'http://example.com/new.yaml', input_format: undefined })
  expect((w.vm as any).addDialog).toBe(false)
  w.unmount()
})

it('删除对话框：确认调用 removeRuleSource', async () => {
  const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
  await flushPromises()
  ;(w.vm as any).openRemove('src2')
  await flushPromises()
  await (w.vm as any).confirmRemove()
  await flushPromises()
  expect(removeMock).toHaveBeenCalledWith('src2')
  expect((w.vm as any).removeDialog).toBe(false)
  w.unmount()
})

it('行内更新调用 updateRuleSourceOne（不是 updateAll）', async () => {
  const w = mount(RuleSourcesPage, { global: { plugins: [vuetify] } })
  await flushPromises()
  await (w.vm as any).updateOne('src1')
  await flushPromises()
  expect(updateOneMock).toHaveBeenCalledWith('src1')
  expect(updateMock).not.toHaveBeenCalled()
  w.unmount()
})
```
（注意 :199 的按钮 @click 也要改——上面 script 的 updateOne 已经写好，模板别忘了。）

## 6. 自测命令（repo 根逐条跑，全部贴原文）
```
pnpm -C web run typecheck
pnpm -C web run test -- --run
pnpm -C web run build
```
预期：typecheck 0 error；test 全 pass（含旧用例——旧用例挂在哪就修哪，不许删旧用例）；build 成功。

## 7. 完成报告格式（心跳4 一起发）
- 改动文件列表 + 三条命令输出原文 + 校对三项的实际结果（加载函数名/展示函数名/OkResponse 是否存在）