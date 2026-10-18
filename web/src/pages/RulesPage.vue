<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { endpoints, type Rule, type RuleProvider, type Suggestion } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import PageHeader from '../components/PageHeader.vue'
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
    await endpoints.bulkRules({ source: 'airport:', enabled })
    notify('已更新', 'ok')
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

const resource: any = {
  list: endpoints.listRules,
  create: endpoints.createRule,
  update: (item: any) => endpoints.updateRule(item.id, item),
  remove: (item: any) => endpoints.deleteRule(item.id),
}

onMounted(() => {
  void loadProviders()
  void loadCandidates()
  void loadSuggestions()
})
</script>

<template>
  <PageHeader title="规则" subtitle="规则行 / 规则集 / 导入候选 / Rule-Bot 建议" />
  <v-tabs v-model="tab" class="mb-4">
    <v-tab>规则行</v-tab>
    <v-tab>规则集</v-tab>
    <v-tab>导入候选</v-tab>
    <v-tab>Rule-Bot 建议</v-tab>
  </v-tabs>

  <!-- Block 1: 规则行 -->
  <div v-if="tab === 0">
    <ResourcePage
      title="规则"
      subtitle="转换规则"
      :columns="[{ key: 'rule_type', title: '类型' }, { key: 'rule_param', title: '参数' }, { key: 'rule_config', title: '配置' }, { key: 'resolve_dns', title: '解析DNS' }, { key: 'enabled', title: '启用' }]"
      :fields="[{ key: 'rule_type', label: '类型', type: 'text' }, { key: 'rule_param', label: '参数', type: 'text' }, { key: 'rule_config', label: '配置', type: 'number' }, { key: 'resolve_dns', label: '解析DNS', type: 'switch' }, { key: 'enabled', label: '启用', type: 'switch' }]"
      :resource="resource"
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
            <v-chip size="small" :color="p.enabled ? 'success' : 'default'">{{ p.enabled ? '启用' : '禁用' }}</v-chip>
          </template>
        </v-list-item>
      </v-list>
      <v-expansion-panels class="mt-2">
        <v-expansion-panel v-for="p in providers" :key="'raw-' + p.id" :title="p.name + ' raw_json'">
          <v-expansion-panel-text>
            <pre class="text-caption">{{ p.raw_json }}</pre>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </AsyncSection>
  </div>

  <!-- Block 3: 导入候选 -->
  <div v-if="tab === 2">
    <AsyncSection :loading="candidatesLoading" :error="candidatesError" :empty="candidates.length === 0" empty-text="暂无导入候选" @retry="loadCandidates">
      <div class="mb-2">
        <v-btn size="small" color="primary" class="mr-2" @click="enableSelected(true)">启用选中</v-btn>
        <v-btn size="small" variant="tonal" class="mr-2" @click="enableSelected(false)">禁用选中</v-btn>
        <v-btn size="small" color="success" @click="enableAllAirport(true)">启用本机场全部</v-btn>
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
    <div v-if="suggestionsUnsupported" class="text-medium-emphasis pa-4">Rule-Bot 建议队列未启用</div>
    <AsyncSection v-else :loading="suggestionsLoading" :error="suggestionsError" :empty="suggestions.length === 0" empty-text="暂无建议" @retry="loadSuggestions">
      <div class="mb-2">
        <v-btn size="small" color="primary" @click="applySuggestions">应用已确认建议</v-btn>
      </div>
      <v-list>
        <v-list-item v-for="(s, i) in suggestions" :key="i">
          <v-list-item-title>{{ s.Anomaly.Type }}: {{ s.Anomaly.Message }}</v-list-item-title>
          <v-list-item-subtitle>{{ s.Rule }} — {{ s.Reason }}</v-list-item-subtitle>
          <template #append>
            <v-btn size="small" color="success" class="mr-2" @click="confirmSuggestion(i)">Confirm</v-btn>
            <v-btn size="small" color="error" @click="rejectSuggestion(i)">Reject</v-btn>
          </template>
        </v-list-item>
      </v-list>
    </AsyncSection>
  </div>
</template>
