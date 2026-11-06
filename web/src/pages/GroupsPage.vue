<script setup lang="ts">
import { computed, ref } from 'vue'
import { IconChevronDown } from '@tabler/icons-vue'
import SfResourcePage from '../components/ui/SfResourcePage.vue'
import SfDialog from '../components/ui/SfDialog.vue'
import { endpoints, type ProxyGroup } from '../api/endpoints'

const membersGroup = ref<ProxyGroup | null>(null)
const membersOpen = ref(false)
const membersSearch = ref('')
const filteredMembers = computed(() => {
  const members = membersGroup.value?.members ?? []
  const q = membersSearch.value.trim().toLowerCase()
  return q === '' ? members : members.filter((m) => m.toLowerCase().includes(q))
})

// v2：分组不再归属方案，按来源(source)分层。来源列展示文案：
// '' → 「自定义」；'airport:<机场名>' → 「机场:<机场名>」；其它原样显示。
function sourceText(source?: string): string {
  if (!source || source === '') return '自定义'
  return source.startsWith('airport:') ? '机场:' + source.slice('airport:'.length) : source
}

const groupColumns = [
  { key: 'name', title: '名称' },
  { key: 'source_text', title: '来源' },
  { key: 'type', title: '类型', mono: true },
  { key: 'regex', title: '匹配正则', mono: true },
  { key: 'members_text', title: '成员' },
  { key: 'expand_text', title: '成员扩展' },
  { key: 'url', title: '检测地址', mono: true },
  { key: 'interval', title: '间隔(秒)', mono: true, align: 'right' as const },
]

// 机场自带分组的成员可能很多：只预览前三个，完整列表按需打开。
function membersText(members?: string[]): string {
  if (!members || members.length === 0) return '—'
  const preview = members.slice(0, 3).join('、')
  return members.length > 3 ? `${members.length} 个：${preview}…` : preview
}

function openMembers(group: ProxyGroup & Record<string, unknown>): void {
  membersGroup.value = group
  membersSearch.value = ''
  membersOpen.value = true
}

// 成员扩展是「全部来源 + 自建」两种开关的组合，表格只显示最终生效范围。
function expandText(g: ProxyGroup): string {
  if (g.include_all_sources && g.include_self) return '全部来源'
  if (g.include_all_sources) return '其他机场'
  if (g.include_self) return '自建'
  return '—'
}

// v2：表单不再有「方案」下拉；新建分组的 source 恒为 ''（自定义），由后端默认。
const groupFields = [
  { key: 'name', label: '名称', type: 'text', hint: '客户端中显示的策略组名称；规则通过该名称选择目标分组。', example: '香港' },
  { key: 'type', label: '类型', type: 'text', hint: 'select 手动选择；url-test 自动选择延迟最低；fallback 按顺序故障切换；load-balance 负载均衡。', example: 'select' },
  { key: 'regex', label: '匹配正则', type: 'text', hint: '按节点名称匹配成员；留空时不按正则选择成员。', example: '香港|HK' },
  { key: 'url', label: '检测地址', type: 'text', hint: '自动测速使用的检测地址；仅 url-test 与 fallback 需要。', example: 'http://www.gstatic.com/generate_204' },
  { key: 'interval', label: '检测间隔（秒）', type: 'number', hint: '自动测速间隔，单位秒。', example: '300' },
  {
    key: 'include_all_sources',
    label: '包含全部来源节点',
    type: 'switch',
    default: false,
    hint: '将方案中所有机场的节点加入该分组。成员覆盖本机场全部节点的分组在导入时默认开启。',
  },
  {
    key: 'include_self',
    label: '包含自建节点',
    type: 'switch',
    default: false,
    hint: '将方案中选中的自建节点加入该分组。',
  },
]

// 包装 list：给每行补一个 source_text 展示字段；新增/编辑透传 createGroup/updateGroup 原样。
const resource = {
  list: async (): Promise<(ProxyGroup & Record<string, unknown>)[]> => {
    const groups = await endpoints.listGroups()
    return groups.map((g) => ({
      ...g,
      include_self: !!g.include_self,
      include_all_sources: !!g.include_all_sources,
      source_text: sourceText(g.source),
      members_text: membersText(g.members),
      expand_text: expandText(g),
    })) as (ProxyGroup & Record<string, unknown>)[]
  },
  create: (item: Record<string, unknown>) => endpoints.createGroup(item as unknown as Parameters<typeof endpoints.createGroup>[0]),
  update: (item: Record<string, unknown>) => endpoints.updateGroup(item['id'] as number, item as unknown as Parameters<typeof endpoints.updateGroup>[1]),
  remove: (item: Record<string, unknown>) => endpoints.deleteGroup(item['id'] as number),
}

