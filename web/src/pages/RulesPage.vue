<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type ProxyGroup, type Rule, type RuleProvider, type Suggestion } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import PageHeader from '../components/PageHeader.vue'
import { pageHelp } from '../content/help'
import AsyncSection from '../components/AsyncSection.vue'
import ResourcePage from '../components/ResourcePage.vue'



// Tab
const tab = ref(0)

// Block 2: rule providers
const providersLoading = ref(false)
const providersError = ref('')
const providers = ref<RuleProvider[]>([])
const providerEnabledFilter = ref<string>('all')

async function loadProviders(): Promise<void> {
  providersLoading.value = true
  providersError.value = ''
  try {
    const enabled = providerEnabledFilter.value === 'all' ? undefined : providerEnabledFilter.value === 'enabled'
    providers.value = await endpoints.listRuleProviders(undefined, enabled)
  } catch (err) {
    providersError.value = errorMessage(err)
  } finally {
    providersLoading.value = false
  }
}

const togglingProvider = ref<number | null>(null)

async function toggleProvider(p: RuleProvider, value: boolean): Promise<void> {
  togglingProvider.value = p.id
  try {
    await endpoints.updateRuleProvider(p.id, value)
    p.enabled = value
  } catch (err) {
    notify(errorMessage(err), 'error')
  } finally {
    togglingProvider.value = null
  }
}

// T10: rule_config 分组下拉数据。拉失败只留「无分组（DIRECT）」，
// 不能把规则页主功能（列表/新增/编辑）拖死。
const groups = ref<ProxyGroup[]>([])
const groupOptions = computed(() => [
  { title: '无分组（DIRECT）', value: 0 },
  ...groups.value.map((g) => ({ title: `#${g.id} ${g.name}`, value: g.id })),
])

async function loadGroups(): Promise<void> {
  try {
    groups.value = await endpoints.listGroups()
  } catch {
    groups.value = []
  }
}

// Block 3: import candidates (rules with airport: source & enabled=false)
const candidatesLoading = ref(false)
const candidatesError = ref('')
const candidates = ref<Rule[]>([])
const selectedIds = ref<number[]>([])

async function loadCandidates(): Promise<void> {
  candidatesLoading.value = true
  candidatesError.value = ''
  try {
    const all = await endpoints.listRules()
    candidates.value = all.filter((r) => r.source?.startsWith('airport:') && r.enabled === false)
    selectedIds.value = []
  } catch (err) {
    candidatesError.value = errorMessage(err)
  } finally {
    candidatesLoading.value = false
  }
}

