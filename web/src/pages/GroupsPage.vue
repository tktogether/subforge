<script setup lang="ts">
import { computed, ref } from 'vue'
import { pageHelp } from '../content/help'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints, type ProxyGroup } from '../api/endpoints'

const help = pageHelp('groups')

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

// 来源/成员列挂计算字段，ResourcePage 直接当普通列渲染。
const groupColumns = [
  { key: 'name', title: '名称' },
  { key: 'source_text', title: '来源' },
  { key: 'type', title: '类型' },
  { key: 'regex', title: '匹配正则' },
  { key: 'members_text', title: '成员' },
  { key: 'expand_text', title: '成员扩展' },
  { key: 'url', title: '检测地址' },
  { key: 'interval', title: '间隔(秒)' },
]

// 机场自带分组的成员可能很多：只预览前三个，完整列表按需打开。
function membersText(members?: string[]): string {
  if (!members || members.length === 0) return '—'
  const preview = members.slice(0, 3).join('、')
  return members.length > 3 ? `${members.length} 个：${preview}…` : preview
}

function openMembers(group: any): void {
  membersGroup.value = group
  membersSearch.value = ''
  membersOpen.value = true
}

function closeMembers(): void {
  membersOpen.value = false
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
const resource: any = {
  list: async (): Promise<(ProxyGroup & { source_text?: string })[]> => {
    const groups = await endpoints.listGroups()
    return groups.map((g) => ({
      ...g,
      include_self: !!g.include_self,
      include_all_sources: !!g.include_all_sources,
      source_text: sourceText(g.source),
      members_text: membersText(g.members),
      expand_text: expandText(g),
    }))
  },
  create: endpoints.createGroup,
  update: (item: any) => endpoints.updateGroup(item.id, item),
  remove: (item: any) => endpoints.deleteGroup(item.id),
}
</script>

<template>
  <div class="groups-page">
    <ResourcePage
      title="代理分组"
      :subtitle="help.oneLiner"
      help-key="groups"
      :columns="groupColumns"
      :fields="groupFields"
      :resource="resource"
      :empty="help.emptyState"
    >
      <template #members="{ item }">
        <button
          v-if="item.members && item.members.length > 0"
          type="button"
          class="members-trigger"
          data-testid="group-members-open"
          @click="openMembers(item)"
        >{{ membersText(item.members) }}</button>
        <span v-else>—</span>
      </template>

    </ResourcePage>

    <v-dialog :model-value="membersOpen" max-width="680" scrollable data-testid="group-members-dialog" @update:model-value="closeMembers">
      <v-card>
        <v-card-title>分组成员 · {{ membersGroup?.name }}</v-card-title>
        <v-card-text>
          <div class="text-caption text-medium-emphasis mb-2">共 {{ membersGroup?.members?.length ?? 0 }} 个成员</div>
          <v-text-field
            v-if="(membersGroup?.members?.length ?? 0) > 10"
            v-model="membersSearch"
            label="搜索成员"
            density="compact"
            variant="outlined"
            hide-details
            clearable
            data-testid="group-members-search"
            class="mb-3"
          />
          <div class="member-grid" data-testid="group-member-list">
            <span v-for="member in filteredMembers" :key="member" class="member-item" data-testid="group-member-item">{{ member }}</span>
          </div>
          <div v-if="filteredMembers.length === 0" class="text-caption text-medium-emphasis mt-2">没有匹配的成员</div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="group-members-close" @click="closeMembers">关闭</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-expansion-panels class="mt-4" data-testid="group-recipes">
      <v-expansion-panel>
        <v-expansion-panel-title>常见分组配方</v-expansion-panel-title>
        <v-expansion-panel-text>
          <ul class="text-body-2">
            <li><strong>香港</strong>：type=<code>url-test</code>，regex=<code>香港|HK</code>，url=<code>http://www.gstatic.com/generate_204</code>，interval=<code>300</code></li>
            <li><strong>自动选择</strong>：type=<code>url-test</code>，regex=<code>.</code>（匹配全部）</li>
            <li><strong>故障转移</strong>：type=<code>fallback</code>，regex=<code>香港|HK</code></li>
          </ul>
        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>
  </div>
</template>

<style scoped>
.members-trigger {
  max-width: 260px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--sf-accent);
  font: inherit;
  text-align: left;
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 2px;
}
.member-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 16px;
}
.member-item {
  min-width: 0;
  padding: 4px 0;
  border-bottom: 1px solid var(--sf-border);
  overflow-wrap: anywhere;
  font-size: 13px;
}
@media (max-width: 600px) {
  .member-grid {
    grid-template-columns: 1fr;
  }
}
</style>
