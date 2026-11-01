<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type SettingItem } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp, fieldHelp } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import InfoTip from '../components/InfoTip.vue'

const help = pageHelp('settings')

const error = ref('')
const loading = ref(false)
const settings = ref<SettingItem[]>([])
const saving = ref(false)

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
    const st = await endpoints.listSettings()
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

// 数字输入框在部分浏览器会把值转成 number；统一回字符串，后端设置项只接受字符串。
function setNumber(item: SettingItem, value: unknown): void {
  item.value = value == null ? '' : String(value)
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
    <v-card class="pa-4 settings-card">
      <div v-for="item in settings" :key="item.key" data-testid="setting-row" class="setting-row">
        <div class="setting-row__label">
          <div class="sf-setting-label">
            <span data-testid="setting-label">{{ item.label }}</span>
            <InfoTip :text="fieldHelp(item.key)?.hint ?? ''" :label="item.label" />
          </div>
          <div data-testid="setting-desc" class="text-caption text-medium-emphasis">{{ item.description }}</div>
          <code data-testid="setting-key" class="setting-key">{{ item.key }}</code>
        </div>
        <div class="setting-row__control">
          <v-switch
            v-if="item.kind === 'bool'"
            data-testid="setting-switch"
            :model-value="item.value === 'true'"
            color="primary"
            hide-details
            @update:model-value="setSwitch(item, $event as boolean)"
          />
          <v-select
            v-else-if="item.kind === 'select'"
            data-testid="setting-select"
            :model-value="item.value"
            :items="item.options ?? []"
            item-title="label"
            item-value="value"
            variant="outlined"
            density="comfortable"
            hide-details
            @update:model-value="item.value = ($event as string | null) ?? ''"
          />
          <v-text-field
            v-else-if="item.kind === 'number'"
            data-testid="setting-number"
            :model-value="item.value"
            type="number"
            min="0"
            :placeholder="item.placeholder"
            variant="outlined"
            density="comfortable"
            hide-details
            @update:model-value="setNumber(item, $event)"
          />
          <v-text-field
            v-else
            data-testid="setting-input"
            v-model="item.value"
            :placeholder="item.placeholder"
            variant="outlined"
            density="comfortable"
            hide-details
          />
          <div v-if="item.is_default === true" data-testid="setting-default-badge" class="text-caption text-medium-emphasis mt-1">当前为默认值</div>
          <div v-if="item.placeholder" data-testid="setting-placeholder" class="text-caption text-medium-emphasis mt-1">默认：{{ item.placeholder }}</div>
        </div>
      </div>
      <div class="d-flex ga-2 mt-4">
        <v-btn data-testid="settings-save" :loading="saving" @click="save">保存</v-btn>
        <v-btn data-testid="settings-reset" variant="tonal" @click="resetDefaults">重置为默认</v-btn>
      </div>
      <div class="text-caption text-medium-emphasis mt-2">清空输入框表示恢复默认值。</div>
    </v-card>
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
      当前未启用转换模板（启动时未指定 -config）。如需自定义模板，请使用
      <code>-config /data/template.ini</code> 指定模板文件后重新启动。
    </v-alert>
    <div v-if="templateError !== ''" data-testid="template-error" class="text-error mt-2">{{ templateError }}</div>
  </v-card>
  <div v-if="error !== ''" class="text-error mt-2">{{ error }}</div>
</template>

<style scoped>
.settings-card {
  margin-top: 4px;
}
.setting-row {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(220px, 360px);
  gap: 20px;
  align-items: start;
  padding: 14px 0;
  border-bottom: 1px solid var(--sf-border);
}
.setting-row:last-of-type {
  border-bottom: 0;
}
.setting-row__label {
  min-width: 0;
}
.setting-key {
  display: inline-block;
  margin-top: 4px;
  color: var(--sf-text-faint);
  font-size: 11px;
}
.setting-row__control {
  min-width: 0;
}
@media (max-width: 700px) {
  .setting-row {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
.sf-setting-label {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}
.template-editor :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
}
</style>