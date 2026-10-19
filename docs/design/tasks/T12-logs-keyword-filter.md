# T12 日志页关键字过滤（F13 落地形态）

## WHY（为什么）
日志页只能整页翻。后端 LogEntry 是 {id, ts, line}（internal/api/logbuf.go:11-15），没有级别字段——所以不做假的「级别过滤」，做实用的**关键字过滤**（大小写不敏感，匹配 line）。

## 心跳（4 时点 send_message 给 lead）
1. 任务书读完 + 锚点校对完
2. 改动落盘
3. 自测开始
4. 全部完成 + 全量命令输出原文

## 红线
- 只许动：web/src/pages/LogsPage.vue、web/src/pages/LogsPage.spec.ts
- 不许 git commit / 不许建卡
- 不许改后端（logbuf / /api/logs 不动）

## 第 0 步：锚点校对
- web/src/pages/LogsPage.vue:34  const res = await endpoints.logs(lastId)
- 先通读 LogsPage.vue 全文（很短），找到：日志条目的渲染模板（v-for 列表）与轮询/加载逻辑。
对不上 → 停下报告 lead。

## 第 1 步：改 LogsPage.vue
1. script 加：
```ts
const keyword = ref('')
const filteredLogs = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (kw === '') return logs.value
  return logs.value.filter((l) => l.line.toLowerCase().includes(kw))
})
```
（logs 的真名以现有代码为准——:34 附近赋值的那个 ref。）
2. 模板：日志列表上方加：
```vue
  <v-text-field v-model="keyword" label="按关键字过滤（大小写不敏感）" clearable density="compact"
    data-testid="log-filter" class="mb-2"></v-text-field>
```
3. 渲染日志的 v-for 从 logs 改为 filteredLogs（变量名以实际为准）。
4. 空态区分：filteredLogs 为空但 logs 非空时显示「没有匹配「{{ keyword }}」的日志」（data-testid="log-filter-empty"）；用 v-if 加一行 text-caption 即可，不要动 AsyncSection 的空态。

## 第 2 步：spec（LogsPage.spec.ts 追加，mock 方式照抄现有用例）
- 用例 A：mock logs 返回 3 条 line 分别含 "GET /api/convert"、"refresh airport"、"error boom"；在 log-filter 输入 "error" 后，列表只剩 1 条，且页面不含另外两条的文本。
- 用例 B：输入 "nothing-matches" 后出现 log-filter-empty。

## 第 3 步：自测（原样贴输出给 lead）
```
pnpm -C web run typecheck
pnpm -C web run test -- --run web/src/pages/LogsPage.spec.ts
pnpm -C web run test -- --run
```
预期：全绿。
（pnpm 不在 PATH 时用 node C:\Users\橙先生\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\pnpm\bin\pnpm.mjs -C web run ...）

## 完成定义
- typecheck + 全量测试绿（输出原文贴 lead）
- git status 只含本卡两个文件
