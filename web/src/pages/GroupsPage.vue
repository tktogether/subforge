<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { pageHelp } from '../content/help'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints, type Profile } from '../api/endpoints'

const help = pageHelp('groups')

// 方案（Profile）列表：分组可以挂到某个方案下，只有绑定该方案的订阅才会用到它。
// 拉取失败时降级成「只有未分配」，不阻塞分组页本身的增删改查。
const profiles = ref<Profile[]>([])

async function loadProfiles(): Promise<void> {
  try {
    profiles.value = await endpoints.listProfiles()
  } catch {
    profiles.value = []
  }
}

// 方案列展示后端回填的 profile_name；未分配的分组该字段为空。
const groupColumns = [
  { key: 'name', title: '名称' },
  { key: 'profile_name', title: '方案' },
  { key: 'type', title: '类型' },
  { key: 'regex', title: '匹配正则' },
  { key: 'url', title: '测活地址' },
  { key: 'interval', title: '间隔(秒)' },
  { key: 'sort_order', title: '顺序' },
]

// 表单字段：方案用 select，选项 = 未分配(0) + 各方案；ResourcePage 的 blank()
// 会取第一个选项，即「未分配」。
const groupFields = computed(() => [
  { key: 'name', label: '名称', type: 'text', hint: '客户端里显示的策略组名；规则行的目标分组就是按这个名找的', example: '香港' },
  { key: 'profile_id', label: '方案', type: 'select', options: [{ title: '未分配', value: 0 }, ...profiles.value.map((p) => ({ title: p.name, value: p.id }))], hint: '方案里的分组只出现在绑定该方案的订阅；未分配 = 旧全局池' },
  { key: 'type', label: '类型', type: 'text', hint: 'select=手动选；url-test=自动选延迟最低；fallback=按顺序回退；load-balance=负载均衡', example: 'select' },
  { key: 'regex', label: '匹配正则', type: 'text', hint: '按节点名匹配成员；留空则不自动收节点', example: '香港|HK' },
  { key: 'url', label: '测活地址', type: 'text', hint: '仅 url-test / fallback 需要；返回 204 的地址最合适', example: 'http://www.gstatic.com/generate_204' },
  { key: 'interval', label: '测活间隔（秒）', type: 'number', hint: '仅 url-test 需要；太小会频繁测活', example: '300' },
  { key: 'sort_order', label: '排序', type: 'number', hint: '越小越靠前' },
])

onMounted(() => {
  void loadProfiles()
})

const resource: any = {
  list: endpoints.listGroups,
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
