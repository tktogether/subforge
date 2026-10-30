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
  { key: 'include_self_text', title: '含自建' },
  { key: 'url', title: '测活地址' },
  { key: 'interval', title: '间隔(秒)' },
  { key: 'sort_order', title: '顺序' },
]

// 机场自带分组的成员可能很多：只预览前三个，避免表格被长列表撑破。
function membersText(members?: string[]): string {
  if (!members || members.length === 0) return '—'
  const preview = members.slice(0, 3).join('、')
  return members.length > 3 ? `${members.length} 个：${preview}…` : preview
}

// v2：表单不再有「方案」下拉；新建分组的 source 恒为 ''（自定义），由后端默认。
const groupFields = [
  { key: 'name', label: '名称', type: 'text', hint: '客户端里显示的策略组名；规则行的目标分组就是按这个名找的', example: '香港' },
  { key: 'type', label: '类型', type: 'text', hint: 'select=手动选；url-test=自动选延迟最低；fallback=按顺序回退；load-balance=负载均衡', example: 'select' },
  { key: 'regex', label: '匹配正则', type: 'text', hint: '按节点名匹配成员；留空则不自动收节点', example: '香港|HK' },
  { key: 'url', label: '测活地址', type: 'text', hint: '仅 url-test / fallback 需要；返回 204 的地址最合适', example: 'http://www.gstatic.com/generate_204' },
  { key: 'interval', label: '测活间隔（秒）', type: 'number', hint: '仅 url-test 需要；太小会频繁测活', example: '300' },
  {
    key: 'include_self',
    label: '包含自建节点',
    type: 'switch',
    default: false,
    hint: '开启后，订阅会在该分组成员末尾追加方案选中的自建节点。机场分组按成员渲染，自建节点不会自动出现；导入时成员覆盖全部机场节点的组默认开启，子集组默认关闭。',
  },
  { key: 'sort_order', label: '排序', type: 'number', hint: '越小越靠前' },
]

// 包装 list：给每行补一个 source_text 展示字段；新增/编辑透传 createGroup/updateGroup 原样。
const resource: any = {
  list: async (): Promise<(ProxyGroup & { source_text?: string })[]> => {
    const groups = await endpoints.listGroups()
    return groups.map((g) => ({
      ...g,
      source_text: sourceText(g.source),
      members_text: membersText(g.members),
      include_self_text: g.include_self ? '是' : '否',
    }))
  },
  create: endpoints.createGroup,
  update: (item: any) => endpoints.updateGroup(item.id, item),
  remove: (item: any) => endpoints.deleteGroup(item.id),
}
</script>

<template>
  <div class="groups-page">
    <v-alert type="info" variant="tonal" class="mb-4" data-testid="group-help">
      <div class="text-body-2">
        <p class="mb-1"><strong>分组是什么：</strong>分组 = 客户端里那个可以切换的「策略组」（select / url-test / fallback …）。它决定哪些节点归在一起、以及客户端怎么选节点。</p>
        <p class="mb-1"><strong>成员怎么来：</strong>填了「匹配正则」的分组，会把<strong>名字匹配该正则</strong>的节点收进来（例如 <code>香港|HK</code> 收所有名字带香港或 HK 的节点）。正则留空的分组不自动收节点。</p>
        <p class="mb-1"><strong>它怎么影响输出（关键）：</strong><strong>规则行里的 <code>rule_config</code> 填的就是这里的分组 id。</strong>分组被删掉或改名，引用它的规则行会退化成 <code>DIRECT</code>（直连）——改分组前先去「规则」页看一眼有没有规则在引用。</p>
        <p class="mb-1"><strong>机场自带分组：</strong>导入机场规则时会保留机场自己的分组与成员，显示在「成员」列；这类分组按成员渲染，编辑「匹配正则」不会覆盖成员。</p>
        <p class="mb-1"><strong>自建节点：</strong>机场分组只认自己的成员，自建节点不会自动出现。需要时打开「包含自建节点」，订阅会在原成员后追加方案选中的自建节点；导入时成员覆盖全部机场节点的组默认开启，子集组默认关闭。</p>
        <p class="mb-0"><strong>顺序：</strong><code>sort_order</code> 越小越靠前，决定客户端里分组的显示顺序。</p>
      </div>
    </v-alert>

    <ResourcePage
      title="代理分组"
      subtitle="决定客户端里的策略组与节点归组"
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