const recipesOpen = ref(false)
</script>

<template>
  <div>
    <SfResourcePage
      page-title="代理分组"
      :columns="groupColumns"
      :fields="groupFields"
      :resource="resource"
      empty-title="还没有代理分组"
      empty-desc="新增一个分组，或从机场页导入订阅自带分组。"
    >
      <template #cell-url="{ value }">
        <span class="sf-mono group-url" :title="String(value)">{{ value }}</span>
      </template>
      <template #cell-members_text="{ row }">
        <button
          v-if="Array.isArray(row['members']) && row['members'].length > 0"
          type="button"
          class="members-trigger"
          data-testid="group-members-open"
          @click="openMembers(row as ProxyGroup & Record<string, unknown>)"
        >{{ String(row['members_text']) }}</button>
        <span v-else class="sf-faint">—</span>
      </template>
    </SfResourcePage>

    <SfDialog :open="membersOpen" label="GROUP MEMBERS" :title="'分组成员 · ' + (membersGroup?.name ?? '')" @close="membersOpen = false">
      <p class="sf-faint" style="margin: 0; font-size: 12.5px">共 {{ membersGroup?.members?.length ?? 0 }} 个成员</p>
      <input
        v-if="(membersGroup?.members?.length ?? 0) > 10"
        v-model="membersSearch"
        class="sf-input sf-input--mono"
        placeholder="搜索成员…"
        data-testid="group-members-search"
      />
      <div class="member-grid" data-testid="group-member-list">
        <span v-for="member in filteredMembers" :key="member" class="member-item" data-testid="group-member-item">{{ member }}</span>
      </div>
      <p v-if="filteredMembers.length === 0" class="sf-faint" style="margin: 0; font-size: 12.5px">没有匹配的成员</p>
      <template #foot>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="group-members-close" @click="membersOpen = false">关闭</button>
      </template>
    </SfDialog>

    <section class="sf-panel sf-rise sf-d2" style="margin-top: 16px" data-testid="group-recipes">
      <button type="button" class="recipes-head" @click="recipesOpen = !recipesOpen">
        <span class="sf-panel__title">常见分组配方</span>
        <span class="sf-spacer" />
        <IconChevronDown :size="16" :stroke-width="1.8" class="sf-faint" :style="{ transform: recipesOpen ? 'rotate(180deg)' : 'none', transition: 'transform var(--sf-t-fast) var(--sf-ease)' }" aria-hidden="true" />
      </button>
      <div class="sf-collapse" :class="{ 'is-open': recipesOpen }">
        <div class="sf-collapse__inner">
          <ul class="recipes-list">
            <li><b>香港</b>：type=<code>url-test</code>，regex=<code>香港|HK</code>，url=<code>http://www.gstatic.com/generate_204</code>，interval=<code>300</code></li>
            <li><b>自动选择</b>：type=<code>url-test</code>，regex=<code>.</code>（匹配全部）</li>
            <li><b>故障转移</b>：type=<code>fallback</code>，regex=<code>香港|HK</code></li>
          </ul>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.group-url {
  display: inline-block;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  font-size: 11.5px;
  color: var(--sf-text-muted);
}

.members-trigger {
  max-width: 260px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--sf-accent);
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 3px;
  text-decoration-color: oklch(from var(--sf-accent) l c h / 0.4);
}

.members-trigger:hover {
  text-decoration-color: var(--sf-accent);
}

.member-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 18px;
}

.member-item {
  min-width: 0;
  padding: 5px 0;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.55);
  overflow-wrap: anywhere;
  font-size: 13px;
}

@media (max-width: 600px) {
  .member-grid {
    grid-template-columns: 1fr;
  }
}

.recipes-head {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 18px;
  background: none;
  border: none;
  color: var(--sf-text);
  cursor: pointer;
  font: inherit;
  text-align: left;
}

.recipes-list {
  margin: 0;
  padding: 4px 18px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--sf-text-muted);
}

.recipes-list code {
  font-size: 12px;
  background: var(--sf-surface-dim);
  border: 1px solid var(--sf-border);
  border-radius: 3px;
  padding: 0 5px;
}
</style>
