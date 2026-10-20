// 管线步骤参数归一化的纯函数（可单测，不依赖 Vue / Vuetify）。
//
// 背景：v-combobox 的 update:model-value 可能回传字符串，也可能是
// Vuetify 的 { title, value } 形态对象；当用户把输入框清空时还会回传
// null。旧的内联处理器直接读 (v as { value?: unknown }).value，
// v 为 null 时抛 TypeError（见 TASK-G G1）。

/**
 * 把 v-combobox 回传的 ref 参数归一化为字符串。
 * 语义：
 * - 字符串：原样返回；
 * - 对象且含 value 字段：取 value（字符串原样；数字/布尔用 String()；
 *   null/undefined 及其它类型一律 ''）；
 * - null / undefined / 其它类型：一律 ''。
 * 任何输入都不抛异常。
 */
export function stepArgRef(v: unknown): string {
  if (typeof v === 'string') return v
  if (v === null || typeof v !== 'object') return ''
  const inner = (v as { value?: unknown }).value
  if (typeof inner === 'string') return inner
  if (typeof inner === 'number' || typeof inner === 'boolean') return String(inner)
  return ''
}
