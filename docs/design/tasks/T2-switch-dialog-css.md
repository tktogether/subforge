# T2 · 全局 CSS：开关未选中态对比度 + 对话框顶部裁切

## 0. 你是谁、红线是什么
你是 SubForge 前端实施队员。三条红线：
1. 只允许修改 web/src/styles/vuetify-overrides.css 这一个文件。其他一律只读。
2. 颜色只许用 var(--sf-*)，不许写字面 hex/rgb（web/src/design/tokens.css:5-6 红线）。
3. 不许 git commit / git add；不许新建任务卡。

## 1. 进度心跳（必须执行）
send_message 发给 lead，共 4 次：
- 心跳1「T2 book read, starting」
- 心跳2「T2 CSS appended」
- 心跳3「T2 self-test starting」
- 心跳4「T2 done」+ 自测输出原文

## 2. WHY（两个用户可见 bug）
1. v-switch 未选中态：Vuetify 默认 track 是半透明 currentColor，叠在米色页面背景（--sf-bg:#F0F2E6）上几乎看不见（用户截图实锤）。
2. v-dialog 高内容顶部被视口裁掉：Vuetify 3.13 默认 .v-overlay__content 只 flex 居中不限制 card 高度，内容超过视口时顶部（含标题/关闭区）溢出到视口外。

## 3. 改动清单
只改 web/src/styles/vuetify-overrides.css：文件末尾追加一段（该文件现约 127 行，无 v-switch/v-dialog 规则，追加不冲突）。

## 4. 类名依据（Vuetify 3.13.7 node_modules 实测，直接用）
- node_modules/vuetify/lib/components/VSwitch/VSwitch.css：未选中 .v-selection-control:not(.v-selection-control--dirty)、选中 .v-selection-control--dirty、.v-switch__track、.v-switch__thumb。
- node_modules/vuetify/lib/components/VDialog/VDialog.css：.v-dialog > .v-overlay__content 居中；card 直接是 > .v-card 或 > form > .v-card（有 form 包裹变体）。

## 5. 实施：在 web/src/styles/vuetify-overrides.css 末尾原样追加
```css

/* ---- 开关对比度修复：未选中态从半透明改成实心描边（用户反馈与背景重合） ---- */
.v-switch .v-selection-control:not(.v-selection-control--dirty) .v-switch__track {
  background: var(--sf-border-strong);
  opacity: 1;
}
.v-switch .v-selection-control:not(.v-selection-control--dirty) .v-switch__thumb {
  background: var(--sf-surface);
  border: 1px solid var(--sf-border-strong);
  color: var(--sf-text-faint);
}
/* ---- 开关选中态：与主题主色对齐 ---- */
.v-switch .v-selection-control--dirty .v-switch__track {
  background: var(--sf-accent);
  opacity: 1;
}
.v-switch .v-selection-control--dirty .v-switch__thumb {
  background: var(--sf-accent-fg);
}

/* ---- 对话框防顶部裁切：card 限高 + 内部滚动 ---- */
.v-dialog > .v-overlay__content > .v-card,
.v-dialog > .v-overlay__content > form > .v-card {
  max-height: calc(100dvh - 48px);
  display: flex;
  flex-direction: column;
}
.v-dialog > .v-overlay__content > .v-card > .v-card-text,
.v-dialog > .v-overlay__content > form > .v-card > .v-card-text {
  overflow-y: auto;
}
```

## 6. 自测（repo 根逐条跑，贴原文）
```
pnpm -C web run test -- --run
pnpm -C web run typecheck
```
预期：全部 pass。再自查一遍：追加段里不许出现 # 或 rgb( 字面色（var(--sf-*) 之外零色值）。

## 7. 完成报告格式（心跳4 一起发）
- 改动文件（完整路径）+ 追加行数 + 两条命令输出原文 + 「色值红线自查通过」