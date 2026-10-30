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

// 转换模板（INI）：只有启动时指定 -config 才存在；保存后若路径就是 -config，服务会热重载。
const templatePath = ref('')
const templateContent = ref('')
const templateAvailable = ref(false)
const templateLoading = ref(false)
const templateSaving = ref(false)
const templateError = ref('')

async function loadTemplate(): Promise<void> {
  templateLoading.value = true
  templateError.value = ''
  try {
    const r = await endpoints.getTemplate()
    templatePath.value = r.path
    templateContent.value = r.content
    templateAvailable.value = true
  } catch (err) {
    templateAvailable.value = false
    templatePath.value = ''
    templateContent.value = ''
    templateError.value = errorMessage(err)
  } finally {
    templateLoading.value = false
  }
}

async function saveTemplate(): Promise<void> {
  templateSaving.value = true
  templateError.value = ''
  try {
    await endpoints.putTemplate(templateContent.value)
    notify('模板已保存', 'ok')
    await loadTemplate()
  } catch (err) {
    templateError.value = errorMessage(err)
    notify(errorMessage(err), 'error')
  } finally {
    templateSaving.value = false
  }
}

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
  void loadTemplate()
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
  <v-card class="pa-4 mt-4" data-testid="template-card">
    <div class="d-flex align-center ga-2 mb-2">
      <span class="text-subtitle-1">转换模板（INI）</span>
      <v-spacer />
      <v-btn data-testid="template-reload" variant="text" size="small" :loading="templateLoading" @click="loadTemplate">重新加载</v-btn>
    </div>
    <div v-if="templateLoading" class="text-medium-emphasis">加载中…</div>
    <template v-else-if="templateAvailable">
      <div data-testid="template-path" class="text-caption text-medium-emphasis mb-2">路径：{{ templatePath }}</div>
      <v-textarea
        v-model="templateContent"
        data-testid="template-content"
        rows="16"
        variant="outlined"
        class="template-editor"
        spellcheck="false"
      />
      <div class="d-flex align-center ga-2">
        <v-btn data-testid="template-save" color="primary" :loading="templateSaving" @click="saveTemplate">保存模板</v-btn>
        <span class="text-caption text-medium-emphasis">保存后若该路径就是启动时的 -config，服务会在约 1 秒内热重载。</span>
      </div>
    </template>
    <v-alert v-else type="info" variant="tonal" data-testid="template-unavailable">
      未启用转换模板：启动时没有指定 -config。纯控制台模式不需要模板；
      需要自定义模板时，用 <code>-config /data/template.ini</code> 启动后再回来编辑。
    </v-alert>
    <div v-if="templateError !== ''" data-testid="template-error" class="text-error mt-2">{{ templateError }}</div>
  </v-card>
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

<style scoped>
.template-editor :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
}
</style>