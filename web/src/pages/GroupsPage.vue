<script setup lang="ts">
import { pageHelp } from '../content/help'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints, type ProxyGroup } from '../api/endpoints'

const help = pageHelp('groups')

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
  { key: 'sort_order', title: '顺序' },
]

// 机场自带分组的成员可能很多：只预览前三个，避免表格被长列表撑破。
function membersText(members?: string[]): string {
  if (!members || members.length === 0) return '—'
  const preview = members.slice(0, 3).join('、')
  return members.length > 3 ? `${members.length} 个：${preview}…` : preview
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
  { key: 'sort_order', label: '排序', type: 'number', hint: '数值越小越靠前。' },
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
      rank-key="sort_order"
    />

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