async function enableSelected(enabled: boolean): Promise<void> {
  if (selectedIds.value.length === 0) return
  try {
    await endpoints.bulkRules({ ids: [...selectedIds.value], enabled })
    notify('已更新', 'ok')
    await loadCandidates()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function enableAllAirport(enabled: boolean): Promise<void> {
  try {
    const all = await endpoints.listRules()
    const ids = all.filter((r) => (r.source ?? '').startsWith('airport:')).map((r) => r.id)
    if (ids.length === 0) { notify('没有来自机场的规则', 'error'); return }
    const r = await endpoints.bulkRules({ ids, enabled })
    notify('已更新 ' + r.updated + ' 条', 'ok')
    await loadCandidates()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

// Block 4: suggestions
const suggestionsLoading = ref(false)
const suggestionsError = ref('')
const suggestions = ref<Suggestion[]>([])
const suggestionsUnsupported = ref(false)

async function loadSuggestions(): Promise<void> {
  suggestionsLoading.value = true
  suggestionsError.value = ''
  suggestionsUnsupported.value = false
  try {
    suggestions.value = await endpoints.listSuggestions()
  } catch (err) {
    const msg = errorMessage(err)
    if (msg.includes('501') || msg.includes('未启用')) {
      suggestionsUnsupported.value = true
    } else {
      suggestionsError.value = msg
    }
  } finally {
    suggestionsLoading.value = false
  }
}

async function confirmSuggestion(i: number): Promise<void> {
  try {
    await endpoints.confirmSuggestion(i)
    notify('已确认', 'ok')
    await loadSuggestions()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function rejectSuggestion(i: number): Promise<void> {
  try {
    await endpoints.rejectSuggestion(i)
    notify('已拒绝', 'ok')
    await loadSuggestions()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function applySuggestions(): Promise<void> {
  try {
    const r = await endpoints.applySuggestions()
    notify('已应用 ' + r.applied + ' 条', 'ok')
    await loadSuggestions()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

// T10: 原来是模板里的内联字面量；rule_config 改 select，options 来自 groupOptions（下拉随分组数据刷新）。
const ruleFields = computed(() => [
  { key: 'rule_type', label: '类型', type: 'text' },
  { key: 'rule_param', label: '参数', type: 'text' },
  { key: 'rule_config', label: '配置', type: 'select', options: groupOptions.value },
  { key: 'resolve_dns', label: '解析DNS', type: 'switch' },
  { key: 'enabled', label: '启用', type: 'switch' },
])

const resource: any = {
  list: endpoints.listRules,
  create: endpoints.createRule,
  update: (item: any) => endpoints.updateRule(item.id, item),
  remove: (item: any) => endpoints.deleteRule(item.id),
}

onMounted(() => {
  void loadProviders()
  void loadGroups()
  void loadCandidates()
  void loadSuggestions()
})
</script>

<template>
  <PageHeader title="规则" subtitle="规则行 / 规则集 / 导入候选 / Rule-Bot 建议" help-key="rules" />
  <v-tabs v-model="tab" class="mb-4">
    <v-tab>规则行</v-tab>
    <v-tab>规则集</v-tab>
    <v-tab>导入候选</v-tab>
    <v-tab>Rule-Bot 建议</v-tab>
  </v-tabs>

  <!-- Block 1: 规则行 -->
  <div v-if="tab === 0">
    <v-alert type="info" variant="tonal" density="comfortable" class="mb-4">
      <div>一行规则最终长这样：rule_type,rule_param,目标分组；勾了「解析DNS」会在末尾追加 no-resolve。</div>
      <div>rule_config 选目标代理分组；选「无分组（DIRECT）」表示该行直连。选不到对应分组（分组 id 已不存在）时同样退化为 DIRECT。</div>
      <div>禁用（enabled=false）的规则完全不参与输出。</div>
    </v-alert>
    <ResourcePage
      title="规则行"
      subtitle="每行 = 输出的一条规则"
      help-key="rules"
      :columns="[{ key: 'rule_type', title: '类型' }, { key: 'rule_param', title: '参数' }, { key: 'group_name', title: '目标分组' }, { key: 'resolve_dns', title: '解析DNS' }, { key: 'enabled', title: '启用' }, { key: 'source', title: '来源' }]"
      :fields="ruleFields"
      :resource="resource"
      :empty="pageHelp('rules').emptyState"
    />
  </div>

  <!-- Block 2: 规则集 -->
  <div v-if="tab === 1">
    <AsyncSection :loading="providersLoading" :error="providersError" :empty="providers.length === 0" empty-text="暂无规则集" @retry="loadProviders">
      <div class="mb-2">
        <v-btn size="small" variant="tonal" class="mr-2" @click="providerEnabledFilter = 'all'; loadProviders()">全部</v-btn>
        <v-btn size="small" variant="tonal" class="mr-2" @click="providerEnabledFilter = 'enabled'; loadProviders()">已启用</v-btn>
        <v-btn size="small" variant="tonal" @click="providerEnabledFilter = 'disabled'; loadProviders()">已禁用</v-btn>
      </div>
      <v-list>
        <v-list-item v-for="p in providers" :key="p.id" :title="p.name" :subtitle="p.source + (p.enabled ? ' · 启用' : ' · 禁用')">
          <template #append>
            <v-switch :model-value="p.enabled" density="compact" :hide-details="true"
              data-testid="provider-switch" :loading="togglingProvider === p.id"
              @update:model-value="toggleProvider(p, $event as boolean)"></v-switch>
          </template>
        </v-list-item>
      </v-list>
      <v-expansion-panels class="mt-2">
        <v-expansion-panel v-for="p in providers" :key="'raw-' + p.id" :title="p.name + ' raw_json'">
          <v-expansion-panel-text eager>
            <pre class="text-caption">{{ p.raw_json }}</pre>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </AsyncSection>
  </div>

  <!-- Block 3: 导入候选 -->
  <div v-if="tab === 2">
    <v-alert type="info" variant="tonal" density="comfortable" class="mb-4" data-testid="candidates-note">
      <div>候选 = 从机场导入进来、但还没有启用的规则。启用它们之前不会影响订阅输出。</div>
      <div>本页在前端筛选（服务端无过滤参数）。</div>
    </v-alert>
    <AsyncSection :loading="candidatesLoading" :error="candidatesError" :empty="candidates.length === 0" empty-text="" :empty-state="pageHelp('rules').emptyState" @retry="loadCandidates">
      <div class="mb-2">
        <v-btn size="small" color="primary" class="mr-2" @click="enableSelected(true)">启用选中</v-btn>
        <v-btn size="small" variant="tonal" class="mr-2" data-testid="disable-selected" :disabled="selectedIds.length === 0" @click="enableSelected(false)">禁用选中</v-btn>
        <v-btn size="small" color="success" data-testid="enable-all-airport" @click="enableAllAirport(true)">启用本机场全部（source 以 airport: 开头的全部规则）</v-btn>
      </div>
      <v-data-table :items="candidates" show-select v-model="selectedIds" item-value="id" density="comfortable">
        <template #item.rule_config="{ item }">
          <span :class="item.rule_config === 0 ? 'text-error font-weight-bold' : ''">
            {{ item.rule_config === 0 ? '目标分组不存在' : item.rule_config }}
          </span>
        </template>
        <template #item.source="{ item }">{{ item.source }}</template>
        <template #item.rule_type="{ item }">{{ item.rule_type }}</template>
      </v-data-table>
    </AsyncSection>
  </div>

  <!-- Block 4: Rule-Bot 建议 -->
  <div v-if="tab === 3">
    <v-card v-if="suggestionsUnsupported" data-testid="suggestions-unsupported" class="pa-4">
      <div class="text-h6">Rule-Bot 建议队列未启用</div>
      <div class="mt-2">建议来自定时任务失败检测；服务启动时会自动启用。如果你看到这条，说明当前进程没有装配调度器。</div>
      <v-btn size="small" color="primary" class="mt-2" @click="loadSuggestions">重试</v-btn>
    </v-card>
    <AsyncSection v-else :loading="suggestionsLoading" :error="suggestionsError" :empty="suggestions.length === 0" empty-text="暂无建议" @retry="loadSuggestions">
      <div class="mb-2">
        <v-btn size="small" color="primary" @click="applySuggestions">应用已确认建议</v-btn>
      </div>
      <v-card v-for="(s, i) in suggestions" :key="i" data-testid="suggestion-card" class="pa-4 mb-2">
        <div class="text-subtitle-1">{{ s.Anomaly.Type }}</div>
        <div class="text-medium-emphasis">{{ s.Anomaly.Message }}</div>
        <pre class="text-caption mt-2">{{ s.Rule }}</pre>
        <div class="mt-2">{{ s.Reason }}</div>
        <div class="text-caption text-medium-emphasis mt-2">{{ s.Anomaly.Source }} · {{ s.Anomaly.Timestamp }}</div>
        <span v-if="s.AutoApply === true" class="sf-status mt-2">
          <span class="sf-status__dot sf-status__dot--muted"></span>
          <span>自动应用</span>
        </span>
        <div class="text-caption mt-2">确认只是标记；要让规则真正生效还要点顶部「应用已确认建议」。</div>
        <div class="mt-2">
          <v-btn size="small" color="success" class="mr-2" data-testid="suggestion-confirm" @click="confirmSuggestion(i)">确认</v-btn>
          <v-btn size="small" color="error" data-testid="suggestion-reject" @click="rejectSuggestion(i)">拒绝</v-btn>
        </div>
      </v-card>
    </AsyncSection>
  </div>
</template>
