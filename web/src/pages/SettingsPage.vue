<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type SettingItem } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { useThemeMode } from '../composables/useThemeMode'
import { notify } from '../composables/useNotify'
import { pageHelp, fieldHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'

const help = pageHelp('settings')

const targets = ref<string[]>([])
const statsHistory = ref(0)
const error = ref('')
const loading = ref(false)
const settings = ref<SettingItem[]>([])
const saving = ref(false)
const theme = useThemeMode()

const emptyState = computed(() => help.emptyState)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const [t, s, st] = await Promise.all([
      endpoints.targets(),
      endpoints.stats(),
      endpoints.listSettings(),
    ])
    targets.value = t
    statsHistory.value = s.history.total
    settings.value = st.settings ?? []
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function setSwitch(item: SettingItem, value: boolean): void {
  item.value = value ? 'true' : 'false'
}

function resetDefaults(): void {
  for (const it of settings.value) {
    it.value = ''
  }
}

async function save(): Promise<void> {
  saving.value = true
  error.value = ''
  try {
    const payload: Record<string, string> = {}
    for (const it of settings.value) {
      payload[it.key] = it.value
    }
    const r = await endpoints.updateSettings(payload)
    settings.value = r.settings ?? (await endpoints.listSettings()).settings ?? []
    notify('已保存', 'ok')
  } catch (err) {
    error.value = errorMessage(err)
    notify(errorMessage(err), 'error')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="设置" :subtitle="help.oneLiner" help-key="settings" />
  <AsyncSection :loading="loading" :error="error" :empty="settings.length === 0" empty-text="" :empty-state="emptyState" @retry="load">
    <div v-for="item in settings" :key="item.key" data-testid="setting-row" class="mb-4">
      <v-card class="pa-4">
        <div data-testid="setting-label">{{ item.label }}</div>
        <code data-testid="setting-key">{{ item.key }}</code>
        <div data-testid="setting-desc">{{ item.description }}</div>
        <v-switch
          v-if="item.kind === 'bool'"
          data-testid="setting-switch"
          :model-value="item.value === 'true'"
          @update:model-value="setSwitch(item, $event as boolean)"
        />
        <v-text-field
          v-else
          data-testid="setting-input"
          v-model="item.value"
          :placeholder="item.placeholder"
        />
        <span v-if="item.is_default === true" data-testid="setting-default-badge">当前为默认值</span>
        <span v-if="item.placeholder" data-testid="setting-placeholder">默认：{{ item.placeholder }}</span>
        <div data-testid="setting-hint">{{ fieldHelp(item.key)?.hint ?? '' }}</div>
      </v-card>
    </div>
    <div class="d-flex ga-2 mt-4">
      <v-btn data-testid="settings-save" :loading="saving" @click="save">保存</v-btn>
      <v-btn data-testid="settings-reset" variant="tonal" @click="resetDefaults">重置为默认</v-btn>
    </div>
    <div class="text-caption text-medium-emphasis mt-2">清空 = 恢复默认</div>
  </AsyncSection>
  <v-card class="pa-4 mt-4">
    <div class="mb-2">支持的目标格式：{{ targets.length === 0 ? '（无）' : targets.join(', ') }}</div>
    <div class="mb-2">历史条数：{{ statsHistory }}</div>
    <div class="d-flex ga-2">
      <v-btn variant="tonal" @click="theme.setMode('light')">浅色</v-btn>
      <v-btn variant="tonal" @click="theme.setMode('dark')">深色</v-btn>
      <v-btn variant="tonal" @click="theme.setMode('system')">跟随系统</v-btn>
    </div>
  </v-card>
  <div v-if="error !== ''" class="text-error mt-2">{{ error }}</div>
</template>